# AGENTS.md - leeCSV Project Guidelines

## Project Overview

- **Project name:** leeCSV
- **Language:** Go 1.21+
- **Purpose:** Search and filter records from large CSV files (30M+ records) containing national ID data (Venezuelan cedulas)
- **Architecture:** CLI tool using Cobra framework, supports multiple search modes (CSV, Index, SQLite)
- **Module:** `go/csv`

## Project Structure

```
leeCSV/
├── main.go                 # Entry point
├── go.mod                  # Module definition
├── go.sum                  # Dependencies checksums
├── AGENTS.md               # This file
├── cmd/                    # CLI commands (Cobra)
│   ├── root.go            # Root command, config, global flags
│   ├── search.go          # CSV direct search command
│   ├── index.go          # Index build/search commands
│   ├── db.go             # SQLite build/search commands
│   └── types.go          # CLI-specific types (Config, SearchParams)
└── internal/               # Core business logic
    ├── types.go          # Record, SearchCondition, SearchPattern, SearchLogic
    ├── record.go         # CSV reading and search logic
    ├── index.go          # In-memory index for fast lookups
    └── sqlite.go         # SQLite manager with FTS5 support
```

**Note:** The project uses the standard library `encoding/csv` package, NOT `gocarina/gocsv`.

## Available Skills

The following skills are available for this project. Load them when working on specific tasks:

| Skill | When to Use |
|-------|-------------|
| **clean-code** | Refactoring, improving code quality, applying Clean Code principles |
| **cli-developer** | CLI design, adding subcommands, flags, completions, terminal UI |
| **golang-cli** | Go CLI development with Cobra/Viper, exit codes, signal handling |
| **golang-pro** | Concurrency patterns, goroutines, channels, performance optimization |
| **golang-testing** | Writing tests, benchmarks, fuzzing, TDD workflow |
| **refactor** | Extracting functions, renaming, breaking down large functions, design patterns |
| **SQLite Database Expert** | SQLite with FTS5, migrations, security patterns, performance |
| **context7-mcp** | Looking up library/framework documentation |
| **csv-data-wrangler** | CSV processing, data cleaning, DuckDB for large files |
| **csv-excel-merger** | Merging CSV/Excel files, column matching |

### Loading a Skill

When a task matches a skill description, load it using:

```
Load the [skill-name] skill
```

This will inject detailed instructions into the context.

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

### Implementation Best Practices (REQUIRED)

**Always use streaming when processing large files:**
- NEVER load all records into memory with `ReadFile()` for large CSVs
- Use streaming readers (`csv.NewReader`) that process records incrementally
- Build indexes while reading the file, not after loading everything

**Always use goroutines for parallel processing:**
- Use `go func()` to run tasks in parallel when appropriate
- Use channels to stream data between goroutines
- Use `sync.WaitGroup` to coordinate goroutine completion
- Use `context.Context` for cancellation and graceful shutdown

**Always use case-insensitive search:**
- Use `strings.EqualFold()` for string comparisons
- This ensures "JUAN", "Juan", "juan" all match when searching

Example (CORRECT):
```go
// Case-insensitive comparison
if strings.EqualFold(record.DNI, searchDNI) {
    // match
}

// Streaming search (CORRECT)
func SearchCSVStreaming(ctx context.Context, csvPath string, params SearchParams) ([]Record, error) {
    file, err := os.Open(csvPath)
    defer file.Close()
    
    reader := csv.NewReader(file)
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        // process record one at a time
    }
}
```

Example (INCORRECT - DO NOT USE):
```go
// Case-sensitive comparison (WRONG)
if record.DNI == searchDNI {
    // will miss "12345678" when searching for "01234567"
}

// Loading all into memory (WRONG for large files)
records, _ := ReadFileWithContext(ctx, csvPath)  // 30M records = crash
```

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
Use subcommands to select the search mode (Cobra framework):
```bash
# Search using CSV mode (loads all in memory)
leeCSV search --csv=/path/to/file.csv --dni=12345678

# Build and search using index (recommended for 30M+ records)
leeCSV index build --csv=/path/to/large.csv --index=/path/to/index.json
leeCSV index search --index=/path/to/index.json --dni=12345678

# Build and search using SQLite (best for complex queries)
leeCSV db build --csv=/path/to/large.csv --db=/path/to/database.db
leeCSV db search --db=/path/to/database.db --dni=12345678
```

### Search Patterns
Currently supported matching patterns per field:
- **exact:** Exact match (e.g., DNI = "12345678") - default
- **contains:** Substring search (e.g., name contains "Juan") - SQLite only
- **startswith:** Prefix search (e.g., lastname starts with "Gar") - SQLite only
- **regex:** Not yet implemented

**Note:** Pattern selection via CLI flags is not yet available. Currently all searches use exact match by default.

### Index Mode (Recommended for 30M+ records)
The Index mode is designed for large files:
1. **First run:** Build index from CSV (creates .json index file)
2. **Subsequent runs:** Load pre-built index (instant startup)

```bash
# Build index (one-time operation)
leeCSV index build --csv=/path/to/large.csv --index=/path/to/index.json

# Use existing index
leeCSV index search --index=/path/to/index.json --dni=12345678
```

**Index storage:** The index file can be reused since the source CSV doesn't change (it's exported from a database).

### Search Logic Options
Combine multiple search criteria with:
- **AND:** All conditions must match (default)
- **OR:** At least one condition must match

```bash
leeCSV search --logic=AND --dni=12345678 --primer-nombre=Juan
leeCSV index search --logic=OR --primer-nombre=Juan --primer-nombre=Maria
```

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

- `github.com/spf13/cobra` - CLI framework (command structure, flags, subcommands)
- `github.com/mattn/go-sqlite3` - SQLite database driver

**Note:** The project uses Go's standard library `encoding/csv` package, NOT `gocarina/gocsv`.

---

## References

- Go Style Guide: https://google.github.io/styleguide/go/
- Effective Go: https://go.dev/doc/effective_go
- golang-pro skill: See `.agents/skills/golang-pro/`
