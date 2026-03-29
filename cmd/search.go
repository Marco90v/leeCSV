package cmd

import (
	"context"
	"fmt"
	"os"

	"go/csv/internal"

	"github.com/spf13/cobra"
)

// searchCmd represents the search command.
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search records in CSV file",
	Long: `Search records using direct CSV reading with parallel processing.
This mode processes the file in chunks using multiple workers.
Does NOT load all records into memory - ideal for large files (30M+ records).`,
	Run: func(cmd *cobra.Command, args []string) {
		runSearch()
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().StringVar(&cfg.DNI, "dni", "", "Search by DNI (exact match)")
	searchCmd.Flags().StringVar(&cfg.PrimerNombre, "primer-nombre", "", "Search by first name")
	searchCmd.Flags().StringVar(&cfg.SegundoNombre, "segundo-nombre", "", "Search by second name")
	searchCmd.Flags().StringVar(&cfg.PrimerApellido, "primer-apellido", "", "Search by first last name")
	searchCmd.Flags().StringVar(&cfg.SegundoApellido, "segundo-apellido", "", "Search by second last name")
	searchCmd.Flags().StringVar((*string)(&cfg.Logic), "logic", "AND", "Search logic: AND, OR")
}

func runSearch() {
	params := getSearchParams()

	fmt.Printf("CSV mode (streaming) - searching in: %s\n", params.CSVPath)
	fmt.Printf("Workers: %d\n", params.Workers)

	// Check file exists
	if _, err := os.Stat(params.CSVPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("CSV file not found: %s", params.CSVPath), nil)
	}

	// Create context for cancellation support
	ctx := context.Background()

	internalParams := internal.SearchParams{
		DNI:             params.DNI,
		PrimerNombre:    params.PrimerNombre,
		SegundoNombre:   params.SegundoNombre,
		PrimerApellido:  params.PrimerApellido,
		SegundoApellido: params.SegundoApellido,
		Workers:         params.Workers,
	}

	// Use streaming search - doesn't load all records into memory
	results, stats, err := internal.SearchCSVStreaming(ctx, params.CSVPath, internalParams)
	if err != nil {
		exitWithError("Error searching CSV", err)
	}

	// Print stats
	fmt.Printf("\n--- Search Stats ---\n")
	fmt.Printf("Records processed: %d\n", stats.TotalProcessed)
	fmt.Printf("Matches found:    %d\n", stats.TotalMatches)
	fmt.Printf("Workers used:     %d\n", stats.WorkersUsed)
	fmt.Printf("--------------------\n")

	// Print results
	printResults(results)
}
