package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
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

	// Mutex for thread-safe operations (not serialized)
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

// BuildIndex builds an index from CSV records.
func BuildIndex(csvPath string, workers int) (*Index, error) {
	records, err := ReadFile(csvPath)
	if err != nil {
		return nil, fmt.Errorf("read CSV: %w", err)
	}

	index := NewIndex()
	index.TotalRecords = len(records)

	// Build indexes
	for _, r := range records {
		index.DNI[r.DNI] = append(index.DNI[r.DNI], r)
		index.PrimerNombre[r.Primer_Nombre] = append(index.PrimerNombre[r.Primer_Nombre], r)
		index.SegundoNombre[r.Segundo_Nombre] = append(index.SegundoNombre[r.Segundo_Nombre], r)
		index.PrimerApellido[r.Primer_Apellido] = append(index.PrimerApellido[r.Primer_Apellido], r)
		index.SegundoApellido[r.Segundo_Apellido] = append(index.SegundoApellido[r.Segundo_Apellido], r)
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
