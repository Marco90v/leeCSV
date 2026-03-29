package internal

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sync"
)

// ReadFile opens a CSV file and returns records.
func ReadFile(path string) ([]Record, error) {
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

	var records []Record
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV record: %w", err)
		}
		records = append(records, parseRecord(record))
	}

	return records, nil
}

// SearchParams holds search parameters.
type SearchParams struct {
	DNI             string
	PrimerNombre    string
	SegundoNombre   string
	PrimerApellido  string
	SegundoApellido string
	Workers         int
}

// SearchCSV performs a search on CSV records.
func SearchCSV(records []Record, params SearchParams) []Record {
	var results []Record

	for _, r := range records {
		if matchesSearch(r, params) {
			results = append(results, r)
		}
	}

	return results
}

// SearchCSVConcurrent performs concurrent search on CSV records.
func SearchCSVConcurrent(records []Record, params SearchParams) []Record {
	workers := params.Workers
	if workers <= 0 {
		workers = 4
	}

	input := make(chan Record, workers)
	output := make(chan Record)
	var wg sync.WaitGroup

	worker := func(jobs <-chan Record, results chan<- Record) {
		for {
			select {
			case job, ok := <-jobs:
				if !ok {
					return
				}
				if matchesSearch(job, params) {
					results <- job
				}
			}
		}
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(input, output)
		}()
	}

	go func() {
		for _, r := range records {
			input <- r
		}
		close(input)
	}()

	go func() {
		wg.Wait()
		close(output)
	}()

	var results []Record
	for r := range output {
		results = append(results, r)
	}

	return results
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

// matchesSearch checks if a record matches the search criteria.
func matchesSearch(rec Record, params SearchParams) bool {
	if params.DNI != "" && rec.DNI != params.DNI {
		return false
	}
	if params.PrimerNombre != "" && rec.Primer_Nombre != params.PrimerNombre {
		return false
	}
	if params.SegundoNombre != "" && rec.Segundo_Nombre != params.SegundoNombre {
		return false
	}
	if params.PrimerApellido != "" && rec.Primer_Apellido != params.PrimerApellido {
		return false
	}
	if params.SegundoApellido != "" && rec.Segundo_Apellido != params.SegundoApellido {
		return false
	}
	return true
}
