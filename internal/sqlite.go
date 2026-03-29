package internal

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// SQLite configuration constants
const (
	DefaultBatchSize   = 1000
	WorkersBatchFactor = 100
	BusyTimeoutMs      = 30000
)

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

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Set busy timeout
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", BusyTimeoutMs)); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	// Optimize for bulk inserts
	if _, err := db.Exec("PRAGMA synchronous=OFF"); err != nil {
		return nil, fmt.Errorf("set synchronous: %w", err)
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
	if err := sm.createTables(); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

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

	// Batch processing - increase batch size for better performance
	batchSize := DefaultBatchSize * 10 // 10,000 records per batch
	if workers > 0 {
		batchSize = workers * WorkersBatchFactor * 10
	}
	batch := make([][]string, 0, batchSize)

	recordCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			if len(batch) > 0 {
				if err := sm.flushBatch(stmt, batch); err != nil {
					return err
				}
			}
			break
		}
		if err != nil {
			return fmt.Errorf("read CSV: %w", err)
		}

		if len(record) < 7 {
			continue
		}

		batch = append(batch, record)
		recordCount++

		if len(batch) >= batchSize {
			if err := sm.flushBatch(stmt, batch); err != nil {
				return err
			}
			batch = batch[:0]

			// Log progress every 100k records
			if recordCount%100000 == 0 {
				fmt.Printf("Imported %d records...\n", recordCount)
			}
		}
	}

	// Build FTS index for full-text search
	if err := sm.buildFTSIndex(); err != nil {
		fmt.Printf("Warning: FTS index build failed: %v\n", err)
	}

	// Run ANALYZE for query optimization
	if _, err := sm.db.Exec("ANALYZE"); err != nil {
		fmt.Printf("Warning: ANALYZE failed: %v\n", err)
	}

	fmt.Printf("Import complete: %d records\n", recordCount)
	return nil
}

// flushBatch inserts a batch in a transaction.
func (sm *SQLiteManager) flushBatch(stmt *sql.Stmt, batch [][]string) error {
	tx, err := sm.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	for _, record := range batch {
		if _, err := tx.Stmt(stmt).Exec(
			record[0], record[1], record[2], record[3],
			record[4], record[5], record[6],
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// createTables creates database tables.
func (sm *SQLiteManager) createTables() error {
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

	// Index on DNI
	_, err = sm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_dni ON records(dni)`)
	if err != nil {
		return fmt.Errorf("create DNI index: %w", err)
	}

	// Create indexes on name fields for faster lookups
	_, _ = sm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_primer_nombre ON records(primer_nombre)`)
	_, _ = sm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_primer_apellido ON records(primer_apellido)`)

	// Try to create FTS5 virtual table for full-text search
	// Note: FTS5 may not be available in all SQLite builds
	if err := sm.createFTSTable(); err != nil {
		fmt.Printf("Note: FTS5 not available - full-text search will use LIKE queries\n")
	}

	return nil
}

// createFTSTable attempts to create FTS5 table and triggers.
// Returns nil if FTS5 is not available (graceful degradation).
func (sm *SQLiteManager) createFTSTable() error {
	// Create FTS5 virtual table for full-text search
	_, err := sm.db.Exec(`
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
	if err != nil {
		return fmt.Errorf("FTS5 not available: %w", err)
	}

	// Create triggers to keep FTS index synchronized
	// Insert trigger
	_, err = sm.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS records_ai AFTER INSERT ON records BEGIN
			INSERT INTO records_fts(rowid, dni, primer_nombre, segundo_nombre, primer_apellido, segundo_apellido)
			VALUES (new.id, new.dni, new.primer_nombre, new.segundo_nombre, new.primer_apellido, new.segundo_apellido);
		END
	`)
	if err != nil {
		return fmt.Errorf("create insert trigger: %w", err)
	}

	// Delete trigger
	_, err = sm.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS records_ad AFTER DELETE ON records BEGIN
			INSERT INTO records_fts(records_fts, rowid, dni, primer_nombre, segundo_nombre, primer_apellido, segundo_apellido)
			VALUES('delete', old.id, old.dni, old.primer_nombre, old.segundo_nombre, old.primer_apellido, old.segundo_apellido);
		END
	`)
	if err != nil {
		return fmt.Errorf("create delete trigger: %w", err)
	}

	// Update trigger
	_, err = sm.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS records_au AFTER UPDATE ON records BEGIN
			INSERT INTO records_fts(records_fts, rowid, dni, primer_nombre, segundo_nombre, primer_apellido, segundo_apellido)
			VALUES('delete', old.id, old.dni, old.primer_nombre, old.segundo_nombre, old.primer_apellido, old.segundo_apellido);
			INSERT INTO records_fts(rowid, dni, primer_nombre, segundo_nombre, primer_apellido, segundo_apellido)
			VALUES (new.id, new.dni, new.primer_nombre, new.segundo_nombre, new.primer_apellido, new.segundo_apellido);
		END
	`)
	if err != nil {
		return fmt.Errorf("create update trigger: %w", err)
	}

	return nil
}

// buildFTSIndex builds the FTS5 index.
// Returns nil if FTS5 is not available (graceful degradation).
func (sm *SQLiteManager) buildFTSIndex() error {
	// Check if FTS table exists
	var count int
	err := sm.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='records_fts'").Scan(&count)
	if err != nil || count == 0 {
		return fmt.Errorf("FTS5 table not available")
	}

	_, err = sm.db.Exec(`INSERT INTO records_fts(records_fts) VALUES('rebuild')`)
	if err != nil {
		return fmt.Errorf("rebuild FTS: %w", err)
	}
	_, err = sm.db.Exec(`INSERT INTO records_fts(records_fts) VALUES('optimize')`)
	return err
}

// SearchByField performs a search by field.
func (sm *SQLiteManager) SearchByField(field, value string, pattern SearchPattern) ([]Record, error) {
	var query string
	var args []interface{}

	switch pattern {
	case PatternExact:
		query = fmt.Sprintf(`SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			r.primer_nombre, r.segundo_nombre, r.cod_centro
			FROM records r WHERE r.%s = ?`, field)
		args = []interface{}{value}

	case PatternContains:
		query = fmt.Sprintf(`SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			r.primer_nombre, r.segundo_nombre, r.cod_centro
			FROM records r WHERE r.%s LIKE ?`, field)
		args = []interface{}{"%" + value + "%"}

	case PatternStartsWith:
		query = fmt.Sprintf(`SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			r.primer_nombre, r.segundo_nombre, r.cod_centro
			FROM records r WHERE r.%s LIKE ?`, field)
		args = []interface{}{value + "%"}

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

// SearchAll combines multiple field searches.
func (sm *SQLiteManager) SearchAll(conditions []SearchCondition, logic SearchLogic) ([]Record, error) {
	if len(conditions) == 0 {
		return nil, nil
	}

	first := conditions[0]
	results, err := sm.SearchByField(first.Field, first.Value, first.Pattern)
	if err != nil {
		return nil, err
	}

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

// GetRecordCount returns total records.
func (sm *SQLiteManager) GetRecordCount() (int, error) {
	var count int
	err := sm.db.QueryRow("SELECT COUNT(*) FROM records").Scan(&count)
	return count, err
}
