# AGENTS.md - leeCSV Project Guidelines

## Project Overview
- **Name:** leeCSV - CLI tool for searching large CSV files (30M+ records)
- **Language:** Go 1.21+
- **Architecture:** Cobra CLI with 3 search modes (CSV, Index, SQLite)
- **Key:** Handles Venezuelan national ID data efficiently

## Project Structure
```
leeCSV/
├── main.go              # Entry point
├── cmd/                 # CLI commands
│   ├── root.go         # Root + config
│   ├── search.go       # CSV mode
│   ├── index.go        # Index mode
│   └── db.go           # SQLite mode
└── internal/            # Core logic
    ├── types.go        # Record, SearchCondition
    ├── record.go       # CSV read + streaming search
    ├── index.go        # In-memory index
    └── sqlite.go       # SQLite + FTS5
```

## Commands
```bash
# Build & Run
go build -o leeCSV .           # Build binary
go build -tags fts5 -o leeCSV . # Build with FTS5 full-text search
go run .                       # Run
go run -tags fts5 .             # Run with FTS5

# Development
go test ./...           # Run all tests
go test -race ./...    # Test with race detector
gofmt -w .              # Format code
golangci-lint run ./... # Lint
go vet ./...            # Vet

# Dependencies
go mod tidy             # Clean dependencies
```

## Development Workflow
```bash
# Add new command
# 1. Create cmd/newcmd.go with cobra.Command
# 2. Import in root.go: rootCmd.AddCommand(newCmd)

# Add new search mode
# 1. Add to internal/types.go (SearchPattern, SearchLogic)
# 2. Implement in record.go or index.go or sqlite.go
# 3. Wire in cmd/search.go or cmd/index.go
```

## CSV Format
- **Separator:** `;`
- **Header:** Skip first line
- **Fields:** Nacionalidad; Cedula; Primer_Apellido; Segundo_Apellido; Primer_Nombre; Segundo_Nombre; Cod_Centro

## Record Structure
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

## Search Modes

### CSV Mode (slow, all in memory)
```bash
leeCSV search --csv=data.csv --dni=12345678
```

### Index Mode (recommended for 30M+)
```bash
# First: build index
leeCSV index build --csv=data.csv --index=index.json

# Then: search (instant)
leeCSV index search --index=index.json --dni=12345678
```

### SQLite Mode (best for complex queries)
```bash
# First: build database (use -tags fts5 for full-text search)
go run -tags fts5 . db build --csv=data.csv --db=data.db

# Then: search
leeCSV db search --db=data.db --dni=12345678

# Or: LIKE queries (contains)
leeCSV db search --db=data.db --primer-nombre=Juan --pattern=contains
```

## Skills
Load when needed:
```
Load [skill-name] skill
```
| Skill | Use Case |
|-------|----------|
| **golang-cli** | CLI, Cobra, flags |
| **golang-pro** | Concurrency, performance |
| **golang-testing** | Tests, benchmarks |
| **refactor** | Code improvements |
| **sqlite-database-expert** | SQLite, FTS5 |

---

## Essential Rules (MANDATORY)

### 1. Streaming for Large Files
**Never load all records into memory** for files with 30M+ records.
- Use `csv.NewReader` (streaming, line-by-line)
- Build index incrementally while reading
- Reference: `SearchCSVStreaming()` in internal/record.go

```go
// CORRECT: Stream processing
reader := csv.NewReader(file)
for {
    record, err := reader.Read()
    if err == io.EOF { break }
    // process one at a time
}

// WRONG: Load all into memory
records, _ := ReadFile(csvPath)  // crashes on 30M records
```

### 2. Case-Insensitive Search
Always use `strings.EqualFold()` for text comparisons.
```go
// CORRECT
if strings.EqualFold(record.DNI, searchDNI) { }

// WRONG - misses "12345678" when searching "012345678"
if record.DNI == searchDNI { }
```

### 3. Context for Blocking Operations
Add `context.Context` to all I/O operations for cancellation support.
```go
func ReadCSV(ctx context.Context, path string) (<-chan Record, error) {
    // ...
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    // ...
    }
}
```

---

## Debugging & Troubleshooting

### Out of memory
- Use Index or SQLite mode instead of CSV mode
- Ensure streaming (no ReadFile for large files)

### Slow indexing
- Normal: ~6-7M records/minute on single thread
- Use `--workers` flag for parallelism

### Search returns nothing
- Verify case: use `--dni=12345678` not `--dni=012345678`
- Check field names: `--primer-nombre`, not `--primernombre`

### SQLite FTS not working
- Use `-tags fts5` when building: `go build -tags fts5 -o leeCSV .`
- Falls back to LIKE queries automatically if FTS5 not available

---

## Dependencies
- `github.com/spf13/cobra` - CLI framework
- `github.com/mattn/go-sqlite3` - SQLite driver
- Uses Go standard library `encoding/csv` (NOT gocarina/gocsv)

## References
- Go Style Guide: https://google.github.io/styleguide/go/
- Effective Go: https://go.dev/doc/effective_go
