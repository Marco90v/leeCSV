package internal

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Index represents an in-memory index for fast CSV lookups.
type Index struct {
	// DNI lookup: map[dni]records
	DNI map[string][]Record `json:"dni"`

	// Name indexes
	PrimerNombre    map[string][]Record `json:"primer_nombre"`
	SegundoNombre   map[string][]Record `json:"segundo_nombre"`
	PrimerApellido  map[string][]Record `json:"primer_apellido"`
	SegundoApellido map[string][]Record `json:"segundo_apellido"`

	// Metadata
	TotalRecords int `json:"total_records"`

	// Mutex for thread-safe operations
	mu sync.RWMutex `json:"-"`
}

// NewIndex creates a new Index structure.
func NewIndex() *Index {
	return &Index{
		DNI:             make(map[string][]Record),
		PrimerNombre:    make(map[string][]Record),
		SegundoNombre:   make(map[string][]Record),
		PrimerApellido:  make(map[string][]Record),
		SegundoApellido: make(map[string][]Record),
	}
}

// BuildIndex builds an index from CSV records using parallel workers.
// Uses streaming to avoid loading all records into memory.
// Uses context for cancellation support.
func BuildIndex(csvPath string, workers int) (*Index, error) {
	return BuildIndexStreaming(context.Background(), csvPath, workers)
}

// BuildIndexStreaming builds an index from CSV using streaming.
// Does NOT load all records into memory - streams through the file and builds index incrementally.
// Uses a single worker with mutex for thread safety - simpler and memory-efficient.
func BuildIndexStreaming(ctx context.Context, csvPath string, workers int) (*Index, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Create index upfront
	index := NewIndex()

	// Use mutex for thread-safe index updates
	var mu sync.Mutex

	// Total records counter
	var totalRecords int64

	// Progress reporting
	var progressMu sync.Mutex
	lastProgress := time.Now()

	// Reader goroutine - streams records from CSV
	readerErr := make(chan error, 1)
	go func() {
		reader := csv.NewReader(file)
		reader.Comma = ';'

		// Skip header
		if _, err := reader.Read(); err != nil {
			readerErr <- fmt.Errorf("read header: %w", err)
			return
		}

		for {
			select {
			case <-ctx.Done():
				readerErr <- ctx.Err()
				return
			default:
			}

			record, err := reader.Read()
			if err == io.EOF {
				readerErr <- nil
				return
			}
			if err != nil || len(record) < 7 {
				continue
			}

			rec := parseRecord(record)

			// Add to index with mutex protection
			mu.Lock()
			index.DNI[rec.DNI] = append(index.DNI[rec.DNI], rec)
			index.PrimerNombre[rec.Primer_Nombre] = append(index.PrimerNombre[rec.Primer_Nombre], rec)
			index.SegundoNombre[rec.Segundo_Nombre] = append(index.SegundoNombre[rec.Segundo_Nombre], rec)
			index.PrimerApellido[rec.Primer_Apellido] = append(index.PrimerApellido[rec.Primer_Apellido], rec)
			index.SegundoApellido[rec.Segundo_Apellido] = append(index.SegundoApellido[rec.Segundo_Apellido], rec)
			mu.Unlock()

			atomic.AddInt64(&totalRecords, 1)

			// Progress report every 2 seconds
			progressMu.Lock()
			if time.Since(lastProgress) > 2*time.Second {
				fmt.Printf("\rIndexed: %d records...", atomic.LoadInt64(&totalRecords))
				lastProgress = time.Now()
			}
			progressMu.Unlock()
		}
	}()

	// Wait for reader to finish
	err = <-readerErr
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("reader error: %w", err)
	}

	// Update total records
	index.TotalRecords = int(atomic.LoadInt64(&totalRecords))

	fmt.Printf("\rIndexed: %d records... done!\n", index.TotalRecords)

	return index, nil
}

// buildIndexSequential builds the index without parallelism (better for smaller files).
func buildIndexSequential(records []Record) *Index {
	index := NewIndex()
	index.TotalRecords = len(records)

	for _, r := range records {
		index.DNI[r.DNI] = append(index.DNI[r.DNI], r)
		index.PrimerNombre[r.Primer_Nombre] = append(index.PrimerNombre[r.Primer_Nombre], r)
		index.SegundoNombre[r.Segundo_Nombre] = append(index.SegundoNombre[r.Segundo_Nombre], r)
		index.PrimerApellido[r.Primer_Apellido] = append(index.PrimerApellido[r.Primer_Apellido], r)
		index.SegundoApellido[r.Segundo_Apellido] = append(index.SegundoApellido[r.Segundo_Apellido], r)
	}

	return index
}

