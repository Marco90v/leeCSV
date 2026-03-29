package cmd

import (
	"fmt"
	"os"

	"go/csv/internal"

	"github.com/spf13/cobra"
)

// indexCmd represents the index command.
var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index operations (build, search)",
	Long:  `Build and search using in-memory index for fast lookups.`,
}

func init() {
	rootCmd.AddCommand(indexCmd)
}

// indexBuildCmd represents the index build command.
var indexBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build index from CSV",
	Long: `Build an in-memory index from a CSV file.
The index allows for fast lookups on subsequent searches.`,
	Run: func(cmd *cobra.Command, args []string) {
		runIndexBuild()
	},
}

func init() {
	indexCmd.AddCommand(indexBuildCmd)

	indexBuildCmd.Flags().StringVar(&cfg.CSVPath, "csv", DefaultCSVPath, "Path to CSV file")
	indexBuildCmd.Flags().StringVar(&cfg.IndexPath, "index", DefaultIndexPath, "Path to index file")
	indexBuildCmd.Flags().IntVarP(&cfg.Workers, "workers", "w", 0, "Number of workers (0=auto)")
}

// indexSearchCmd represents the index search command.
var indexSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search using index",
	Long:  `Search records using an existing index file.`,
	Run: func(cmd *cobra.Command, args []string) {
		runIndexSearch()
	},
}

func init() {
	indexCmd.AddCommand(indexSearchCmd)

	indexSearchCmd.Flags().StringVar(&cfg.IndexPath, "index", DefaultIndexPath, "Path to index file")
	indexSearchCmd.Flags().StringVar(&cfg.DNI, "dni", "", "Search by DNI")
	indexSearchCmd.Flags().StringVar(&cfg.PrimerNombre, "primer-nombre", "", "Search by first name")
	indexSearchCmd.Flags().StringVar(&cfg.SegundoNombre, "segundo-nombre", "", "Search by second name")
	indexSearchCmd.Flags().StringVar(&cfg.PrimerApellido, "primer-apellido", "", "Search by first last name")
	indexSearchCmd.Flags().StringVar(&cfg.SegundoApellido, "segundo-apellido", "", "Search by second last name")
	indexSearchCmd.Flags().StringVar((*string)(&cfg.Logic), "logic", "AND", "Search logic: AND, OR")
}

func runIndexBuild() {
	fmt.Printf("Building index from: %s\n", cfg.CSVPath)
	fmt.Printf("Workers: %d\n", cfg.Workers)

	// Check file exists
	if _, err := os.Stat(cfg.CSVPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("CSV file not found: %s", cfg.CSVPath), nil)
	}

	index, err := internal.BuildIndex(cfg.CSVPath, cfg.Workers)
	if err != nil {
		exitWithError("Error building index", err)
	}

	fmt.Printf("Index built: %d records\n", index.TotalRecords)

	if err := index.Save(cfg.IndexPath); err != nil {
		exitWithError("Error saving index", err)
	}
	fmt.Printf("Index saved to: %s\n", cfg.IndexPath)
}

func runIndexSearch() {
	fmt.Printf("Loading index from: %s\n", cfg.IndexPath)

	// Check file exists
	if _, err := os.Stat(cfg.IndexPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("Index file not found: %s", cfg.IndexPath), nil)
	}

	index, err := internal.LoadIndex(cfg.IndexPath)
	if err != nil {
		exitWithError("Error loading index", err)
	}
	fmt.Printf("Index loaded: %d records\n", index.TotalRecords)

	// Collect conditions
	conditions := collectConditions()
	results := index.SearchAll(conditions, cfg.Logic)
	printResults(results)
}

func collectConditions() []SearchCondition {
	var conditions []SearchCondition

	if cfg.DNI != "" {
		pattern := cfg.DNIPattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "dni", Value: cfg.DNI, Pattern: pattern})
	}

	if cfg.PrimerNombre != "" {
		pattern := cfg.PrimerNombrePattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "primer_nombre", Value: cfg.PrimerNombre, Pattern: pattern})
	}

	if cfg.SegundoNombre != "" {
		pattern := cfg.SegundoNombrePattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "segundo_nombre", Value: cfg.SegundoNombre, Pattern: pattern})
	}

	if cfg.PrimerApellido != "" {
		pattern := cfg.PrimerApellidoPattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "primer_apellido", Value: cfg.PrimerApellido, Pattern: pattern})
	}

	if cfg.SegundoApellido != "" {
		pattern := cfg.SegundoApellidoPattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "segundo_apellido", Value: cfg.SegundoApellido, Pattern: pattern})
	}

	return conditions
}
