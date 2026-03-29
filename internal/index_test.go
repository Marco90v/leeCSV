package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewIndex(t *testing.T) {
	index := NewIndex()

	if index.DNI == nil {
		t.Error("DNI map should be initialized")
	}
	if index.PrimerNombre == nil {
		t.Error("PrimerNombre map should be initialized")
	}
	if index.SegundoNombre == nil {
		t.Error("SegundoNombre map should be initialized")
	}
	if index.PrimerApellido == nil {
		t.Error("PrimerApellido map should be initialized")
	}
	if index.SegundoApellido == nil {
		t.Error("SegundoApellido map should be initialized")
	}
	if index.TotalRecords != 0 {
		t.Errorf("TotalRecords = %d; want 0", index.TotalRecords)
	}
}

func TestBuildIndex(t *testing.T) {
	// Create temp CSV
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")

	content := `nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
V;12345678;GARCIA;PEREZ;JUAN;CARLOS;100
V;87654321;LOPEZ;;MARIA;;200
V;11223344;GOMEZ;RUIZ;ANA;BELEN;300`

	err := os.WriteFile(csvPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test CSV: %v", err)
	}

	index, err := BuildIndex(csvPath, 1)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v; want nil", err)
	}

	if index.TotalRecords != 3 {
		t.Errorf("TotalRecords = %d; want 3", index.TotalRecords)
	}

	// Check DNI index
	if len(index.DNI) != 3 {
		t.Errorf("DNI index size = %d; want 3", len(index.DNI))
	}

	// Check name indexes
	if len(index.PrimerNombre) != 3 {
		t.Errorf("PrimerNombre index size = %d; want 3", len(index.PrimerNombre))
	}
}

func TestIndexSaveAndLoad(t *testing.T) {
	// Create index with known data
	index := NewIndex()
	index.TotalRecords = 2
	index.DNI["12345678"] = []Record{
		{DNI: "12345678", Primer_Nombre: "JUAN"},
	}
	index.PrimerNombre["JUAN"] = []Record{
		{DNI: "12345678", Primer_Nombre: "JUAN"},
	}

	// Save to temp file
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "test_index.json")

	err := index.Save(indexPath)
	if err != nil {
		t.Fatalf("Index.Save() error = %v; want nil", err)
	}

	// Load and verify
	loaded, err := LoadIndex(indexPath)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v; want nil", err)
	}

	if loaded.TotalRecords != 2 {
		t.Errorf("loaded.TotalRecords = %d; want 2", loaded.TotalRecords)
	}

	if len(loaded.DNI) != 1 {
		t.Errorf("loaded.DNI size = %d; want 1", len(loaded.DNI))
	}

	records, ok := loaded.DNI["12345678"]
	if !ok {
		t.Error("expected to find DNI 12345678 in loaded index")
	}
	if len(records) != 1 {
		t.Errorf("found %d records; want 1", len(records))
	}
}

func TestIndexSaveLoadJSONRoundtrip(t *testing.T) {
	original := NewIndex()
	original.TotalRecords = 5
	original.DNI["A1"] = []Record{{DNI: "A1", Primer_Nombre: "Name1"}}
	original.DNI["B2"] = []Record{{DNI: "B2", Primer_Nombre: "Name2"}}
	original.PrimerNombre["Name1"] = []Record{{DNI: "A1", Primer_Nombre: "Name1"}}

	// Marshal and unmarshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v; want nil", err)
	}

	var loaded Index
	err = json.Unmarshal(data, &loaded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v; want nil", err)
	}

	if loaded.TotalRecords != 5 {
		t.Errorf("TotalRecords = %d; want 5", loaded.TotalRecords)
	}
	if len(loaded.DNI) != 2 {
		t.Errorf("DNI count = %d; want 2", len(loaded.DNI))
	}
}

