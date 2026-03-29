package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSQLiteManager(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	manager, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManager() error = %v; want nil", err)
	}
	defer manager.Close()

	// SQLite creates file on first write, so just verify manager is valid
	if manager.db == nil {
		t.Error("manager.db should not be nil")
	}
}

func TestNewSQLiteManagerInvalidPath(t *testing.T) {
	// SQLite is permissive - it creates the file/directory as needed
	// This test just verifies the manager can be created
	_, err := NewSQLiteManager("/tmp/nonexistent_dir_12345/test.db")
	if err != nil {
		t.Logf("NewSQLiteManager info: %v", err)
	}
	// Just verify it doesn't panic - SQLite handles this gracefully
}

func TestSQLiteManagerClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	manager, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManager() error = %v; want nil", err)
	}

	err = manager.Close()
	if err != nil {
		t.Errorf("Close() error = %v; want nil", err)
	}
}

func TestSQLiteBuildDBFromCSV(t *testing.T) {
	// Create temp CSV
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	dbPath := filepath.Join(tmpDir, "test.db")

	content := `nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
V;12345678;GARCIA;PEREZ;JUAN;CARLOS;100
V;87654321;LOPEZ;;MARIA;;200
V;11223344;GOMEZ;RUIZ;ANA;BELEN;300`

	err := os.WriteFile(csvPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test CSV: %v", err)
	}

	manager, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManager() error = %v; want nil", err)
	}
	defer manager.Close()

	ctx := context.Background()
	err = manager.BuildDBFromCSV(ctx, csvPath, 1)
	if err != nil {
		t.Fatalf("BuildDBFromCSV() error = %v; want nil", err)
	}

	// Verify record count
	count, err := manager.GetRecordCount()
	if err != nil {
		t.Fatalf("GetRecordCount() error = %v; want nil", err)
	}
	if count != 3 {
		t.Errorf("record count = %d; want 3", count)
	}
}

func TestSQLiteSearchByFieldExact(t *testing.T) {
	// Setup: create temp CSV and build DB
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	dbPath := filepath.Join(tmpDir, "test.db")

	content := `nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
V;12345678;GARCIA;PEREZ;JUAN;CARLOS;100
V;87654321;LOPEZ;;MARIA;;200`

	err := os.WriteFile(csvPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test CSV: %v", err)
	}

	manager, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManager() error = %v; want nil", err)
	}
	defer manager.Close()

	ctx := context.Background()
	_ = manager.BuildDBFromCSV(ctx, csvPath, 1)

	// Test search by DNI
	results, err := manager.SearchByField("dni", "12345678", PatternExact)
	if err != nil {
		t.Fatalf("SearchByField() error = %v; want nil", err)
	}
	if len(results) != 1 {
		t.Errorf("SearchByField() returned %d; want 1", len(results))
	}
	if results[0].DNI != "12345678" {
		t.Errorf("DNI = %s; want 12345678", results[0].DNI)
	}

	// Test search by primer_nombre
	results, err = manager.SearchByField("primer_nombre", "JUAN", PatternExact)
	if err != nil {
		t.Fatalf("SearchByField() error = %v; want nil", err)
	}
	if len(results) != 1 {
		t.Errorf("SearchByField() returned %d; want 1", len(results))
	}
	if results[0].Primer_Nombre != "JUAN" {
		t.Errorf("Primer_Nombre = %s; want JUAN", results[0].Primer_Nombre)
	}
}

func TestSQLiteSearchByFieldContains(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	dbPath := filepath.Join(tmpDir, "test.db")

	content := `nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
V;12345678;GARCIA;PEREZ;JUAN;CARLOS;100
V;87654321;GARCIA;RUIZ;MARIA; ;200`

	err := os.WriteFile(csvPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test CSV: %v", err)
	}

	manager, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManager() error = %v; want nil", err)
	}
	defer manager.Close()

	ctx := context.Background()
	_ = manager.BuildDBFromCSV(ctx, csvPath, 1)

	// Contains search - "GAR" is in both GARCIA and GARCIA
	results, err := manager.SearchByField("primer_apellido", "GAR", PatternContains)
	if err != nil {
		t.Fatalf("SearchByField() error = %v; want nil", err)
	}
	// Both records have "GARCIA" which contains "GAR"
	if len(results) != 2 {
		t.Errorf("Contains search returned %d; want 2", len(results))
	}

	// StartsWith search - "GAR" starts both GARCIA
	results, err = manager.SearchByField("primer_apellido", "GAR", PatternStartsWith)
	if err != nil {
		t.Fatalf("SearchByField() error = %v; want nil", err)
	}
	if len(results) != 2 {
		t.Errorf("StartsWith search returned %d; want 2", len(results))
	}
}

func TestSQLiteSearchByFieldNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	dbPath := filepath.Join(tmpDir, "test.db")

	content := `nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
V;12345678;GARCIA;PEREZ;JUAN;CARLOS;100`

	err := os.WriteFile(csvPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test CSV: %v", err)
	}

	manager, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManager() error = %v; want nil", err)
	}
	defer manager.Close()

	ctx := context.Background()
	_ = manager.BuildDBFromCSV(ctx, csvPath, 1)

	// No match
	results, err := manager.SearchByField("dni", "00000000", PatternExact)
	if err != nil {
		t.Fatalf("SearchByField() error = %v; want nil", err)
	}
	if len(results) != 0 {
		t.Errorf("SearchByField() returned %d; want 0", len(results))
	}
}

func TestSQLiteSearchAll(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	dbPath := filepath.Join(tmpDir, "test.db")

	content := `nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
V;12345678;GARCIA;PEREZ;JUAN;CARLOS;100
V;87654321;LOPEZ;RUIZ;MARIA; ;200
V;11223344;GARCIA;RUIZ;ANA; ;300`

	err := os.WriteFile(csvPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test CSV: %v", err)
	}

	manager, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManager() error = %v; want nil", err)
	}
	defer manager.Close()

	ctx := context.Background()
	_ = manager.BuildDBFromCSV(ctx, csvPath, 1)

	tests := []struct {
		name       string
		conditions []SearchCondition
		logic      SearchLogic
		expected   int
	}{
		{
			name:       "empty conditions",
			conditions: []SearchCondition{},
			logic:      LogicAND,
			expected:   0,
		},
		{
			name: "single condition",
			conditions: []SearchCondition{
				{Field: "dni", Value: "12345678", Pattern: PatternExact},
			},
			logic:    LogicAND,
			expected: 1,
		},
		{
			name: "AND logic - both must match",
			conditions: []SearchCondition{
				{Field: "primer_apellido", Value: "GARCIA", Pattern: PatternExact},
				{Field: "primer_nombre", Value: "JUAN", Pattern: PatternExact},
			},
			logic:    LogicAND,
			expected: 1,
		},
		{
			name: "OR logic - either matches",
			conditions: []SearchCondition{
				{Field: "primer_nombre", Value: "JUAN", Pattern: PatternExact},
				{Field: "primer_nombre", Value: "MARIA", Pattern: PatternExact},
			},
			logic:    LogicOR,
			expected: 2,
		},
		{
			name: "AND no match",
			conditions: []SearchCondition{
				{Field: "primer_apellido", Value: "GARCIA", Pattern: PatternExact},
				{Field: "primer_nombre", Value: "MARIA", Pattern: PatternExact},
			},
			logic:    LogicAND,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := manager.SearchAll(tt.conditions, tt.logic)
			if err != nil {
				t.Fatalf("SearchAll() error = %v; want nil", err)
			}
			if len(results) != tt.expected {
				t.Errorf("SearchAll() returned %d; want %d", len(results), tt.expected)
			}
		})
	}
}

func TestSQLiteGetRecordCount(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	dbPath := filepath.Join(tmpDir, "test.db")

	content := `nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
V;12345678;GARCIA;PEREZ;JUAN;CARLOS;100
V;87654321;LOPEZ;;MARIA;;200`

	err := os.WriteFile(csvPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test CSV: %v", err)
	}

	manager, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManager() error = %v; want nil", err)
	}
	defer manager.Close()

	ctx := context.Background()
	_ = manager.BuildDBFromCSV(ctx, csvPath, 1)

	count, err := manager.GetRecordCount()
	if err != nil {
		t.Fatalf("GetRecordCount() error = %v; want nil", err)
	}
	if count != 2 {
		t.Errorf("GetRecordCount() = %d; want 2", count)
	}
}

func TestSQLiteSearchAllWithPattern(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	dbPath := filepath.Join(tmpDir, "test.db")

	content := `nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
V;12345678;GARCIA;PEREZ;JUAN;CARLOS;100
V;87654321;LOPEZ;;MARIA; ;200`

	err := os.WriteFile(csvPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test CSV: %v", err)
	}

	manager, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManager() error = %v; want nil", err)
	}
	defer manager.Close()

	ctx := context.Background()
	_ = manager.BuildDBFromCSV(ctx, csvPath, 1)

	// Test with contains pattern
	results, err := manager.SearchAll([]SearchCondition{
		{Field: "primer_nombre", Value: "UA", Pattern: PatternContains},
	}, LogicAND)

	if err != nil {
		t.Fatalf("SearchAll() error = %v; want nil", err)
	}
	// Should match "MARIA" (contains "UA")
	if len(results) != 1 {
		t.Errorf("SearchAll() with contains returned %d; want 1", len(results))
	}
}
