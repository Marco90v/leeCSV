package cmd

import (
	"fmt"
	"os"

	"go/csv/internal"

	"github.com/spf13/cobra"
)

// searchCmd represents the search command.
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search records in CSV file",
	Long: `Search records using direct CSV reading.
This mode loads all records into memory and searches sequentially.
Use for small to medium files (< 1M records).`,
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

	fmt.Printf("CSV mode - searching in: %s\n", params.CSVPath)

	// Check file exists
	if _, err := os.Stat(params.CSVPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("CSV file not found: %s", params.CSVPath), nil)
	}

	records, err := internal.ReadFile(params.CSVPath)
	if err != nil {
		exitWithError("Error reading CSV", err)
	}

	internalParams := internal.SearchParams{
		DNI:             params.DNI,
		PrimerNombre:    params.PrimerNombre,
		SegundoNombre:   params.SegundoNombre,
		PrimerApellido:  params.PrimerApellido,
		SegundoApellido: params.SegundoApellido,
		Workers:         params.Workers,
	}
	results := internal.SearchCSV(records, internalParams)
	printResults(results)
}
