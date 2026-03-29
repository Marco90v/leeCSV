package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Default configuration constants
const (
	DefaultCSVPath   = "./nacional.csv"
	DefaultIndexPath = "./index.json"
	DefaultDBPath    = "./data.db"
	DefaultWorkers   = 0 // 0 = auto-detect
	DefaultLogic     = "AND"
	DefaultMode      = "csv"
)

// Field names for search (used consistently across modes)
const (
	FieldDNI             = "dni"
	FieldPrimerNombre    = "primer_nombre"
	FieldSegundoNombre   = "segundo_nombre"
	FieldPrimerApellido  = "primer_apellido"
	FieldSegundoApellido = "segundo_apellido"
	FieldNacionalidad    = "nacionalidad"
	FieldCodCentro       = "cod_centro"
)

// SearchMode defines the operating mode of the application.
type SearchMode string

const (
	ModeCSV    SearchMode = "csv"
	ModeIndex  SearchMode = "index"
	ModeSQLite SearchMode = "sqlite"
)

// Config holds all CLI configuration.
type Config struct {
	Mode       SearchMode
	CSVPath    string
	IndexPath  string
	DBPath     string
	BuildIndex bool
	BuildDB    bool
	Workers    int
	Logic      SearchLogic

	// Search parameters with pattern support
	DNI                    string
	DNIPattern             SearchPattern
	PrimerNombre           string
	PrimerNombrePattern    SearchPattern
	SegundoNombre          string
	SegundoNombrePattern   SearchPattern
	PrimerApellido         string
	PrimerApellidoPattern  SearchPattern
	SegundoApellido        string
	SegundoApellidoPattern SearchPattern
}

var config Config

func init() {
	flag.StringVar(&config.CSVPath, "csv", DefaultCSVPath, "Path to CSV file")
	flag.StringVar(&config.IndexPath, "index", DefaultIndexPath, "Path to index file")
	flag.StringVar(&config.DBPath, "db", DefaultDBPath, "Path to SQLite database")
	flag.StringVar((*string)(&config.Mode), "mode", DefaultMode, "Search mode: csv, index, sqlite")
	flag.BoolVar(&config.BuildIndex, "build", false, "Build index from CSV (index mode only)")
	flag.IntVar(&config.Workers, "workers", 0, "Number of workers (0 = auto)")

	flag.StringVar(&config.DNI, "dni", "", "Search by DNI (exact match)")
	flag.StringVar(&config.PrimerNombre, "primerNombre", "", "Search by first name")
	flag.StringVar(&config.SegundoNombre, "segundoNombre", "", "Search by second name")
	flag.StringVar(&config.PrimerApellido, "primerApellido", "", "Search by first last name")
	flag.StringVar(&config.SegundoApellido, "segundoApellido", "", "Search by second last name")
	flag.BoolVar(&config.BuildDB, "build-db", false, "Build SQLite database from CSV (sqlite mode only)")
	flag.StringVar((*string)(&config.Logic), "logic", DefaultLogic, "Search logic: AND, OR")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -mode=csv -csv=data.csv -dni=12345678\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode=index -build -csv=data.csv -index=idx.json\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode=index -index=idx.json -dni=12345678\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode=sqlite -build-db -csv=data.csv -db=data.db\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode=sqlite -db=data.db -dni=12345678\n", os.Args[0])
	}
}

func main() {
	flag.Parse()

	// Validate input
	if err := validateConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	if config.Workers <= 0 {
		config.Workers = runtime.NumCPU()
	}

	fmt.Printf("leeCSV - Search Mode: %s\n", config.Mode)

	switch config.Mode {
	case ModeCSV:
		runCSVMode()
	case ModeIndex:
		runIndexMode()
	case ModeSQLite:
		runSQLiteMode()
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", config.Mode)
		os.Exit(1)
	}
}

