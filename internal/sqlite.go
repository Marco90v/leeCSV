package internal

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Constantes de configuración para SQLite.
//
// Estos valores están optimizados para importación masiva de datos:
//   - DefaultBatchSize: tamaño de lote por defecto
//   - WorkersBatchFactor: multiplicador según workers
//   - BusyTimeoutMs: tiempo de espera si la DB está ocupada
const (
	// DefaultBatchSize es el tamaño de lote por defecto para inserción.
	// Cada lote se inserta en una transacción.
	DefaultBatchSize = 1000

	// WorkersBatchFactor multiplica el tamaño de lote según workers.
	// workers * WorkersBatchFactor * 10 = batch size final
	WorkersBatchFactor = 100

	// BusyTimeoutMs es el tiempo máximo de espera (en ms) cuando
	// SQLite está ocupado con otra operación.
	BusyTimeoutMs = 30000
)

// SQLiteManager maneja todas las operaciones con la base de datos SQLite.
//
// Proporciona una capa de abstracción sobre sql.DB con:
//   - Gestión de conexión
//   - Creación de tablas
//   - Importación de CSV
//   - Búsquedas (con y sin FTS5)
//   - Manejo de índices
//
// El manager mantiene estado de:
//   - Ruta a la base de datos
//   - Conexión abierta
//   - Si FTS5 está disponible
type SQLiteManager struct {
	// dbPath guarda la ruta al archivo de base de datos
	dbPath string

	// db es la conexión a SQLite
	// sql.DB es seguro para uso concurrente (pool de conexiones)
	db *sql.DB

	// ftsAvailable indica si FTS5 está habilitado.
	// FTS5 puede no estar disponible en algunas compilaciones de SQLite.
	ftsAvailable bool
}

// NewSQLiteManager crea un nuevo manager y abre la base de datos.
//
// Esta función:
//  1. Abre la conexión a SQLite
//  2. Configura WAL mode para mejor rendimiento concurrente
//  3. Configura busy timeout para evitar errores de "database locked"
//  4. Desactiva synchronous para inserción más rápida
//  5. Verifica si FTS5 está disponible
//
// Parámetros:
//   - dbPath: ruta al archivo de base de datos SQLite
//
// Retorna:
//   - *SQLiteManager: manager configurado
//   - error: error si falla la conexión
//
// Ejemplo:
//
//	db, err := NewSQLiteManager("datos.db")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
func NewSQLiteManager(dbPath string) (*SQLiteManager, error) {
	// Abrir conexión a SQLite
	// "sqlite3" es el nombre del driver registrado por go-sqlite3
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir base de datos: %w", err)
	}

	// Habilitar WAL (Write-Ahead Logging) mode
	// WAL permite lecturas y escrituras concurrentes
	// Mucho mejor rendimiento que el modo default (rollback)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("error al habilitar WAL mode: %w", err)
	}

	// Configurar busy timeout
	// Si la base de datos está bloqueada, espera hasta 30 segundos
	// antes de retornar error "database is locked"
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", BusyTimeoutMs)); err != nil {
		return nil, fmt.Errorf("error al configurar busy timeout: %w", err)
	}

	// Desactivar synchronous para inserción más rápida
	// ADVERTENCIA: puede perder datos si hay crash del sistema
	// pero es aceptable para importación inicial
	if _, err := db.Exec("PRAGMA synchronous=OFF"); err != nil {
		return nil, fmt.Errorf("error al configurar synchronous: %w", err)
	}

	sm := &SQLiteManager{
		dbPath: dbPath,
		db:     db,
	}

	// Verificar si FTS5 está disponible
	// Si se construyó SQLite sin FTS5, no estará disponible
	sm.ftsAvailable = sm.checkFTS5Available()

	return sm, nil
}

