package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// SearchEngine defines the interface for all search implementations.
type SearchEngine interface {
	SearchAll(conditions []SearchCondition, logic SearchLogic) ([]Record, error)
}

// SearchPattern defines how a search term should be matched.
type SearchPattern string

const (
	PatternExact      SearchPattern = "exact"
	PatternContains   SearchPattern = "contains"
	PatternStartsWith SearchPattern = "startswith"
	PatternRegex      SearchPattern = "regex"
)

// Index represents an in-memory index for fast CSV lookups.
// Uses multiple lookup maps for different search patterns.
type Index struct {
	// Primary lookup: DNI -> records (exact match)
	ByDNI map[string][]Record `json:"dni"`

	// Secondary lookups for name searches
	ByFirstName  map[string][]string `json:"first_name"`
	ByLastName   map[string][]string `json:"last_name"`
	BySecondName map[string][]string `json:"second_name"`
	BySecondLast map[string][]string `json:"second_last"`

	// Metadata
	TotalRecords int `json:"total_records"`

	// Mutex for thread-safe operations (not serialized)
	mu sync.RWMutex `json:"-"`
}

// NewIndex creates a new Index structure.
func NewIndex() *Index {
	return &Index{
		ByDNI:        make(map[string][]Record),
		ByFirstName:  make(map[string][]string),
		ByLastName:   make(map[string][]string),
		BySecondName: make(map[string][]string),
		BySecondLast: make(map[string][]string),
	}
}

// BuildIndex reads a CSV file and builds an in-memory index.
// Uses concurrent workers for performance with large files.
func BuildIndex(csvPath string, workers int) (*Index, error) {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'

	// Skip header
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	index := NewIndex()

	// Channel for feeding records to workers
	recordCh := make(chan []string, workers*2)
	errCh := make(chan error, 1)

	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for record := range recordCh {
				rec := parseRecord(record)
				index.addRecord(rec)
			}
		}()
	}

	// Feed records
	go func() {
		defer close(recordCh)
		for {
			record, err := reader.Read()
			if err == io.EOF {
				return
			}
			if err != nil {
				errCh <- fmt.Errorf("read CSV record: %w", err)
				return
			}
			recordCh <- record
		}
	}()

	// Wait for workers and check for errors
	go func() {
		wg.Wait()
		close(errCh)
	}()

	if err := <-errCh; err != nil {
		return nil, err
	}

	index.TotalRecords = len(index.ByDNI)
	return index, nil
}

// addRecord adds a record to all index maps (thread-safe).
func (idx *Index) addRecord(rec Record) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// DNI exact lookup
	if rec.DNI != "" {
		idx.ByDNI[rec.DNI] = append(idx.ByDNI[rec.DNI], rec)
	}

	// Name lookups (lowercase for case-insensitive search)
	if rec.Primer_Nombre != "" {
		idx.ByFirstName[normalize(rec.Primer_Nombre)] = append(idx.ByFirstName[normalize(rec.Primer_Nombre)], rec.DNI)
	}
	if rec.Primer_Apellido != "" {
		idx.ByLastName[normalize(rec.Primer_Apellido)] = append(idx.ByLastName[normalize(rec.Primer_Apellido)], rec.DNI)
	}
	if rec.Segundo_Nombre != "" {
		idx.BySecondName[normalize(rec.Segundo_Nombre)] = append(idx.BySecondName[normalize(rec.Segundo_Nombre)], rec.DNI)
	}
	if rec.Segundo_Apellido != "" {
		idx.BySecondLast[normalize(rec.Segundo_Apellido)] = append(idx.BySecondLast[normalize(rec.Segundo_Apellido)], rec.DNI)
	}
}

// Save writes the index to a JSON file.
func (idx *Index) Save(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create index file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(idx); err != nil {
		return fmt.Errorf("encode index: %w", err)
	}
	return nil
}

// Load reads an index from a JSON file.
func LoadIndex(path string) (*Index, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open index file: %w", err)
	}
	defer file.Close()

	index := NewIndex()
	if err := json.NewDecoder(file).Decode(index); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}
	return index, nil
}

// SearchByDNI performs an exact match search by DNI.
func (idx *Index) SearchByDNI(dni string) []Record {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.ByDNI[dni]
}

// SearchByName searches by first name (exact match, case-insensitive).
func (idx *Index) SearchByFirstName(name string) []Record {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	dnis := idx.ByFirstName[normalize(name)]
	return idx.lookupByDNIs(dnis)
}

// SearchByLastName searches by last name (exact match, case-insensitive).
func (idx *Index) SearchByLastName(name string) []Record {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	dnis := idx.ByLastName[normalize(name)]
	return idx.lookupByDNIs(dnis)
}

// lookupByDNIs retrieves full records from a list of DNIs.
func (idx *Index) lookupByDNIs(dnis []string) []Record {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	seen := make(map[string]bool)
	var results []Record

	for _, dni := range dnis {
		if !seen[dni] {
			seen[dni] = true
			results = append(results, idx.ByDNI[dni]...)
		}
	}
	return results
}

// normalize converts a string to lowercase for case-insensitive comparison.
func normalize(s string) string {
	return strings.ToLower(s)
}