// runCSVMode runs the legacy CSV search mode.
func runCSVMode() {
	fmt.Println("CSV mode - loading file and searching...")
	fmt.Printf("Search params: DNI=%s, PrimerNombre=%s, PrimerApellido=%s\n",
		config.DNI, config.PrimerNombre, config.PrimerApellido)

	records, err := readFile(config.CSVPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	results := searchCSV(records)
	printResults(results)
}

// runIndexMode handles index build or search.
func runIndexMode() {
	if config.BuildIndex {
		fmt.Printf("Building index from: %s\n", config.CSVPath)
		fmt.Printf("Workers: %d\n", config.Workers)

		index, err := BuildIndex(config.CSVPath, config.Workers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building index: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Index built: %d records\n", index.TotalRecords)

		if err := index.Save(config.IndexPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving index: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Index saved to: %s\n", config.IndexPath)
		return
	}

	// Search mode
	fmt.Printf("Loading index from: %s\n", config.IndexPath)
	index, err := LoadIndex(config.IndexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Index loaded: %d records\n", index.TotalRecords)

	results := searchIndex(index)
	printResults(results)
}

// collectConditions gathers all search conditions from config.
func collectConditions() []SearchCondition {
	var conditions []SearchCondition

	if config.DNI != "" {
		pattern := config.DNIPattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: FieldDNI, Value: config.DNI, Pattern: pattern})
	}

	if config.PrimerNombre != "" {
		pattern := config.PrimerNombrePattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: FieldPrimerNombre, Value: config.PrimerNombre, Pattern: pattern})
	}

	if config.SegundoNombre != "" {
		pattern := config.SegundoNombrePattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: FieldSegundoNombre, Value: config.SegundoNombre, Pattern: pattern})
	}

	if config.PrimerApellido != "" {
		pattern := config.PrimerApellidoPattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: FieldPrimerApellido, Value: config.PrimerApellido, Pattern: pattern})
	}

	if config.SegundoApellido != "" {
		pattern := config.SegundoApellidoPattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: FieldSegundoApellido, Value: config.SegundoApellido, Pattern: pattern})
	}

	return conditions
}

// searchIndex searches using the in-memory index.
func searchIndex(idx *Index) []Record {
	conditions := collectConditions()
	return idx.SearchAll(conditions, config.Logic)
}

// runSQLiteMode handles SQLite build or search.
func runSQLiteMode() {
	db, err := NewSQLiteManager(config.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if config.BuildDB {
		fmt.Printf("Building SQLite database from: %s\n", config.CSVPath)
		fmt.Printf("Workers: %d\n", config.Workers)

		if err := db.BuildDBFromCSV(context.Background(), config.CSVPath, config.Workers); err != nil {
			fmt.Fprintf(os.Stderr, "Error building database: %v\n", err)
			os.Exit(1)
		}

		count, _ := db.GetRecordCount()
		fmt.Printf("Database built: %d records\n", count)
		fmt.Printf("Database saved to: %s\n", config.DBPath)
		return
	}

	// Search mode
	results := searchSQLite(db)
	printResults(results)
}

// searchSQLite searches using SQLite.
func searchSQLite(db *SQLiteManager) []Record {
	conditions := collectConditions()
	results, err := db.SearchAll(conditions, config.Logic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
		os.Exit(1)
	}
	return results
}

// printResults outputs search results.
func printResults(results []Record) {
	fmt.Printf("Found %d matches\n", len(results))
	for _, r := range results {
		fmt.Printf("%s - %s %s, %s %s\n",
			r.DNI,
			r.Primer_Nombre,
			r.Segundo_Nombre,
			r.Primer_Apellido,
			r.Segundo_Apellido,
		)
	}
}

// validateConfig validates the CLI configuration.
func validateConfig() error {
	// Validate mode
	switch config.Mode {
	case ModeCSV, ModeIndex, ModeSQLite:
		// valid
	default:
		return fmt.Errorf("invalid mode: %s (valid: csv, index, sqlite)", config.Mode)
	}

	// Validate logic
	switch config.Logic {
	case LogicAND, LogicOR:
		// valid
	default:
		return fmt.Errorf("invalid logic: %s (valid: AND, OR)", config.Logic)
	}

	// Check if any search criteria or build flags are provided
	hasSearchParams := config.DNI != "" || config.PrimerNombre != "" ||
		config.SegundoNombre != "" || config.PrimerApellido != "" || config.SegundoApellido != ""
	hasBuildFlag := config.BuildIndex || config.BuildDB

	if !hasSearchParams && !hasBuildFlag {
		return fmt.Errorf("no search criteria or build flag provided")
	}

	// Validate paths based on mode and build flag
	if config.BuildIndex {
		if !fileExists(config.CSVPath) {
			return fmt.Errorf("CSV file not found: %s", config.CSVPath)
		}
	}

	if config.BuildDB {
		if !fileExists(config.CSVPath) {
			return fmt.Errorf("CSV file not found: %s", config.CSVPath)
		}
	}

	if config.Mode == ModeIndex && !config.BuildIndex {
		if !fileExists(config.IndexPath) {
			return fmt.Errorf("index file not found: %s", config.IndexPath)
		}
	}

	if config.Mode == ModeSQLite && !config.BuildDB {
		if !fileExists(config.DBPath) {
			return fmt.Errorf("database file not found: %s", config.DBPath)
		}
	}

	return nil
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Abs resolves path to absolute form.
func abs(path string) string {
	if absPath, err := filepath.Abs(path); err == nil {
		return absPath
	}
	return path
}
