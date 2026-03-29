package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteConfig holds SQLite-specific configuration.
type SQLiteConfig struct {
	DBPath     string
	BuildDB    bool
	Workers    int
	BatchSize  int
	FTSEnabled bool
}

// SQLiteManager handles SQLite operations.
type SQLiteManager struct {
	dbPath string
	db     *sql.DB
}

// NewSQLiteManager creates a new SQLite manager.
func NewSQLiteManager(dbPath string) (*SQLiteManager, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return &SQLiteManager{
		dbPath: dbPath,
		db:     db,
	}, nil
}

// Close closes the database connection.
func (sm *SQLiteManager) Close() error {
	return sm.db.Close()
}

// BuildDBFromCSV imports CSV data into SQLite with FTS5.
func (sm *SQLiteManager) BuildDBFromCSV(ctx context.Context, csvPath string, workers int) error {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	// Create tables
	if err := sm.createTables(); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	// Open CSV file
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("open CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'

	// Skip header
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	// Prepare statement
	stmt, err := sm.db.Prepare(`
		INSERT INTO records (nacionalidad, dni, primer_apellido, segundo_apellido, primer_nombre, segundo_nombre, cod_centro)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	// Batch insert with workers
	recordCh := make(chan []string, workers*2)
	errCh := make(chan error, 1)
	doneCh := make(chan struct{})

	// Start workers
	for w := 0; w < workers; w++ {
		go func() {
			for record := range recordCh {
				if len(record) < 7 {
					continue
				}
				_, err := stmt.Exec(
					record[0], // Nacionalidad
					record[1], // DNI
					record[2], // Primer_Apellido
					record[3], // Segundo_Apellido
					record[4], // Primer_Nombre
					record[5], // Segundo_Nombre
					record[6], // Cod_Centro
				)
				if err == nil {
					// Note: atomic increment would be better for high concurrency
				}
			}
		}()
	}

	// Feed records
	go func() {
		defer close(recordCh)
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			record, err := reader.Read()
			if err == io.EOF {
				doneCh <- struct{}{}
				return
			}
			if err != nil {
				errCh <- fmt.Errorf("read CSV: %w", err)
				return
			}
			recordCh <- record
		}
	}()

	// Wait for completion
	select {
	case err := <-errCh:
		return err
	case <-doneCh:
	}

	// Build FTS index (optional - continue if not available)
	_ = sm.buildFTSIndex()

	return nil
}

// createTables creates the necessary database tables.
func (sm *SQLiteManager) createTables() error {
	// Main records table
	_, err := sm.db.Exec(`
		CREATE TABLE IF NOT EXISTS records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nacionalidad TEXT,
			dni TEXT UNIQUE,
			primer_apellido TEXT,
			segundo_apellido TEXT,
			primer_nombre TEXT,
			segundo_nombre TEXT,
			cod_centro TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("create records table: %w", err)
	}

	// Create index on DNI for fast lookups
	_, err = sm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_dni ON records(dni)`)
	if err != nil {
		return fmt.Errorf("create DNI index: %w", err)
	}

	// Try to create FTS5 table - continue if not available
	_, _ = sm.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS records_fts USING fts5(
			dni,
			primer_nombre,
			segundo_nombre,
			primer_apellido,
			segundo_apellido,
			content='records',
			content_rowid='id'
		)
	`)

	return nil
}

// buildFTSIndex builds the FTS5 index from existing records.
func (sm *SQLiteManager) buildFTSIndex() error {
	_, err := sm.db.Exec(`INSERT INTO records_fts(records_fts) VALUES('rebuild')`)
	if err != nil {
		return fmt.Errorf("rebuild FTS: %w", err)
	}

	// Optimize FTS index
	_, err = sm.db.Exec(`INSERT INTO records_fts(records_fts) VALUES('optimize')`)
	return err
}

// SearchByDNI performs an exact match search by DNI.
func (sm *SQLiteManager) SearchByDNI(dni string) ([]Record, error) {
	rows, err := sm.db.Query(`
		SELECT nacionalidad, dni, primer_apellido, segundo_apellido, 
		       primer_nombre, segundo_nombre, cod_centro
		FROM records WHERE dni = ?
	`, dni)
	if err != nil {
		return nil, fmt.Errorf("query DNI: %w", err)
	}
	defer rows.Close()

	return sm.scanRecords(rows)
}

