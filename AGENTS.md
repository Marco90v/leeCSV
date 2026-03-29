# AGENTS.md - leeCSV Project Guidelines

## Project Overview

- **Project name:** leeCSV
- **Language:** Go 1.21+
- **Purpose:** Search and filter records from large CSV files (30M+ records) containing national ID data
- **Architecture:** Concurrent processing with goroutines, supports multiple search modes (CSV, Index, SQLite)

## Build, Lint, and Test Commands

### Build
```bash
go build -o leeCSV .
```

### Run
```bash
go run .
```

### Lint
```bash
golangci-lint run ./...
```

### Format
```bash
gofmt -w .
```

### Vet
```bash
go vet ./...
```

### Test
```bash
go test ./...           # Run all tests
go test -v ./...        # Verbose output
go test -race ./...    # With race detector
go test -run TestName  # Run single test
go test -bench=.       # Run benchmarks
```

### Dependencies
```bash
go mod tidy             # Clean up dependencies
go get -u package@latest  # Update package
```

---

## Code Style Guidelines

### Imports

Organize imports in three groups (standard library first, then external):

```go
import (
    // Standard library
    "context"
    "fmt"
    "io"
    "os"

    // External packages
    "github.com/pkg/errors"

    // Project internal packages (if applicable)
    "go/csv/internal/config"
)
```

### Formatting

- Use `gofmt` before committing (run automatically or configure editor)
- Use **tabs** for indentation (Go standard)
- Keep lines under 100 characters when practical
- Add blank lines between logical sections

### Types and Declarations

```go
// Good: Group related variables
var (
    maxWorkers = 4
    bufferSize = 1000
)

// Good: Use meaningful names
type Record struct {
    Nacionalidad     string
    DNI             string
    Primer_Nombre   string
}

// Bad: Single letter names (except in loops)
for i := 0; i < n; i++ {  // OK: loop variable
```

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Variables | camelCase | `filePath`, `recordCount` |
| Constants | PascalCase or camelCase | `MaxWorkers`, `defaultBuffer` |
| Functions | PascalCase (exported), camelCase (unexported) | `ReadFile`, `parseRecord` |
| Types | PascalCase | `Record`, `SearchResult` |
| Packages | lowercase, short | `csv`, `index`, `search` |

### Error Handling

**MUST DO:**
- Always handle errors explicitly (no naked returns)
- Use `fmt.Errorf` with `%w` for wrapping errors
- Return errors from functions, never use `panic` for expected errors
- Check errors immediately after calls

```go
// Good
func readFile(path string) (*csv.Reader, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("abrir archivo %s: %w", path, err)
    }
    return csv.NewReader(file), nil
}

// Bad
func readFile(path string) *csv.Reader {
    file, err := os.Open(path)
    if err != nil {
        panic(err)  // Never do this for expected errors
    }
    return csv.NewReader(file)
}
```

### Context Usage

Add `context.Context` to all blocking operations:

```go
func readRecords(ctx context.Context, file *os.File) (<-chan []string, error) {
    // ...
}
```

### Concurrency Patterns

When using goroutines:
- Always provide a way to cancel (use `context.Context`)
- Use `sync.WaitGroup` for tracking completion
- Close channels properly to avoid leaks
- Don't forget `defer wg.Done()` in goroutines

```go
func worker(ctx context.Context, jobs <-chan Job, results chan<- Result) {
    for {
        select {
        case <-ctx.Done():
            return
        case job, ok := <-jobs:
            if !ok {
                return
            }
            // process job
        }
    }
}
```

### Performance Considerations

- Avoid `reflect` in hot paths (use direct field access)
- Use buffered channels when appropriate
- Consider memory usage with large files (30M+ records)
- Profile with `pprof` if needed

### Documentation

- Document all exported functions, types, and packages
- Use comments starting with the name being documented

```go
// SearchByDNI searches for records matching the given DNI.
// Returns all matching records or an error if the search fails.
func SearchByDNI(dni string) ([]Record, error) {
```

---

## Project-Specific Guidelines