// checkFTS5Available verifica si FTS5 está disponible en SQLite.
//
// FTS5 (Full-Text Search 5) es una extensión de SQLite para
// búsquedas de texto completo. No está disponible en todas las
// compilaciones.
//
// Retorna:
//   - true: si la tabla FTS5 existe
//   - false: si no está disponible
func (sm *SQLiteManager) checkFTS5Available() bool {
	var count int
	// Consultar sqlite_master (metadatos de SQLite)
	err := sm.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='records_fts'",
	).Scan(&count)
	// Disponible si la tabla existe y no hay error
	return err == nil && count > 0
}

// IsFTS5Available retorna si FTS5 está disponible.
//
// Útil para mostrar al usuario qué modo de búsqueda está disponible.
//
// Retorna:
//   - bool: true si FTS5 está habilitado
func (sm *SQLiteManager) IsFTS5Available() bool {
	return sm.ftsAvailable
}

// Close cierra la conexión a la base de datos.
//
// Siempre debe llamarse cuando Termina el uso del manager.
// Usar defer inmediatamente después de NewSQLiteManager:
//
//	db, _ := NewSQLiteManager("datos.db")
//	defer db.Close()
//
// Retorna:
//   - error: error al cerrar
func (sm *SQLiteManager) Close() error {
	return sm.db.Close()
}

