package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// MigrateConfig holds migration configuration.
type MigrateConfig struct {
	CSVPath   string
	DBPath    string
	TableName string
	Workers   int
	BatchSize int
}

var migrateConfig MigrateConfig

func init() {
	flag.StringVar(&migrateConfig.CSVPath, "csv", "", "Path to CSV file (required)")
	flag.StringVar(&migrateConfig.DBPath, "db", "migration.db", "Path to SQLite database")
	flag.StringVar(&migrateConfig.TableName, "table", "data", "Table name to create")
	flag.IntVar(&migrateConfig.Workers, "workers", 4, "Number of concurrent workers")
	flag.IntVar(&migrateConfig.BatchSize, "batch", 1000, "Batch size for inserts")
}

func main() {
	flag.Parse()

	if migrateConfig.CSVPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -csv flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -csv=data.csv [-db=database.db] [-table=name] [-workers=4] [-batch=1000]\n", os.Args[0])
		os.Exit(1)
	}

	fmt.Printf("Migrating CSV to SQLite\n")
	fmt.Printf("CSV: %s\n", migrateConfig.CSVPath)
	fmt.Printf("DB: %s\n", migrateConfig.DBPath)
	fmt.Printf("Table: %s\n", migrateConfig.TableName)
	fmt.Printf("Workers: %d\n", migrateConfig.Workers)
	fmt.Println()

	if err := runMigration(); err != nil {
		fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ Migration completed successfully!")
}

// runMigration executes the CSV to SQLite migration.
func runMigration() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Open CSV file
	file, err := os.Open(migrateConfig.CSVPath)
	if err != nil {
		return fmt.Errorf("open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'

	// Read header to get column names
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read CSV header: %w", err)
	}

	// Sanitize column names for SQL
	columns := sanitizeColumns(header)
	fmt.Printf("Found %d columns: %v\n", len(columns), columns)

	// Open database
	db, err := sql.Open("sqlite3", migrateConfig.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	// Create table
	if err := createTable(ctx, db, columns); err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	// Migrate data
	count, err := migrateData(ctx, db, reader, columns)
	if err != nil {
		return fmt.Errorf("migrate data: %w", err)
	}

	fmt.Printf("\nTotal records migrated: %d\n", count)
	return nil
}

// sanitizeColumns converts CSV headers to valid SQL column names.
func sanitizeColumns(header []string) []string {
	columns := make([]string, len(header))
	for i, col := range header {
		// Convert to lowercase
		name := strings.ToLower(col)
		// Replace invalid characters with underscore
		name = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(name, "_")
		// Remove leading digits
		name = regexp.MustCompile(`^[0-9]+`).ReplaceAllString(name, "")
		// Default name if empty
		if name == "" {
			name = fmt.Sprintf("col%d", i)
		}
		columns[i] = name
	}
	return columns
}

// createTable creates the SQLite table based on CSV columns.
func createTable(ctx context.Context, db *sql.DB, columns []string) error {
	// Build column definitions
	var colDefs []string
	for _, col := range columns {
		colDefs = append(colDefs, fmt.Sprintf("%s TEXT", col))
	}

	// Add id column
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			%s
		)
	`, migrateConfig.TableName, strings.Join(colDefs, ",\n"))

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("execute CREATE TABLE: %w", err)
	}

	// Create index on first column (usually the main identifier)
	if len(columns) > 0 {
		indexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_%s ON %s(%s)",
			migrateConfig.TableName, columns[0], migrateConfig.TableName, columns[0])
		if _, err := db.ExecContext(ctx, indexQuery); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not create index: %v\n", err)
		}
	}

	fmt.Printf("Table '%s' created with columns: %s\n", migrateConfig.TableName, strings.Join(columns, ", "))
	return nil
}

// migrateData reads CSV and inserts into SQLite using batch processing.
// Uses a single writer to avoid "database is locked" errors with SQLite.
func migrateData(ctx context.Context, db *sql.DB, reader *csv.Reader, columns []string) (int, error) {
	// Prepare insert statement
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = "?"
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		migrateConfig.TableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	// Enable WAL mode for better concurrent read performance
	_, err = db.ExecContext(ctx, "PRAGMA journal_mode=WAL")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not set WAL mode: %v\n", err)
	}

	// Set busy timeout
	_, err = db.ExecContext(ctx, "PRAGMA busy_timeout=30000")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not set busy timeout: %v\n", err)
	}

	batch := make([][]string, 0, migrateConfig.BatchSize)
	count := 0
	progress := 0

	for {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			// Flush remaining batch
			if len(batch) > 0 {
				if err := flushBatch(db, stmt, batch); err != nil {
					return count, err
				}
			}
			return count, nil
		}
		if err != nil {
			return count, fmt.Errorf("read CSV record: %w", err)
		}

		// Skip if record has wrong number of columns
		if len(record) != len(columns) {
			continue
		}

		batch = append(batch, record)
		count++

		// Flush when batch is full
		if len(batch) >= migrateConfig.BatchSize {
			if err := flushBatch(db, stmt, batch); err != nil {
				return count, err
			}
			batch = batch[:0]
			progress += migrateConfig.BatchSize
			fmt.Printf("\rMigrated %d records...", count)
		}
	}
}

// flushBatch inserts a batch of records in a single transaction.
func flushBatch(db *sql.DB, stmt *sql.Stmt, batch [][]string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	for _, record := range batch {
		if _, err := tx.Stmt(stmt).Exec(interfaceSlice(record)...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// interfaceSlice converts []string to []interface{} for SQL execution.
func interfaceSlice(s []string) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}