### CSV Format
- Separator: semicolon (`;`)
- Skip first line (header)
- Fields: Nacionalidad; Cedula; Primer_Apellido; Segundo_Apellido; Primer_Nombre; Segundo_Nombre; Cod_Centro

### Record Structure
```go
type Record struct {
    Nacionalidad     string
    DNI              string  // Cedula
    Primer_Apellido  string
    Segundo_Apellido string
    Primer_Nombre    string
    Segundo_Nombre   string
    Cod_Centro       string
}
```

### Search Modes
The application supports three search modes:
1. **CSV mode:** Direct CSV reading (slow for large files, loads all in memory)
2. **Index mode:** In-memory index for fast lookups (recommended for large files)
3. **SQLite mode:** Full-text search with SQLite FTS5 (best for complex queries)

#### Mode Selection
Use flags to select the search mode:
```bash
./leeCSV -mode=csv -csv=/path/to/file.csv -dni=12345678
./leeCSV -mode=index -index=/path/to/index.json -dni=12345678
./leeCSV -mode=sqlite -db=/path/to/database.db -dni=12345678
```

### Search Patterns
Support multiple matching patterns per field:
- **exact:** Exact match (e.g., DNI = "12345678")
- **contains:** Substring search (e.g., name contains "Juan")
- **startswith:** Prefix search (e.g., lastname starts with "Gar")
- **regex:** Regular expression (for advanced users)

```bash
# Examples
./leeCSV -dni=12345678                          # exact by default
./leeCSV -primerNombre:contains=Juan           # contains
./leeCSV -primerApellido:startswith=Gar       # starts with
./leeCSV -dni:regex=^1[0-9]{7}                 # regex pattern
```

### Search Logic Options
Combine multiple search criteria with:
- **AND:** All conditions must match (default)
- **OR:** At least one condition must match

```bash
./leeCSV -logic=AND -dni=12345678 -primerNombre=Juan
./leeCSV -logic=OR -primerNombre=Juan -primerNombre=Maria
```

### Index Mode (Recommended for 30M+ records)
The Index mode is designed for large files:
1. **First run:** Build index from CSV (creates .json index file)
2. **Subsequent runs:** Load pre-built index (instant startup)

```bash
# Build index (one-time operation)
./leeCSV -mode=index -build -csv=/path/to/large.csv -index=/path/to/index.json

# Use existing index
./leeCSV -mode=index -index=/path/to/index.json -dni=12345678
```

**Index storage:** The index file can be reused since the source CSV doesn't change (it's exported from a database).

### Memory Management
**IMPORTANT:** The main goal is to NOT load all 30M records into memory.
- Use streaming/chunked reading for CSV mode
- Build index incrementally with goroutines
- SQLite mode handles memory automatically via the database engine
- Target: Work with 8GB RAM, 2 cores / 4 threads

### Configuration
- Use flags for CLI arguments (see existing `flag` usage)
- Consider environment variables for file paths
- All paths should be configurable (no hardcoded paths)

---

## Testing Guidelines

Use table-driven tests:

```go
func TestSearchByDNI(t *testing.T) {
    tests := []struct {
        name     string
        dni      string
        wantLen  int
        wantErr  bool
    }{
        {"found", "12345678", 1, false},
        {"not found", "00000000", 0, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

Run tests with race detector:
```bash
go test -race ./...
```

---

## Common Patterns

### Flag Parsing
```go
func init() {
    flag.StringVar(&config.CSVPath, "csv", "", "Path to CSV file")
    flag.IntVar(&config.Workers, "workers", 4, "Number of workers")
}
```

### Graceful Shutdown
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
// ... setup workers
// On signal: cancel()
```

---

## Dependencies

- `github.com/gocarina/gocsv` - CSV parsing
- `github.com/mattn/go-sqlite3` - SQLite database

---

## References

- Go Style Guide: https://google.github.io/styleguide/go/
- Effective Go: https://go.dev/doc/effective_go
- golang-pro skill: See `.agents/skills/golang-pro/`