// BuildDBFromCSV importa datos de un archivo CSV a SQLite.
//
// Este es el proceso principal de construcción de la base de datos:
//  1. Crea las tablas necesarias
//  2. Lee el CSV línea por línea (streaming)
//  3. Inserta en lotes (transactions) para rendimiento
//  4. Construye el índice FTS5 (si está disponible)
//  5. Ejecuta ANALYZE para optimizar queries
//
// El proceso es resiliente:
//   - Si FTS5 falla, usa LIKE como fallback
//   - Registros malformed se saltan
//   - Soporta cancelación via context
//
// Parámetros:
//   - ctx: contexto para cancelación
//   - csvPath: ruta al archivo CSV
//   - workers: número de workers (afecta tamaño de lote)
//
// Retorna:
//   - error: error si falla la importación
func (sm *SQLiteManager) BuildDBFromCSV(ctx context.Context, csvPath string, workers int) error {
	// Paso 1: Crear tablas
	if err := sm.createTables(); err != nil {
		return fmt.Errorf("error al crear tablas: %w", err)
	}

	// Abrir archivo CSV
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("error al abrir CSV: %w", err)
	}
	defer file.Close()

	// Crear lector CSV
	reader := csv.NewReader(file)
	reader.Comma = ';' // Separador venezolano

	// Saltar header
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("error al leer header: %w", err)
	}

	// Preparar statement INSERT
	// Usar prepared statement es más rápido que INSERT directo
	stmt, err := sm.db.Prepare(`
		INSERT INTO records (nacionalidad, dni, primer_apellido, segundo_apellido, primer_nombre, segundo_nombre, cod_centro)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error al preparar statement: %w", err)
	}
	defer stmt.Close()

	// Calcular tamaño de lote
	// Más workers = mayor lote = más eficiente
	batchSize := DefaultBatchSize * 10 // 10,000 por defecto
	if workers > 0 {
		batchSize = workers * WorkersBatchFactor * 10
	}

	// Crear batch (lote) vacío
	batch := make([][]string, 0, batchSize)
	recordCount := 0

	// Bucle de lectura
	for {
		// Verificar cancelación
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Leer siguiente línea
		record, err := reader.Read()

		// Fin del archivo
		if err == io.EOF {
			// Insertar lote restante si existe
			if len(batch) > 0 {
				if err := sm.flushBatch(stmt, batch); err != nil {
					return err
				}
			}
			break
		}

		// Error de lectura
		if err != nil {
			return fmt.Errorf("error al leer CSV: %w", err)
		}

		// Skip registros cortos
		if len(record) < 7 {
			continue
		}

		// Agregar al batch
		batch = append(batch, record)
		recordCount++

		// Si batch está lleno, insertar y reiniciar
		if len(batch) >= batchSize {
			if err := sm.flushBatch(stmt, batch); err != nil {
				return err
			}
			batch = batch[:0] // Reiniciar batch (mantener capacidad)

			// Log de progreso cada 100k registros
			if recordCount%100000 == 0 {
				fmt.Printf("Importados %d registros...\n", recordCount)
			}
		}
	}

	// Construir índice FTS5 (si está disponible)
	if err := sm.buildFTSIndex(); err != nil {
		fmt.Printf("Advertencia: error al construir índice FTS5: %v\n", err)
	}

	// Ejecutar ANALYZE para que SQLite optimice queries
	// ANALYZE recopila estadísticas de las tablas
	if _, err := sm.db.Exec("ANALYZE"); err != nil {
		fmt.Printf("Advertencia: error en ANALYZE: %v\n", err)
	}

	fmt.Printf("Importación completa: %d registros\n", recordCount)
	return nil
}

// flushBatch inserta un lote de registros en una transacción.
//
// Una transacción agrupa múltiples INSERTs en una sola operación,
// lo cual es MUCH más rápido que INSERTs individuales.
//
// Pasos:
//  1. Begin transaction
//  2. Ejecutar INSERT para cada registro
//  3. Commit si todo OK, Rollback si hay error
//
// Parámetros:
//   - stmt: prepared statement preparado
//   - batch: slice de registros a insertar
//
// Retorna:
//   - error: error si falla
func (sm *SQLiteManager) flushBatch(stmt *sql.Stmt, batch [][]string) error {
	// Iniciar transacción
	tx, err := sm.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}

	// Insertar cada registro del batch
	for _, record := range batch {
		if _, err := tx.Stmt(stmt).Exec(
			record[0], record[1], record[2], record[3],
			record[4], record[5], record[6],
		); err != nil {
			// Rollback si hay error
			_ = tx.Rollback()
			return fmt.Errorf("error al insertar registro: %w", err)
		}
	}

	// Commit de la transacción
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al hacer commit: %w", err)
	}
	return nil
}

// createTables crea las tablas necesarias en SQLite.
//
// Crea:
//  1. Tabla 'records' con los 7 campos del CSV
//  2. Índice en DNI para búsquedas rápidas
//  3. Índices en campos de nombres
//  4. Tabla FTS5 (si es posible)
//
// La tabla 'records' usa:
//   - id INTEGER PRIMARY KEY: auto-increment
//   - dni TEXT UNIQUE: no permite duplicados
func (sm *SQLiteManager) createTables() error {
	// Crear tabla principal
	_, err := sm.db.Exec(`
		CREATE TABLE IF NOT EXISTS records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nacionalidad TEXT,
			dni TEXT UNIQUE,
			primer_apellido TEXT,
			segundo_apellido TEXT,
			primer_nombre TEXT,
			segundo_nombre TEXT,
			cod_centro TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("error al crear tabla records: %w", err)
	}

	// Crear índice en DNI (B-tree, O(log n) lookup)
	_, err = sm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_dni ON records(dni)`)
	if err != nil {
		return fmt.Errorf("error al crear índice DNI: %w", err)
	}

	// Índices en campos de nombres
	_, _ = sm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_primer_nombre ON records(primer_nombre)`)
	_, _ = sm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_primer_apellido ON records(primer_apellido)`)

	// Intentar crear tabla FTS5
	// Si falla (no disponible), se usa LIKE como fallback
	if err := sm.createFTSTable(); err != nil {
		fmt.Printf("Nota: FTS5 no disponible - búsquedas usarán LIKE\n")
	}

	return nil
}

// createFTSTable intenta crear la tabla virtual FTS5 y sus triggers.
//
// FTS5 (Full-Text Search) permite búsquedas rápidas de texto.
// La tabla virtual no almacena datos, es un índice especial.
//
// También crea triggers para mantener el índice sincronizado:
//   - AFTER INSERT: agregar a FTS5
//   - AFTER DELETE: eliminar de FTS5
//   - AFTER UPDATE: actualizar en FTS5
//
// Si FTS5 no está disponible, retorna error (no es fatal).
func (sm *SQLiteManager) createFTSTable() error {
	// Crear tabla virtual FTS5
	// content='records' la vincula a la tabla principal
	_, err := sm.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS records_fts USING fts5(
			dni,
			primer_nombre,
			segundo_nombre,
			primer_apellido,
			segundo_apellido,
			content='records',
			content_rowid='id'
		)
	`)
	if err != nil {
		return fmt.Errorf("FTS5 no disponible: %w", err)
	}

	// Trigger para INSERT
	// Cuando se inserta en 'records', también insertar en FTS5
	_, err = sm.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS records_ai AFTER INSERT ON records BEGIN
			INSERT INTO records_fts(rowid, dni, primer_nombre, segundo_nombre, primer_apellido, segundo_apellido)
			VALUES (new.id, new.dni, new.primer_nombre, new.segundo_nombre, new.primer_apellido, new.segundo_apellido);
		END
	`)
	if err != nil {
		return fmt.Errorf("error al crear trigger insert: %w", err)
	}

	// Trigger para DELETE
	// Cuando se elimina de 'records', también de FTS5
	_, err = sm.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS records_ad AFTER DELETE ON records BEGIN
			INSERT INTO records_fts(records_fts, rowid, dni, primer_nombre, segundo_nombre, primer_apellido, segundo_apellido)
			VALUES('delete', old.id, old.dni, old.primer_nombre, old.segundo_nombre, old.primer_apellido, old.segundo_apellido);
		END
	`)
	if err != nil {
		return fmt.Errorf("error al crear trigger delete: %w", err)
	}

	// Trigger para UPDATE
	// Eliminar entrada vieja y agregar nueva
	_, err = sm.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS records_au AFTER UPDATE ON records BEGIN
			INSERT INTO records_fts(records_fts, rowid, dni, primer_nombre, segundo_nombre, primer_apellido, segundo_apellido)
			VALUES('delete', old.id, old.dni, old.primer_nombre, old.segundo_nombre, old.primer_apellido, old.segundo_apellido);
			INSERT INTO records_fts(rowid, dni, primer_nombre, segundo_nombre, primer_apellido, segundo_apellido)
			VALUES (new.id, new.dni, new.primer_nombre, new.segundo_nombre, new.primer_apellido, new.segundo_apellido);
		END
	`)
	if err != nil {
		return fmt.Errorf("error al crear trigger update: %w", err)
	}

	return nil
}

// buildFTSIndex construye el índice FTS5.
//
// Esto es necesario después de una importación masiva,
// porque los triggers no se ejecutaron para cada insert.
//
// Ejecuta:
//  1. 'rebuild': recrea el índice desde cero
//  2. 'optimize': optimiza el índice
func (sm *SQLiteManager) buildFTSIndex() error {
	// Verificar si la tabla FTS5 existe
	var count int
	err := sm.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='records_fts'",
	).Scan(&count)
	if err != nil || count == 0 {
		return fmt.Errorf("tabla FTS5 no disponible")
	}

	// Rebuild el índice
	_, err = sm.db.Exec(`INSERT INTO records_fts(records_fts) VALUES('rebuild')`)
	if err != nil {
		return fmt.Errorf("error al rebuild FTS: %w", err)
	}

	// Optimizar el índice
	_, err = sm.db.Exec(`INSERT INTO records_fts(records_fts) VALUES('optimize')`)
	return err
}

// SearchByField realiza una búsqueda por un campo específico.
//
// Esta es la función principal de búsqueda. Soporta:
//   - Búsqueda exacta (=)
//   - Búsqueda por contenido (LIKE %...%)
//   - Búsqueda por prefijo (LIKE ...%)
//   - FTS5 si está disponible
//
// Elige automáticamente el método:
//   - Si FTS5 disponible y pattern es contains/startswith: usa FTS5
//   - Sino: usa LIKE
//
// Parámetros:
//   - field: nombre del campo a buscar
//   - value: valor a buscar
//   - pattern: tipo de búsqueda
//
// Retorna:
//   - []Record: registros encontrados
//   - error: error si falla
func (sm *SQLiteManager) SearchByField(field, value string, pattern SearchPattern) ([]Record, error) {
	// Usar FTS5 si está disponible y el patrón lo permite
	if sm.ftsAvailable && (pattern == PatternContains || pattern == PatternStartsWith) {
		return sm.searchWithFTS(field, value, pattern)
	}

	// Fallback: usar LIKE queries
	var query string
	var args []interface{}

	switch pattern {
	case PatternExact:
		// WHERE field = value
		query = fmt.Sprintf(`SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			r.primer_nombre, r.segundo_nombre, r.cod_centro
			FROM records r WHERE r.%s = ?`, field)
		args = []interface{}{value}

	case PatternContains:
		// WHERE field LIKE '%value%'
		query = fmt.Sprintf(`SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			r.primer_nombre, r.segundo_nombre, r.cod_centro
			FROM records r WHERE r.%s LIKE ?`, field)
		args = []interface{}{"%" + value + "%"}

	case PatternStartsWith:
		// WHERE field LIKE 'value%'
		query = fmt.Sprintf(`SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			r.primer_nombre, r.segundo_nombre, r.cod_centro
			FROM records r WHERE r.%s LIKE ?`, field)
		args = []interface{}{value + "%"}

	default:
		// Pattern desconocido, usar exacto
		return sm.SearchByField(field, value, PatternExact)
	}

	// Ejecutar query
	rows, err := sm.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error en query: %w", err)
	}
	defer rows.Close()

	// Convertir rows a structs
	return sm.scanRecords(rows)
}

// searchWithFTS realiza búsqueda usando FTS5.
//
// FTS5 es más rápido que LIKE para búsquedas de texto.
// Usa sintaxis especial: MATCH "campo:valor*"
//
// Para contains y startswith usa prefix search (valor*).
func (sm *SQLiteManager) searchWithFTS(field, value string, pattern SearchPattern) ([]Record, error) {
	// Validar que el campo esté en FTS5
	allowedFields := map[string]bool{
		"dni":              true,
		"primer_nombre":    true,
		"segundo_nombre":   true,
		"primer_apellido":  true,
		"segundo_apellido": true,
	}
	if !allowedFields[field] {
		// Campo no en FTS5, usar LIKE
		return sm.searchWithLike(field, value, pattern)
	}

	// Construir expresión de búsqueda FTS5
	// FTS5 MATCH: "campo:valor*" para prefix search
	// Convertir a minúsculas para case-insensitive
	var matchExpr string
	lowerValue := strings.ToLower(value)
	switch pattern {
	case PatternContains, PatternStartsWith:
		// Prefix search con *
		matchExpr = fmt.Sprintf(`%s:%s*`, field, escapeFTS5Value(lowerValue))
	default:
		// Búsqueda exacta
		matchExpr = fmt.Sprintf(`%s:%s`, field, escapeFTS5Value(lowerValue))
	}

	// Query que une records con FTS5
	query := `
		SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			r.primer_nombre, r.segundo_nombre, r.cod_centro
		FROM records r 
		JOIN records_fts fts ON r.id = fts.rowid 
		WHERE records_fts MATCH ?`

	rows, err := sm.db.Query(query, matchExpr)
	if err != nil {
		// Fallback a LIKE si FTS5 falla
		return sm.searchWithLike(field, value, pattern)
	}
	defer rows.Close()

	return sm.scanRecords(rows)
}

// searchWithLike es el fallback cuando FTS5 no está disponible.
//
// Usa el operador LIKE de SQL:
//   - Contains: LIKE '%valor%'
//   - StartsWith: LIKE 'valor%'
func (sm *SQLiteManager) searchWithLike(field, value string, pattern SearchPattern) ([]Record, error) {
	var query string
	var args []interface{}

	switch pattern {
	case PatternContains:
		query = fmt.Sprintf(`SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			r.primer_nombre, r.segundo_nombre, r.cod_centro
			FROM records r WHERE r.%s LIKE ?`, field)
		args = []interface{}{"%" + value + "%"}
	case PatternStartsWith:
		query = fmt.Sprintf(`SELECT r.nacionalidad, r.dni, r.primer_apellido, r.segundo_apellido,
			r.primer_nombre, r.segundo_nombre, r.cod_centro
			FROM records r WHERE r.%s LIKE ?`, field)
		args = []interface{}{value + "%"}
	default:
		return nil, fmt.Errorf("patrón no soportado para LIKE: %s", pattern)
	}

	rows, err := sm.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error en query LIKE: %w", err)
	}
	defer rows.Close()

	return sm.scanRecords(rows)
}

// escapeFTS5Value escapa caracteres especiales para FTS5.
//
// FTS5 tiene caracteres especiales que necesitan escape.
// Escapes: comillas dobles (")
func escapeFTS5Value(value string) string {
	// Comillasdoblesescapadas
	result := value
	result = replaceAllString(result, `"`, `""`)
	return result
}