// buildIndexParallel builds the index using multiple workers.
func buildIndexParallel(ctx context.Context, records []Record, workers int) (*Index, error) {
	index := NewIndex()
	index.TotalRecords = len(records)

	// Partition records into chunks for each worker
	chunkSize := len(records) / workers
	if chunkSize < 1000 {
		chunkSize = 1000 // Minimum chunk size
	}

	type partialIndex struct {
		dni             map[string][]Record
		primerNombre    map[string][]Record
		segundoNombre   map[string][]Record
		primerApellido  map[string][]Record
		segundoApellido map[string][]Record
	}

	// Create work items
	type workItem struct {
		id    int
		start int
		end   int
	}

	workChan := make(chan workItem, workers)
	results := make(chan partialIndex, workers)
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-workChan:
					if !ok {
						return
					}
					// Build partial index
					pi := partialIndex{
						dni:             make(map[string][]Record),
						primerNombre:    make(map[string][]Record),
						segundoNombre:   make(map[string][]Record),
						primerApellido:  make(map[string][]Record),
						segundoApellido: make(map[string][]Record),
					}
					for _, r := range records[item.start:item.end] {
						pi.dni[r.DNI] = append(pi.dni[r.DNI], r)
						pi.primerNombre[r.Primer_Nombre] = append(pi.primerNombre[r.Primer_Nombre], r)
						pi.segundoNombre[r.Segundo_Nombre] = append(pi.segundoNombre[r.Segundo_Nombre], r)
						pi.primerApellido[r.Primer_Apellido] = append(pi.primerApellido[r.Primer_Apellido], r)
						pi.segundoApellido[r.Segundo_Apellido] = append(pi.segundoApellido[r.Segundo_Apellido], r)
					}
					results <- pi
				}
			}
		}()
	}

	// Send work items
	go func() {
		for i := 0; i < len(records); i += chunkSize {
			end := i + chunkSize
			if end > len(records) {
				end = len(records)
			}
			workChan <- workItem{start: i, end: end}
		}
		close(workChan)
	}()

	// Wait for all workers and collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Merge partial indexes into main index
	for pi := range results {
		for k, v := range pi.dni {
			index.DNI[k] = append(index.DNI[k], v...)
		}
		for k, v := range pi.primerNombre {
			index.PrimerNombre[k] = append(index.PrimerNombre[k], v...)
		}
		for k, v := range pi.segundoNombre {
			index.SegundoNombre[k] = append(index.SegundoNombre[k], v...)
		}
		for k, v := range pi.primerApellido {
			index.PrimerApellido[k] = append(index.PrimerApellido[k], v...)
		}
		for k, v := range pi.segundoApellido {
			index.SegundoApellido[k] = append(index.SegundoApellido[k], v...)
		}
	}

	return index, nil
}

// Save saves the index to a JSON file.
func (idx *Index) Save(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(idx)
}

// LoadIndex loads an index from a JSON file.
func LoadIndex(path string) (*Index, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	var index Index
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&index); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	return &index, nil
}

// SearchAll searches the index with multiple conditions.
func (idx *Index) SearchAll(conditions []SearchCondition, logic SearchLogic) []Record {
	if len(conditions) == 0 {
		return nil
	}

	// Get results from first condition
	results := idx.searchByCondition(conditions[0])

	// Apply remaining conditions
	for i := 1; i < len(conditions); i++ {
		more := idx.searchByCondition(conditions[i])
		if logic == LogicAND {
			results = intersectRecords(results, more)
		} else {
			results = unionRecords(results, more)
		}
	}

	return results
}

// searchByCondition searches by a single condition.
func (idx *Index) searchByCondition(cond SearchCondition) []Record {
	var records []Record

	switch cond.Field {
	case "dni":
		records = idx.DNI[cond.Value]
	case "primer_nombre":
		records = idx.PrimerNombre[cond.Value]
	case "segundo_nombre":
		records = idx.SegundoNombre[cond.Value]
	case "primer_apellido":
		records = idx.PrimerApellido[cond.Value]
	case "segundo_apellido":
		records = idx.SegundoApellido[cond.Value]
	}

	return records
}

// intersectRecords returns records that exist in both slices.
func intersectRecords(a, b []Record) []Record {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}

	// Build a set of record signatures from b
	bSet := make(map[string]bool)
	for _, r := range b {
		bSet[r.DNI] = true
	}

	// Find intersection
	var result []Record
	for _, r := range a {
		if bSet[r.DNI] {
			result = append(result, r)
		}
	}
	return result
}

// unionRecords returns all unique records from both slices.
func unionRecords(a, b []Record) []Record {
	seen := make(map[string]bool)
	var result []Record

	for _, r := range a {
		if !seen[r.DNI] {
			seen[r.DNI] = true
			result = append(result, r)
		}
	}
	for _, r := range b {
		if !seen[r.DNI] {
			seen[r.DNI] = true
			result = append(result, r)
		}
	}
	return result
}
