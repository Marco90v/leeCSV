package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRecord(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected Record
	}{
		{
			name:  "valid complete record",
			input: []string{"V", "12345678", "GARCIA", "PEREZ", "JUAN", "CARLOS", "100"},
			expected: Record{
				Nacionalidad:     "V",
				DNI:              "12345678",
				Primer_Apellido:  "GARCIA",
				Segundo_Apellido: "PEREZ",
				Primer_Nombre:    "JUAN",
				Segundo_Nombre:   "CARLOS",
				Cod_Centro:       "100",
			},
		},
		{
			name:  "record with empty fields",
			input: []string{"V", "87654321", "LOPEZ", "", "MARIA", "", "200"},
			expected: Record{
				Nacionalidad:     "V",
				DNI:              "87654321",
				Primer_Apellido:  "LOPEZ",
				Segundo_Apellido: "",
				Primer_Nombre:    "MARIA",
				Segundo_Nombre:   "",
				Cod_Centro:       "200",
			},
		},
		{
			name:     "insufficient fields returns zero value",
			input:    []string{"V", "12345678"},
			expected: Record{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRecord(tt.input)
			if got != tt.expected {
				t.Errorf("parseRecord() = %+v; want %+v", got, tt.expected)
			}
		})
	}
}

func TestMatchesSearch(t *testing.T) {
	record := Record{
		Nacionalidad:     "V",
		DNI:              "12345678",
		Primer_Apellido:  "GARCIA",
		Segundo_Apellido: "PEREZ",
		Primer_Nombre:    "JUAN",
		Segundo_Nombre:   "CARLOS",
		Cod_Centro:       "100",
	}

	tests := []struct {
		name     string
		params   SearchParams
		expected bool
	}{
		{
			name:     "empty params matches all",
			params:   SearchParams{},
			expected: true,
		},
		{
			name: "DNI matches",
			params: SearchParams{
				DNI: "12345678",
			},
			expected: true,
		},
		{
			name: "DNI does not match",
			params: SearchParams{
				DNI: "00000000",
			},
			expected: false,
		},
		{
			name: "PrimerNombre matches",
			params: SearchParams{
				PrimerNombre: "JUAN",
			},
			expected: true,
		},
		{
			name: "PrimerNombre does not match",
			params: SearchParams{
				PrimerNombre: "PEDRO",
			},
			expected: false,
		},
		{
			name: "multiple fields match",
			params: SearchParams{
				DNI:          "12345678",
				PrimerNombre: "JUAN",
			},
			expected: true,
		},
		{
			name: "multiple fields one fails",
			params: SearchParams{
				DNI:          "12345678",
				PrimerNombre: "PEDRO",
			},
			expected: false,
		},
		{
			name: "all fields match",
			params: SearchParams{
				DNI:            "12345678",
				PrimerNombre:   "JUAN",
				SegundoNombre:  "CARLOS",
				PrimerApellido: "GARCIA",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesSearch(record, tt.params)
			if got != tt.expected {
				t.Errorf("matchesSearch() = %v; want %v", got, tt.expected)
			}
		})
	}
}

func TestSearchCSV(t *testing.T) {
	records := []Record{
		{DNI: "123", Primer_Nombre: "JUAN", Primer_Apellido: "GARCIA"},
		{DNI: "456", Primer_Nombre: "MARIA", Primer_Apellido: "LOPEZ"},
		{DNI: "789", Primer_Nombre: "PEDRO", Primer_Apellido: "GARCIA"},
	}

	tests := []struct {
		name     string
		params   SearchParams
		expected int
	}{
		{
			name:     "empty search returns all",
			params:   SearchParams{},
			expected: 3,
		},
		{
			name: "find by DNI",
			params: SearchParams{
				DNI: "123",
			},
			expected: 1,
		},
		{
			name: "find by nombre",
			params: SearchParams{
				PrimerNombre: "MARIA",
			},
			expected: 1,
		},
		{
			name: "find by apellido",
			params: SearchParams{
				PrimerApellido: "GARCIA",
			},
			expected: 2,
		},
		{
			name: "no match",
			params: SearchParams{
				DNI: "999",
			},
			expected: 0,
		},
		{
			name: "multiple criteria AND",
			params: SearchParams{
				PrimerNombre:   "JUAN",
				PrimerApellido: "GARCIA",
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SearchCSV(records, tt.params)
			if len(got) != tt.expected {
				t.Errorf("SearchCSV() returned %d; want %d", len(got), tt.expected)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	// Create temp CSV file
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

	records, err := ReadFile(csvPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v; want nil", err)
	}

	if len(records) != 3 {
		t.Errorf("ReadFile() returned %d records; want 3", len(records))
	}

	// Verify first record
	if records[0].DNI != "12345678" {
		t.Errorf("first record DNI = %s; want 12345678", records[0].DNI)
	}
	if records[0].Primer_Nombre != "JUAN" {
		t.Errorf("first record Primer_Nombre = %s; want JUAN", records[0].Primer_Nombre)
	}
}

func TestReadFileError(t *testing.T) {
	_, err := ReadFile("/nonexistent/path/to/file.csv")
	if err == nil {
		t.Error("ReadFile() expected error for nonexistent file; got nil")
	}
}

func TestSearchCSVConcurrent(t *testing.T) {
	// Generate test records
	records := make([]Record, 100)
	for i := 0; i < 100; i++ {
		records[i] = Record{
			DNI:             "10000000",
			Primer_Nombre:   "NAME",
			Primer_Apellido: "LASTNAME",
		}
	}
	records = append(records, Record{
		DNI:             "99999999",
		Primer_Nombre:   "UNIQUE",
		Primer_Apellido: "NAME",
	})

	tests := []struct {
		name     string
		params   SearchParams
		expected int
	}{
		{
			name: "find unique record",
			params: SearchParams{
				DNI:     "99999999",
				Workers: 4,
			},
			expected: 1,
		},
		{
			name: "find multiple records",
			params: SearchParams{
				PrimerNombre: "NAME",
				Workers:      4,
			},
			expected: 100,
		},
		{
			name: "no match",
			params: SearchParams{
				DNI:     "00000000",
				Workers: 4,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SearchCSVConcurrent(records, tt.params)
			if len(got) != tt.expected {
				t.Errorf("SearchCSVConcurrent() returned %d; want %d", len(got), tt.expected)
			}
		})
	}
}