// replaceAllString reemplaza todas las ocurrencias de old en s.
//
// Helper que no usa librería estándar para mantener simple.
func replaceAllString(s, old, new string) string {
	result := s
	for {
		i := findIndex(result, old)
		if i == -1 {
			break
		}
		result = result[:i] + new + result[i+len(old):]
	}
	return result
}

// findIndex retorna el índice de la primera ocurrencia de substr.
func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// SearchAll combina búsquedas de múltiples campos.
//
// Aplica lógica AND u OR:
//   - AND: intersección de resultados
//   - OR: unión de resultados
//
// Proceso:
//  1. Buscar con primera condición
//  2. Para cada condición adicional:
//     - Si AND: intersect con resultados actuales
//     - Si OR: union con resultados actuales
func (sm *SQLiteManager) SearchAll(conditions []SearchCondition, logic SearchLogic) ([]Record, error) {
	if len(conditions) == 0 {
		return nil, nil
	}

	// Primera condición
	first := conditions[0]
	results, err := sm.SearchByField(first.Field, first.Value, first.Pattern)
	if err != nil {
		return nil, err
	}

	// Condiciones restantes
	for i := 1; i < len(conditions); i++ {
		cond := conditions[i]
		more, err := sm.SearchByField(cond.Field, cond.Value, cond.Pattern)
		if err != nil {
			return nil, err
		}

		if logic == LogicAND {
			results = intersectRecords(results, more)
		} else {
			results = unionRecords(results, more)
		}
	}

	return results, nil
}

// scanRecords convierte filas SQL a structs Record.
//
// Itera sobre las filas y hace Scan a cada una.
// Go unmarshal automático: cada columna se mapea al campo correspondiente.
func (sm *SQLiteManager) scanRecords(rows *sql.Rows) ([]Record, error) {
	var records []Record
	for rows.Next() {
		var r Record
		err := rows.Scan(
			&r.Nacionalidad,
			&r.DNI,
			&r.Primer_Apellido,
			&r.Segundo_Apellido,
			&r.Primer_Nombre,
			&r.Segundo_Nombre,
			&r.Cod_Centro,
		)
		if err != nil {
			return nil, fmt.Errorf("error al escanear fila: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// GetRecordCount retorna el número total de registros en la tabla.
func (sm *SQLiteManager) GetRecordCount() (int, error) {
	var count int
	err := sm.db.QueryRow("SELECT COUNT(*) FROM records").Scan(&count)
	return count, err
}