func TestIndexSearchAll(t *testing.T) {
	index := NewIndex()
	index.TotalRecords = 3

	// Add test records
	rec1 := Record{DNI: "1", Primer_Nombre: "JUAN", Primer_Apellido: "GARCIA"}
	rec2 := Record{DNI: "2", Primer_Nombre: "MARIA", Primer_Apellido: "LOPEZ"}
	rec3 := Record{DNI: "3", Primer_Nombre: "JUAN", Primer_Apellido: "LOPEZ"}

	index.DNI["1"] = []Record{rec1}
	index.DNI["2"] = []Record{rec2}
	index.DNI["3"] = []Record{rec3}
	index.PrimerNombre["JUAN"] = []Record{rec1, rec3}
	index.PrimerNombre["MARIA"] = []Record{rec2}
	index.PrimerApellido["GARCIA"] = []Record{rec1}
	index.PrimerApellido["LOPEZ"] = []Record{rec2, rec3}

	tests := []struct {
		name       string
		conditions []SearchCondition
		logic      SearchLogic
		expected   int
	}{
		{
			name:       "empty conditions returns nil",
			conditions: []SearchCondition{},
			logic:      LogicAND,
			expected:   0,
		},
		{
			name: "single condition DNI",
			conditions: []SearchCondition{
				{Field: "dni", Value: "1"},
			},
			logic:    LogicAND,
			expected: 1,
		},
		{
			name: "single condition primer_nombre",
			conditions: []SearchCondition{
				{Field: "primer_nombre", Value: "JUAN"},
			},
			logic:    LogicAND,
			expected: 2,
		},
		{
			name: "AND logic - both must match",
			conditions: []SearchCondition{
				{Field: "primer_nombre", Value: "JUAN"},
				{Field: "primer_apellido", Value: "GARCIA"},
			},
			logic:    LogicAND,
			expected: 1,
		},
		{
			name: "OR logic - either matches",
			conditions: []SearchCondition{
				{Field: "dni", Value: "1"},
				{Field: "dni", Value: "2"},
			},
			logic:    LogicOR,
			expected: 2,
		},
		{
			name: "AND no match",
			conditions: []SearchCondition{
				{Field: "primer_nombre", Value: "JUAN"},
				{Field: "primer_apellido", Value: "LOPEZ"},
			},
			logic:    LogicAND,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := index.SearchAll(tt.conditions, tt.logic)
			if len(results) != tt.expected {
				t.Errorf("SearchAll() returned %d; want %d", len(results), tt.expected)
			}
		})
	}
}

func TestIntersectRecords(t *testing.T) {
	a := []Record{
		{DNI: "1", Primer_Nombre: "A"},
		{DNI: "2", Primer_Nombre: "B"},
		{DNI: "3", Primer_Nombre: "C"},
	}
	b := []Record{
		{DNI: "2", Primer_Nombre: "B"},
		{DNI: "3", Primer_Nombre: "C"},
		{DNI: "4", Primer_Nombre: "D"},
	}

	result := intersectRecords(a, b)

	if len(result) != 2 {
		t.Errorf("intersectRecords() returned %d; want 2", len(result))
	}

	// Check that DNI 2 and 3 are in result
	found := make(map[string]bool)
	for _, r := range result {
		found[r.DNI] = true
	}
	if !found["2"] || !found["3"] || found["1"] || found["4"] {
		t.Error("intersectRecords() returned wrong records")
	}
}

func TestIntersectRecordsEmpty(t *testing.T) {
	// Empty first slice
	result := intersectRecords([]Record{}, []Record{{DNI: "1"}})
	if result != nil {
		t.Error("intersectRecords() with empty first should return nil")
	}

	// Empty second slice
	result = intersectRecords([]Record{{DNI: "1"}}, []Record{})
	if result != nil {
		t.Error("intersectRecords() with empty second should return nil")
	}
}

func TestUnionRecords(t *testing.T) {
	a := []Record{
		{DNI: "1", Primer_Nombre: "A"},
		{DNI: "2", Primer_Nombre: "B"},
	}
	b := []Record{
		{DNI: "2", Primer_Nombre: "B"}, // Duplicate
		{DNI: "3", Primer_Nombre: "C"},
	}

	result := unionRecords(a, b)

	if len(result) != 3 {
		t.Errorf("unionRecords() returned %d; want 3", len(result))
	}

	// Check no duplicates
	seen := make(map[string]bool)
	for _, r := range result {
		if seen[r.DNI] {
			t.Errorf("unionRecords() contains duplicate DNI: %s", r.DNI)
		}
		seen[r.DNI] = true
	}
}

func TestUnionRecordsEmpty(t *testing.T) {
	result := unionRecords([]Record{}, []Record{})
	if result != nil {
		t.Error("unionRecords() with both empty should return nil")
	}

	result = unionRecords([]Record{{DNI: "1"}}, []Record{})
	if len(result) != 1 {
		t.Errorf("unionRecords() with one empty returned %d; want 1", len(result))
	}
}

func TestLoadIndexError(t *testing.T) {
	_, err := LoadIndex("/nonexistent/path/index.json")
	if err == nil {
		t.Error("LoadIndex() expected error for nonexistent file; got nil")
	}
}