// SearchByDNIWithPattern searches by DNI with a specific pattern.
func (idx *Index) SearchByDNIWithPattern(dni string, pattern SearchPattern) []Record {
	switch pattern {
	case PatternExact:
		return idx.ByDNI[dni]
	case PatternContains:
		return idx.searchDNIBySubstring(dni)
	case PatternStartsWith:
		return idx.searchDNIByPrefix(dni)
	case PatternRegex:
		return idx.searchDNIByRegex(dni)
	default:
		return idx.ByDNI[dni]
	}
}

// SearchByFieldWithPattern searches by field name with a specific pattern.
func (idx *Index) SearchByFieldWithPattern(field, value string, pattern SearchPattern) []Record {
	var dnis []string

	switch field {
	case "primerNombre", "primer_nombre":
		dnis = idx.ByFirstName[normalize(value)]
	case "segundoNombre", "segundo_nombre":
		dnis = idx.BySecondName[normalize(value)]
	case "primerApellido", "primer_apellido":
		dnis = idx.ByLastName[normalize(value)]
	case "segundoApellido", "segundo_apellido":
		dnis = idx.BySecondLast[normalize(value)]
	}

	if pattern == PatternExact {
		return idx.lookupByDNIs(dnis)
	}

	// For other patterns, we need to iterate all records
	return idx.searchAllByFieldPattern(field, value, pattern)
}

// searchDNIBySubstring searches for DNI containing a substring.
func (idx *Index) searchDNIBySubstring(substr string) []Record {
	substr = strings.ToLower(substr)
	var results []Record
	seen := make(map[string]bool)

	for dni, recs := range idx.ByDNI {
		if strings.Contains(strings.ToLower(dni), substr) {
			if !seen[dni] {
				seen[dni] = true
				results = append(results, recs...)
			}
		}
	}
	return results
}

// searchDNIByPrefix searches for DNI starting with a prefix.
func (idx *Index) searchDNIByPrefix(prefix string) []Record {
	prefix = strings.ToLower(prefix)
	var results []Record
	seen := make(map[string]bool)

	for dni, recs := range idx.ByDNI {
		if strings.HasPrefix(strings.ToLower(dni), prefix) {
			if !seen[dni] {
				seen[dni] = true
				results = append(results, recs...)
			}
		}
	}
	return results
}

// searchDNIByRegex searches for DNI matching a regex pattern.
func (idx *Index) searchDNIByRegex(pattern string) []Record {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	var results []Record
	seen := make(map[string]bool)

	for dni, recs := range idx.ByDNI {
		if re.MatchString(dni) {
			if !seen[dni] {
				seen[dni] = true
				results = append(results, recs...)
			}
		}
	}
	return results
}

// searchAllByFieldPattern searches all records by field pattern.
func (idx *Index) searchAllByFieldPattern(field, value string, pattern SearchPattern) []Record {
	var results []Record
	seen := make(map[string]bool)

	for dni, recs := range idx.ByDNI {
		for _, rec := range recs {
			var fieldValue string
			switch field {
			case "primerNombre", "primer_nombre":
				fieldValue = rec.Primer_Nombre
			case "segundoNombre", "segundo_nombre":
				fieldValue = rec.Segundo_Nombre
			case "primerApellido", "primer_apellido":
				fieldValue = rec.Primer_Apellido
			case "segundoApellido", "segundo_apellido":
				fieldValue = rec.Segundo_Apellido
			}

			matches := matchPattern(fieldValue, value, pattern)
			if matches && !seen[dni] {
				seen[dni] = true
				results = append(results, rec)
				break
			}
		}
	}
	return results
}

// matchPattern checks if a value matches the given pattern.
func matchPattern(value, term string, pattern SearchPattern) bool {
	value = strings.ToLower(value)
	term = strings.ToLower(term)

	switch pattern {
	case PatternExact:
		return value == term
	case PatternContains:
		return strings.Contains(value, term)
	case PatternStartsWith:
		return strings.HasPrefix(value, term)
	case PatternRegex:
		re, err := regexp.Compile(term)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	}
	return false
}

// SearchCondition represents a single search condition.
type SearchCondition struct {
	Field   string
	Value   string
	Pattern SearchPattern
}

// SearchLogic defines how multiple conditions are combined.
type SearchLogic string

const (
	LogicAND SearchLogic = "AND"
	LogicOR  SearchLogic = "OR"
)

// SearchAll combines multiple field searches with AND/OR logic.
func (idx *Index) SearchAll(conditions []SearchCondition, logic SearchLogic) []Record {
	if len(conditions) == 0 {
		return nil
	}

	// Get initial results from first condition
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
	switch cond.Field {
	case "dni":
		return idx.SearchByDNIWithPattern(cond.Value, cond.Pattern)
	case "primer_nombre", "primernombre":
		return idx.SearchByFieldWithPattern("primerNombre", cond.Value, cond.Pattern)
	case "segundo_nombre", "segundonombre":
		return idx.SearchByFieldWithPattern("segundoNombre", cond.Value, cond.Pattern)
	case "primer_apellido", "primerapellido":
		return idx.SearchByFieldWithPattern("primerApellido", cond.Value, cond.Pattern)
	case "segundo_apellido", "segundoapellido":
		return idx.SearchByFieldWithPattern("segundoApellido", cond.Value, cond.Pattern)
	default:
		return nil
	}
}

// intersectRecords returns records that exist in both slices.
func intersectRecords(a, b []Record) []Record {
	seen := make(map[string]bool)
	for _, r := range a {
		seen[r.DNI] = true
	}

	var result []Record
	for _, r := range b {
		if seen[r.DNI] {
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
