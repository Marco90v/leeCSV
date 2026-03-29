package cmd

import (
	"context"
	"fmt"
	"os"

	"go/csv/internal"

	"github.com/spf13/cobra"
)

// dbCmd represents the db command.
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database operations (build, search)",
	Long:  `Build and search using SQLite database.`,
}

func init() {
	rootCmd.AddCommand(dbCmd)
}

// dbBuildCmd represents the db build command.
var dbBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build SQLite database from CSV",
	Long: `Build a SQLite database from a CSV file.
The database supports fast searches using indexes and optional FTS5.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDBBuild()
	},
}

func init() {
	dbCmd.AddCommand(dbBuildCmd)

	dbBuildCmd.Flags().StringVar(&cfg.CSVPath, "csv", DefaultCSVPath, "Path to CSV file")
	dbBuildCmd.Flags().StringVar(&cfg.DBPath, "db", DefaultDBPath, "Path to SQLite database")
	dbBuildCmd.Flags().IntVarP(&cfg.Workers, "workers", "w", 0, "Number of workers (0=auto)")
}

// dbSearchCmd represents the db search command.
var dbSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search using SQLite database",
	Long:  `Search records using a SQLite database.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDBSearch()
	},
}

func init() {
	dbCmd.AddCommand(dbSearchCmd)

	dbSearchCmd.Flags().StringVar(&cfg.DBPath, "db", DefaultDBPath, "Path to SQLite database")
	dbSearchCmd.Flags().StringVar(&cfg.DNI, "dni", "", "Search by DNI")
	dbSearchCmd.Flags().StringVar((*string)(&cfg.DNIPattern), "dni-pattern", "exact", "DNI pattern: exact, contains, startswith")
	dbSearchCmd.Flags().StringVar(&cfg.PrimerNombre, "primer-nombre", "", "Search by first name")
	dbSearchCmd.Flags().StringVar((*string)(&cfg.PrimerNombrePattern), "primer-nombre-pattern", "exact", "First name pattern: exact, contains, startswith")
	dbSearchCmd.Flags().StringVar(&cfg.SegundoNombre, "segundo-nombre", "", "Search by second name")
	dbSearchCmd.Flags().StringVar(&cfg.PrimerApellido, "primer-apellido", "", "Search by first last name")
	dbSearchCmd.Flags().StringVar(&cfg.SegundoApellido, "segundo-apellido", "", "Search by second last name")
	dbSearchCmd.Flags().StringVar((*string)(&cfg.Logic), "logic", "AND", "Search logic: AND, OR")
}

func runDBBuild() {
	fmt.Printf("Building SQLite database from: %s\n", cfg.CSVPath)
	fmt.Printf("Workers: %d\n", cfg.Workers)

	// Check file exists
	if _, err := os.Stat(cfg.CSVPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("CSV file not found: %s", cfg.CSVPath), nil)
	}

	db, err := internal.NewSQLiteManager(cfg.DBPath)
	if err != nil {
		exitWithError("Error creating database", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.BuildDBFromCSV(ctx, cfg.CSVPath, cfg.Workers); err != nil {
		exitWithError("Error building database", err)
	}

	count, _ := db.GetRecordCount()
	fmt.Printf("Database built: %d records\n", count)
	fmt.Printf("Database saved to: %s\n", cfg.DBPath)
}

func runDBSearch() {
	fmt.Printf("Using database: %s\n", cfg.DBPath)

	// Check file exists
	if _, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("Database file not found: %s", cfg.DBPath), nil)
	}

	db, err := internal.NewSQLiteManager(cfg.DBPath)
	if err != nil {
		exitWithError("Error opening database", err)
	}
	defer db.Close()

	conditions := collectConditions()
	results, err := db.SearchAll(conditions, cfg.Logic)
	if err != nil {
		exitWithError("Search error", err)
	}
	printResults(results)
}
