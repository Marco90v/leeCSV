package internal

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// SearchChunkResult contains results from processing a chunk.
type SearchChunkResult struct {
	Records []Record
	Count   int64
}

// SearchStats holds statistics from a parallel search.
type SearchStats struct {
	TotalProcessed  int64
	TotalMatches    int64
	WorkersUsed     int
	ChunksProcessed int64
}

// ReadFile opens a CSV file and returns records.
// Note: For large files, consider using ReadFileWithContext for cancellation support.
func ReadFile(path string) ([]Record, error) {
	return ReadFileWithContext(context.Background(), path)
}

// ReadFileWithContext opens a CSV file and returns records with context cancellation support.
// This is essential for large files (30M+ records) where operations may take significant time.
func ReadFileWithContext(ctx context.Context, path string) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file %s: %w", path, err)
	}
	defer file.Close()

	r := csv.NewReader(file)
	r.Comma = ';'
	r.Comment = '#'

	// Skip first line (header)
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	var records []Record
	for {
		// Check for cancellation periodically
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("cancelled: %w", ctx.Err())
		default:
		}

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

// SearchCSVStreaming performs a streaming parallel search on a CSV file.
// Uses a pipeline pattern: one reader feeds workers via channels.
// Does NOT load all records into memory.
func SearchCSVStreaming(ctx context.Context, csvPath string, params SearchParams) ([]Record, *SearchStats, error) {
	workers := params.Workers
	if workers <= 0 {
		workers = 4
	}

	// Open file
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Channel for records from reader to workers
	// Buffer size affects memory usage - larger = more memory but faster
	recordChan := make(chan Record, workers*100)

	// Channel for results from workers
	resultChan := make(chan []Record, workers)

	// Stats - using atomic for shared access
	var totalProcessed int64
	var totalMatches int64

	// Start reader goroutine
	var readerErr error
	go func() {
		defer close(recordChan)

		reader := csv.NewReader(file)
		reader.Comma = ';'
		reader.Comment = '#'

		// Skip header
		if _, err := reader.Read(); err != nil {
			readerErr = fmt.Errorf("read header: %w", err)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			record, err := reader.Read()
			if err == io.EOF {
				return
			}
			if err != nil {
				// Skip malformed records
				continue
			}

			// Skip records with insufficient fields
			if len(record) < 7 {
				continue
			}

			rec := parseRecord(record)

			// Send to workers - block if channel is full (backpressure)
			select {
			case recordChan <- rec:
			case <-ctx.Done():
				return
			}

			atomic.AddInt64(&totalProcessed, 1)
		}
	}()

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			localResults := []Record{}

			for {
				select {
				case <-ctx.Done():
					return
				case rec, ok := <-recordChan:
					if !ok {
						// Send remaining results
						if len(localResults) > 0 {
							resultChan <- localResults
						}
						return
					}

					if matchesSearch(rec, params) {
						localResults = append(localResults, rec)
						atomic.AddInt64(&totalMatches, 1)
					}

					// Send results periodically to avoid holding too much
					if len(localResults) >= 100 {
						resultChan <- localResults
						localResults = nil
					}
				}
			}
		}(w)
	}

	// Wait for workers and close result channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Progress reporter
	progressCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-progressCtx.Done():
				return
			case <-ticker.C:
				current := atomic.LoadInt64(&totalProcessed)
				matches := atomic.LoadInt64(&totalMatches)
				fmt.Printf("\rProcessed: %d records, Matches: %d", current, matches)
			}
		}
	}()

	// Collect results
	var allRecords []Record
	for results := range resultChan {
		allRecords = append(allRecords, results...)
	}

	cancel()

	// Check for reader errors
	if readerErr != nil {
		return allRecords, &SearchStats{
			TotalProcessed: atomic.LoadInt64(&totalProcessed),
			TotalMatches:   atomic.LoadInt64(&totalMatches),
			WorkersUsed:    workers,
		}, readerErr
	}

	stats := &SearchStats{
		TotalProcessed: atomic.LoadInt64(&totalProcessed),
		TotalMatches:   atomic.LoadInt64(&totalMatches),
		WorkersUsed:    workers,
	}

	return allRecords, stats, nil
}