// SearchByField performs a search by field using FTS5.
func (sm *SQLiteManager) SearchByField(field, value string, pattern SearchPattern) ([]Record, error) {
	var query string
	var args []interface{}

	switch pattern {
	case PatternExact:
		query = `SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
					r.primer_nombre, r.segundo_nombre, r.cod_centro
				 FROM records r
				 WHERE r.` + field + ` = ?`
		args = []interface{}{value}

	case PatternContains:
		query = `SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
					r.primer_nombre, r.segundo_nombre, r.cod_centro
				 FROM records r
				 WHERE r.` + field + ` LIKE ?`
		args = []interface{}{"%" + value + "%"}

	case PatternStartsWith:
		query = `SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
					r.primer_nombre, r.segundo_nombre, r.cod_centro
				 FROM records r
				 WHERE r.` + field + ` LIKE ?`
		args = []interface{}{value + "%"}

	case PatternRegex:
		// SQLite doesn't support regex natively, fall back to LIKE
		query = `SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
					r.primer_nombre, r.segundo_nombre, r.cod_centro
				 FROM records r
				 WHERE r.` + field + ` LIKE ?`
		args = []interface{}{"%" + value + "%"}

	default:
		return sm.SearchByField(field, value, PatternExact)
	}

	rows, err := sm.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query field: %w", err)
	}
	defer rows.Close()

	return sm.scanRecords(rows)
}

// SearchFTS performs a full-text search using FTS5.
func (sm *SQLiteManager) SearchFTS(query string) ([]Record, error) {
	rows, err := sm.db.Query(`
		SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			   r.primer_nombre, r.segundo_nombre, r.cod_centro
		FROM records r
		JOIN records_fts f ON r.id = f.rowid
		WHERE records_fts MATCH ?
	`, query)
	if err != nil {
		return nil, fmt.Errorf("FTS query: %w", err)
	}
	defer rows.Close()

	return sm.scanRecords(rows)
}

// searchAll combines multiple field searches with AND/OR logic.
func (sm *SQLiteManager) SearchAll(conditions []SearchCondition, logic SearchLogic) ([]Record, error) {
	if len(conditions) == 0 {
		return nil, nil
	}

	// For simplicity, build a query that filters in memory
	// For large datasets, this should be done at SQL level

	var results []Record
	var err error

	// Get initial results from first condition
	first := conditions[0]
	results, err = sm.SearchByField(first.Field, first.Value, first.Pattern)
	if err != nil {
		return nil, err
	}

	// Apply remaining conditions
	for i := 1; i < len(conditions); i++ {
		cond := conditions[i]
		more, err := sm.SearchByField(cond.Field, cond.Value, cond.Pattern)
		if err != nil {
			return nil, err
		}

		if logic == LogicAND {
			results = intersectRecords(results, more)
		} else {
			results = unionRecords(results, more)
		}
	}

	return results, nil
}

// scanRecords scans rows into Record structs.
func (sm *SQLiteManager) scanRecords(rows *sql.Rows) ([]Record, error) {
	var records []Record
	for rows.Next() {
		var r Record
		err := rows.Scan(
			&r.Nacionalidad,
			&r.DNI,
			&r.Primer_Apellido,
			&r.Segundo_Apellido,
			&r.Primer_Nombre,
			&r.Segundo_Nombre,
			&r.Cod_Centro,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// GetRecordCount returns the total number of records in the database.
func (sm *SQLiteManager) GetRecordCount() (int, error) {
	var count int
	err := sm.db.QueryRow("SELECT COUNT(*) FROM records").Scan(&count)
	return count, err
}

// fieldToColumn maps field names to database column names.
func fieldToColumn(field string) string {
	switch strings.ToLower(field) {
	case "dni", "cedula":
		return "dni"
	case "primernombre", "primer_nombre", "firstname":
		return "primer_nombre"
	case "segundonombre", "segundo_nombre", "secondname":
		return "segundo_nombre"
	case "primerapellido", "primer_apellido", "firstlastname":
		return "primer_apellido"
	case "segundoapellido", "segundo_apellido", "secondlastname":
		return "segundo_apellido"
	case "nacionalidad":
		return "nacionalidad"
	case "codcentro", "cod_centro", "centro":
		return "cod_centro"
	default:
		return field
	}
}
