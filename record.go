package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sync"
)

// Record represents a single CSV record from the national ID database.
// CSV format: Nacionalidad;Cedula;Primer_Apellido;Segundo_Apellido;Primer_Nombre;Segundo_Nombre;Cod_Centro
type Record struct {
	Nacionalidad     string
	DNI              string
	Primer_Apellido  string
	Segundo_Apellido string
	Primer_Nombre    string
	Segundo_Nombre   string
	Cod_Centro       string
}

// readFile opens a CSV file and returns a reader configured for semicolon-separated files.
// It skips the header row. Returns an error if the file cannot be opened or read.
func readFile(path string) (*csv.Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file %s: %w", path, err)
	}

	r := csv.NewReader(file)
	r.Comma = ';'
	r.Comment = '#'

	// Skip first line (header)
	if _, err := r.Read(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	return r, nil
}

// searchCSV performs a search on the CSV file using workers.
func searchCSV(records *csv.Reader) []Record {
	nmWp := config.Workers
	if nmWp <= 0 {
		nmWp = 4
	}

	input := make(chan []string, nmWp)
	output := make(chan Record)
	var wg sync.WaitGroup

	worker := func(jobs <-chan []string, results chan<- Record) {
		for {
			select {
			case job, ok := <-jobs:
				if !ok {
					return
				}
				user := parseRecord(job)
				if matchesSearch(user) {
					results <- user
				}
			}
		}
	}

	for w := 0; w < nmWp; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(input, output)
		}()
	}

	errCh := make(chan error, 1)
	go readRecords(records, input, errCh)
	go func() {
		wg.Wait()
		close(output)
	}()

	// Check for read errors
	select {
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	default:
	}

	var results []Record
	for user := range output {
		results = append(results, user)
	}

	return results
}

// readRecords reads all records from the CSV and sends them to the jobs channel.
// It closes the jobs channel when done or on error.
func readRecords(records *csv.Reader, jobs chan<- []string, errCh chan<- error) {
	defer close(jobs)

	for {
		record, err := records.Read()
		if err == io.EOF {
			return
		}
		if err != nil {
			errCh <- fmt.Errorf("error reading CSV record: %w", err)
			return
		}
		jobs <- record
	}
}

// matchesSearch checks if a record matches the search criteria.
func matchesSearch(rec Record) bool {
	// Direct field access is much faster than reflection
	if config.DNI != "" && rec.DNI != config.DNI {
		return false
	}
	if config.PrimerNombre != "" && rec.Primer_Nombre != config.PrimerNombre {
		return false
	}
	if config.SegundoNombre != "" && rec.Segundo_Nombre != config.SegundoNombre {
		return false
	}
	if config.PrimerApellido != "" && rec.Primer_Apellido != config.PrimerApellido {
		return false
	}
	if config.SegundoApellido != "" && rec.Segundo_Apellido != config.SegundoApellido {
		return false
	}
	return true
}

// parseRecord converts a CSV record slice to a Record struct.
func parseRecord(record []string) Record {
	if len(record) < 6 {
		return Record{}
	}
	return Record{
		Nacionalidad:     record[0],
		DNI:              record[1],
		Primer_Apellido:  record[2],
		Segundo_Apellido: record[3],
		Primer_Nombre:    record[4],
		Segundo_Nombre:   record[5],
		Cod_Centro:       record[6],
	}
}
