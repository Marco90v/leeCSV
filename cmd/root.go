package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

// Default configuration constants
const (
	DefaultCSVPath   = "./nacional.csv"
	DefaultIndexPath = "./index.json"
	DefaultDBPath    = "./data.db"
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
	UseFTS     *bool // nil=auto, true=force FTS5, false=disable FTS5

	// Search parameters
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

var cfg Config

// SearchParams holds search parameters for commands.
type SearchParams struct {
	DNI             string
	PrimerNombre    string
	SegundoNombre   string
	PrimerApellido  string
	SegundoApellido string
	Logic           SearchLogic
	Mode            SearchMode
	CSVPath         string
	IndexPath       string
	DBPath          string
	Workers         int
}

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "leeCSV",
	Short: "Search and filter records from large CSV files",
	Long: `leeCSV is a CLI tool for searching and filtering records from large CSV files 
containing national ID data. It supports multiple search modes: CSV, Index, and SQLite.

Examples:
  # Search using CSV mode
  leeCSV search --dni=12345678

  # Build and search using index
  leeCSV index build --csv=data.csv
  leeCSV index search --dni=12345678

  # Build and search using SQLite
  leeCSV db build --csv=data.csv
  leeCSV db search --dni=12345678`,
	Version: "1.0.0",
}

// Execute adds all child commands to the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfg.CSVPath, "csv", "./nacional.csv", "Path to CSV file")
	rootCmd.PersistentFlags().StringVar(&cfg.IndexPath, "index", "./index.json", "Path to index file")
	rootCmd.PersistentFlags().StringVar(&cfg.DBPath, "db", "./data.db", "Path to SQLite database")
	rootCmd.PersistentFlags().IntVarP(&cfg.Workers, "workers", "w", 0, "Number of workers (0=auto)")
}

func initConfig() {
	// Auto-detect workers if not set
	if cfg.Workers <= 0 {
		cfg.Workers = getDefaultWorkers()
	}
}

func getDefaultWorkers() int {
	// Use number of CPUs
	return runtime.NumCPU()
}

// getSearchParams converts global config to SearchParams.
func getSearchParams() SearchParams {
	return SearchParams{
		DNI:             cfg.DNI,
		PrimerNombre:    cfg.PrimerNombre,
		SegundoNombre:   cfg.SegundoNombre,
		PrimerApellido:  cfg.PrimerApellido,
		SegundoApellido: cfg.SegundoApellido,
		Logic:           cfg.Logic,
		Mode:            cfg.Mode,
		CSVPath:         cfg.CSVPath,
		IndexPath:       cfg.IndexPath,
		DBPath:          cfg.DBPath,
		Workers:         cfg.Workers,
	}
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

// exitWithError prints error and exits.
func exitWithError(msg string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", msg)
	}
	os.Exit(1)
}
