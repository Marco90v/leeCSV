package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
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
	Workers    int

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
	flag.StringVar(&config.CSVPath, "csv", "./nacional.csv", "Path to CSV file")
	flag.StringVar(&config.IndexPath, "index", "./index.json", "Path to index file")
	flag.StringVar(&config.DBPath, "db", "./data.db", "Path to SQLite database")
	flag.StringVar((*string)(&config.Mode), "mode", "csv", "Search mode: csv, index, sqlite")
	flag.BoolVar(&config.BuildIndex, "build", false, "Build index from CSV (index mode only)")
	flag.IntVar(&config.Workers, "workers", 0, "Number of workers (0 = auto)")

	flag.StringVar(&config.DNI, "dni", "", "Search by DNI (exact match)")
	flag.StringVar(&config.PrimerNombre, "primerNombre", "", "Search by first name")
	flag.StringVar(&config.SegundoNombre, "segundoNombre", "", "Search by second name")
	flag.StringVar(&config.PrimerApellido, "primerApellido", "", "Search by first last name")
	flag.StringVar(&config.SegundoApellido, "segundoApellido", "", "Search by second last name")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -mode=csv -csv=data.csv -dni=12345678\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode=index -build -csv=data.csv -index=idx.json\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode=index -index=idx.json -dni=12345678\n", os.Args[0])
	}
}

func main() {
	flag.Parse()

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
		fmt.Println("SQLite mode not yet implemented")
		os.Exit(1)
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

// searchIndex searches using the in-memory index.
func searchIndex(idx *Index) []Record {
	if config.DNI != "" {
		pattern := config.DNIPattern
		if pattern == "" {
			pattern = PatternExact
		}
		return idx.SearchByDNIWithPattern(config.DNI, pattern)
	}

	if config.PrimerNombre != "" {
		pattern := config.PrimerNombrePattern
		if pattern == "" {
			pattern = PatternExact
		}
		return idx.SearchByFieldWithPattern("primerNombre", config.PrimerNombre, pattern)
	}

	if config.SegundoNombre != "" {
		pattern := config.SegundoNombrePattern
		if pattern == "" {
			pattern = PatternExact
		}
		return idx.SearchByFieldWithPattern("segundoNombre", config.SegundoNombre, pattern)
	}

	if config.PrimerApellido != "" {
		pattern := config.PrimerApellidoPattern
		if pattern == "" {
			pattern = PatternExact
		}
		return idx.SearchByFieldWithPattern("primerApellido", config.PrimerApellido, pattern)
	}

	if config.SegundoApellido != "" {
		pattern := config.SegundoApellidoPattern
		if pattern == "" {
			pattern = PatternExact
		}
		return idx.SearchByFieldWithPattern("segundoApellido", config.SegundoApellido, pattern)
	}

	return nil
}

// parseSearchFlag parses a flag value with pattern suffix like "value:contains" or "value:startswith".
// Returns the value and the pattern.
func parseSearchFlag(flagVal string) (value string, pattern SearchPattern) {
	if flagVal == "" {
		return "", ""
	}

	// Check for pattern suffix: "value:pattern"
	if idx := strings.LastIndex(flagVal, ":"); idx > 0 {
		prefix := flagVal[:idx]
		suffix := flagVal[idx+1:]

		// Check if suffix is a valid pattern
		switch SearchPattern(suffix) {
		case PatternExact, PatternContains, PatternStartsWith, PatternRegex:
			return prefix, SearchPattern(suffix)
		}
	}

	return flagVal, PatternExact
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
