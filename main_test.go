package main

import (
	"os"
	"strings"
	"testing"
)

func TestParseRecord(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		expectedDNI string
		expectedErr bool
	}{
		{
			name:        "valid record",
			input:       []string{"V", "12345678", "Gomez", "Garcia", "Juan", "Maria", "001"},
			expectedDNI: "12345678",
			expectedErr: false,
		},
		{
			name:        "short record",
			input:       []string{"V", "12345678", "Gomez"},
			expectedDNI: "",
			expectedErr: true,
		},
		{
			name:        "empty record",
			input:       []string{},
			expectedDNI: "",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRecord(tt.input)
			if tt.expectedErr {
				if result.DNI != "" {
					t.Errorf("expected empty DNI for short record, got %s", result.DNI)
				}
			} else {
				if result.DNI != tt.expectedDNI {
					t.Errorf("DNI = %s, want %s", result.DNI, tt.expectedDNI)
				}
				if result.Primer_Nombre != "Juan" {
					t.Errorf("Primer_Nombre = %s, want Juan", result.Primer_Nombre)
				}
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"existing file", tmpPath, true},
		{"non-existing file", "/tmp/nonexistent-file-12345.txt", false},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileExists(tt.path)
			if result != tt.expected {
				t.Errorf("fileExists(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestValidateConfigInvalidMode(t *testing.T) {
	// Save original config
	origConfig := config

	// Restore after test
	defer func() { config = origConfig }()

	// Test invalid mode
	config = Config{Mode: SearchMode("invalid")}
	err := validateConfig()
	if err == nil {
		t.Error("expected error for invalid mode, got nil")
	} else if !strings.Contains(err.Error(), "invalid mode") {
		t.Errorf("error = %q, want to contain 'invalid mode'", err.Error())
	}
}

func TestValidateConfigNoCriteria(t *testing.T) {
	// Save original config
	origConfig := config

	// Restore after test
	defer func() { config = origConfig }()

	// Test no search criteria (with valid mode and logic)
	config = Config{Mode: ModeCSV, Logic: LogicAND}
	err := validateConfig()
	if err == nil {
		t.Error("expected error for no criteria, got nil")
	} else if !strings.Contains(err.Error(), "no search criteria") {
		t.Errorf("error = %q, want to contain 'no search criteria'", err.Error())
	}
}

func TestSearchConstants(t *testing.T) {
	// Verify constants are properly defined
	if FieldDNI != "dni" {
		t.Errorf("FieldDNI = %q, want 'dni'", FieldDNI)
	}
	if FieldPrimerNombre != "primer_nombre" {
		t.Errorf("FieldPrimerNombre = %q, want 'primer_nombre'", FieldPrimerNombre)
	}
	if DefaultCSVPath == "" {
		t.Error("DefaultCSVPath should not be empty")
	}
	if DefaultBatchSize <= 0 {
		t.Errorf("DefaultBatchSize = %d, want > 0", DefaultBatchSize)
	}
}
