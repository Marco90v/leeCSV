package internal

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SearchChunkResult contiene los resultados del procesamiento de un chunk (lote).
//
// Un chunk es un grupo de registros que se procesan juntos.
// Esta estructura se usa para devolver los registros encontrados
// junto con la cantidad de registros procesados en ese chunk.
//
// Uso interno: principalmente para debugging y estadísticas.
type SearchChunkResult struct {
	// Records contiene los registros que coincidieron con la búsqueda
	Records []Record

	// Count es el número de registros procesados en este chunk
	Count int64
}

// SearchStats contiene estadísticas completas de una búsqueda paralela.
//
// Proporciona información sobre:
//   - Cuántos registros se procesaron
//   - Cuántos registros coincidieron
//   - Cuántos workers se usaron
//   - Cuántos chunks se procesaron
//
// Esta información es útil para:
//   - Monitorear progreso de búsquedas largas
//   - Optimizar el número de workers
//   - Depuración de problemas de rendimiento
type SearchStats struct {
	// TotalProcessed es el número total de registros leídos del archivo CSV
	TotalProcessed int64

	// TotalMatches es el número de registros que coincidieron con los criterios
	TotalMatches int64

	// WorkersUsed es el número de goroutines workers utilizados
	WorkersUsed int

	// ChunksProcessed es el número de chunks procesadas
	ChunksProcessed int64
}

// SearchParams contiene los parámetros para realizar una búsqueda en los registros.
//
// Define qué campos buscar y con qué valores.
// Los campos vacíos se ignoran en la búsqueda.
//
// Ejemplo de uso:
//
//	params := SearchParams{
//	    DNI:          "12345678",
//	    PrimerNombre: "Juan",
//	    Workers:      4,
//	}
type SearchParams struct {
	// DNI es el número de cédula a buscar (opcional)
	DNI string

	// PrimerNombre es el primer nombre a buscar (opcional)
	PrimerNombre string

	// SegundoNombre es el segundo nombre a buscar (opcional)
	SegundoNombre string

	// PrimerApellido es el primer apellido a buscar (opcional)
	PrimerApellido string

	// SegundoApellido es el segundo apellido a buscar (opcional)
	SegundoApellido string

	// Workers es el número de goroutines a usar para la búsqueda
	// Si es 0, se usará un valor por defecto (4)
	Workers int
}

// ReadFile abre un archivo CSV y retorna todos los registros.
//
// Esta función carga TODO el archivo en memoria RAM.
// ADVERTENCIA: No usar para archivos grandes (30M+ registros).
// Para archivos grandes, usar ReadFileWithContext.
//
// La función:
//  1. Abre el archivo CSV
//  2. Lee y descarta la primera línea (header)
//  3. Lee todas las líneas restantes
//  4. Convierte cada línea a un struct Record
//  5. Retorna un slice con todos los registros
//
// Parámetros:
//   - path: ruta al archivo CSV
//
// Retorna:
//   - []Record: todos los registros del archivo
//   - error: error si algo falla
//
// Ejemplo:
//
//	records, err := ReadFile("./datos.csv")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Leídos %d registros\n", len(records))
func ReadFile(path string) ([]Record, error) {
	return ReadFileWithContext(context.Background(), path)
}

// ReadFileWithContext abre un archivo CSV y retorna los registros con soporte de cancelación.
//
// Esta versión soporta context.Context para permitir:
//   - Cancelación de la operación (ej: Ctrl+C)
//   - Timeout (tiempo máximo de ejecución)
//   - Cancelación desde otro goroutine
//
// Esencial para archivos muy grandes (30M+ registros) donde
// las operaciones pueden tomar mucho tiempo.
//
// El contexto se verifica periódicamente durante la lectura.
// Si se cancela, la función retorna inmediatamente con error.
//
// Parámetros:
//   - ctx: contexto para control de cancelación
//   - path: ruta al archivo CSV
//
// Retorna:
//   - []Record: registros leídos hasta ahora (o vací si hay error)
//   - error: error si algo falla o se canceló
func ReadFileWithContext(ctx context.Context, path string) ([]Record, error) {
	// Abrir archivo CSV
	// os.Open retorna un *File que implementa io.Reader
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo CSV %s: %w", path, err)
	}
	// defer ejecuta file.Close() al salir de la función
	defer file.Close()

	// Crear lector CSV
	// csv.NewReader retorna un lector configurado para parsear CSV
	r := csv.NewReader(file)

	// Configurar separador: punto y coma (;) según formato venezolano
	r.Comma = ';'

	// Configurar carácter de comentario (líneas que empiezan con # se ignoran)
	r.Comment = '#'

	// Saltar primera línea (header del CSV)
	// El CSV tiene formato: Nacionalidad;Cedula;...
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("error al leer el header del CSV: %w", err)
	}

	// Slice para almacenar todos los registros
	// Se usa append() para agregar registros dinámicamente
	var records []Record

	// Bucle infinito que lee línea por línea
	for {
		// Verificar si el contexto fue cancelado
		// Esto permite interrumpir la lectura si es necesario
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("operación cancelada: %w", ctx.Err())
		default:
			// Continuar si no hay cancelación
		}

		// Leer siguiente línea del CSV
		record, err := r.Read()

		// io.EOF significa fin del archivo (no es un error)
		if err == io.EOF {
			break
		}

		// Otro error significa problema de lectura
		if err != nil {
			return nil, fmt.Errorf("error al leer registro del CSV: %w", err)
		}

		// Convertir línea CSV a struct Record y agregar al slice
		records = append(records, parseRecord(record))
	}

	return records, nil
}

// parseRecord convierte un registro (slice de strings) a un struct Record.
//
// Esta función es un "parser" que mapea los campos del CSV
// a los campos del struct. El orden es:
//
//	0: Nacionalidad
//	1: Cedula (DNI)
//	2: Primer_Apellido
//	3: Segundo_Apellido
//	4: Primer_Nombre
//	5: Segundo_Nombre
//	6: Cod_Centro
//
// Si el registro tiene menos de 7 campos, retorna un Record vacío.
//
// Esta función no retorna error porque assume que los datos
// ya fueron validados antes de llamar esta función.
//
// Parámetros:
//   - record: slice de strings con los campos del CSV
//
// Retorna:
//   - Record: struct con los datos parseados
func parseRecord(record []string) Record {
	// Validar que tenga suficientes campos
	// < 7 porque necesitamos al menos 7 campos (índice 0-6)
	if len(record) < 6 {
		return Record{}
	}

	// Mapear campos según posición en el CSV
	return Record{
		Nacionalidad:     record[0],
		DNI:              record[1],
		Primer_Apellido:  record[2],
		Segundo_Apellido: record[3],
		Primer_Nombre:    record[4],
		Segundo_Nombre:   record[5],
		Cod_Centro:       record[6],
	}
}

// matchesSearch verifica si un registro cumple con los criterios de búsqueda.
//
// La comparación es case-insensitive (no distingue mayúsculas de minúsculas)
// para todos los campos de texto. Esto significa que buscar "JUAN"
// encontrará "Juan", "juan", "JUAN", etc.
//
// La función usa "AND" implícito: TODOS los campos especificados
// deben coincidir para que el registro pase el filtro.
//
// Si un campo en params está vacío, ese campo SE IGNORA
// (no se usa como criterio de búsqueda).
//
// Parámetros:
//   - rec: registro a verificar
//   - params: parámetros de búsqueda
//
// Retorna:
//   - true: si el registro coincide con todos los criterios no-vacíos
//   - false: si no coincide con al menos un criterio
func matchesSearch(rec Record, params SearchParams) bool {
	// Verificar DNI si se especificó
	// strings.EqualFold hace comparación case-insensitive
	if params.DNI != "" && !strings.EqualFold(rec.DNI, params.DNI) {
		return false
	}

	// Verificar Primer Nombre
	if params.PrimerNombre != "" && !strings.EqualFold(rec.Primer_Nombre, params.PrimerNombre) {
		return false
	}

	// Verificar Segundo Nombre
	if params.SegundoNombre != "" && !strings.EqualFold(rec.Segundo_Nombre, params.SegundoNombre) {
		return false
	}

	// Verificar Primer Apellido
	if params.PrimerApellido != "" && !strings.EqualFold(rec.Primer_Apellido, params.PrimerApellido) {
		return false
	}

	// Verificar Segundo Apellido
	if params.SegundoApellido != "" && !strings.EqualFold(rec.Segundo_Apellido, params.SegundoApellido) {
		return false
	}

	// Todos los campos especificados coinciden
	return true
}

// SearchCSV realiza una búsqueda secuencial en los registros.
//
// Esta función itera sobre todos los registros uno por uno
// y retorna los que coinciden con los criterios de búsqueda.
//
// ADVERTENCIA: Esta función carga todos los registros en memoria.
// Para archivos grandes, usar SearchCSVStreaming.
//
// Características:
//   - Búsqueda secuencial (no paralela)
//   - Case-insensitive para todos los campos
//   - Lógica AND: todos los criterios deben cumplirse
//
// Parámetros:
//   - records: slice con todos los registros a buscar
//   - params: criterios de búsqueda
//
// Retorna:
//   - []Record: registros que coinciden
func SearchCSV(records []Record, params SearchParams) []Record {
	var results []Record

	// Iterar sobre cada registro
	for _, r := range records {
		// Verificar si coincide
		if matchesSearch(r, params) {
			// Agregar a resultados si coincide
			results = append(results, r)
		}
	}

	return results
}

// SearchCSVConcurrent realiza una búsqueda concurrente usando múltiples goroutines.
//
// Esta función implementa el patrón "worker pool":
//   - Un producer envía registros a un channel
//   - Múltiples workers procesan registros en paralelo
//   - Un consumer收集 resultados
//
// Es más rápida que SearchCSV para archivos grandes,
// pero aún carga todos los registros en memoria.
//
// Parámetros:
//   - records: slice con todos los registros
//   - params: criterios de búsqueda
//
// Retorna:
//   - []Record: registros que coinciden
func SearchCSVConcurrent(records []Record, params SearchParams) []Record {
	// Determinar número de workers
	workers := params.Workers
	if workers <= 0 {
		workers = 4 // Valor por defecto
	}

	// Channel para enviar registros del producer a los workers
	// El buffer size afecta el rendimiento: más buffer = más memoria pero más rápido
	input := make(chan Record, workers)

	// Channel para enviar resultados de los workers al consumer
	output := make(chan Record)

	// WaitGroup para esperar que todos los workers terminen
	var wg sync.WaitGroup

	// Función worker: procesa registros del input channel
	worker := func(jobs <-chan Record, results chan<- Record) {
		for {
			select {
			case job, ok := <-jobs:
				// Si el channel está cerrado, terminar
				if !ok {
					return
				}
				// Verificar si coincide y enviar a resultados
				if matchesSearch(job, params) {
					results <- job
				}
			}
		}
	}

	// Iniciar workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(input, output)
		}()
	}

	// Producer: enviar todos los registros al channel de entrada
	go func() {
		for _, r := range records {
			input <- r
		}
		close(input) // Cerrar cuando termine de enviar
	}()

	// Consumer: esperar workers y cerrar output
	go func() {
		wg.Wait()
		close(output)
	}()

	// Recolectar resultados
	var results []Record
	for r := range output {
		results = append(results, r)
	}

	return results
}

// SearchCSVStreaming realiza una búsqueda paralela con streaming en un archivo CSV.
//
// Esta es la función PRINCIPAL para archivos grandes (30M+ registros).
// NO carga todo el archivo en memoria - procesa streaming.
//
// Implementa el patrón de pipeline:
//  1. Reader goroutine: lee líneas del CSV y las envía a workers
//  2. Worker pool: múltiples goroutines procesan registros en paralelo
//  3. Collector: recibe resultados de los workers
//  4. Reporter: muestra progreso cada 2 segundos
//
// Características clave:
//   - Streaming: no carga todo el archivo en RAM
//   - Paralelo: usa múltiples workers
//   - Backpressure: si workers están lentos, reader espera
//   - Progreso: muestra estadísticas cada 2 segundos
//   - Cancellation: soporta context para cancelar operación
//
// Parámetros:
//   - ctx: contexto para cancelación y timeouts
//   - csvPath: ruta al archivo CSV
//   - params: criterios de búsqueda
//
// Retorna:
//   - []Record: registros que coinciden
//   - *SearchStats: estadísticas de la búsqueda
//   - error: error si algo falla
//
// Ejemplo:
//
//	ctx := context.Background()
//	params := SearchParams{DNI: "12345678", Workers: 4}
//	results, stats, err := SearchCSVStreaming(ctx, "datos.csv", params)
func SearchCSVStreaming(ctx context.Context, csvPath string, params SearchParams) ([]Record, *SearchStats, error) {
	// Determinar número de workers
	workers := params.Workers
	if workers <= 0 {
		workers = 4
	}

	// Abrir archivo CSV
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error al abrir archivo: %w", err)
	}
	defer file.Close()

	// Channel para pasar registros del reader a los workers
	// Buffer: workers * 100 = 400 registros en memoria máxima
	// Este es el corazón del streaming: regulamos cuántos registros
	// hay en memoria a la vez
	recordChan := make(chan Record, workers*100)

	// Channel para pasar resultados de workers al collector
	resultChan := make(chan []Record, workers)

	// Variables atómicas para estadísticas compartidas
	// Usamos atomic porque múltiples goroutines escriben estas variables
	var totalProcessed int64
	var totalMatches int64

	// ===== GOROUTINE 1: READER =====
	// Lee líneas del CSV y las envía a los workers
	var readerErr error
	go func() {
		defer close(recordChan) // Cerrar cuando termine

		reader := csv.NewReader(file)
		reader.Comma = ';'   // Separador del CSV venezolano
		reader.Comment = '#' // Líneas que empiezan con # son comentarios

		// Saltar header
		if _, err := reader.Read(); err != nil {
			readerErr = fmt.Errorf("error al leer header: %w", err)
			return
		}

		// Bucle infinito de lectura
		for {
			// Verificar cancelación
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Leer siguiente línea
			record, err := reader.Read()
			if err == io.EOF {
				return // Fin del archivo
			}
			if err != nil {
				// Skip malformed records - continuar con siguiente
				continue
			}

			// Skip registros con campos insuficientes
			if len(record) < 7 {
				continue
			}

			// Convertir a struct Record
			rec := parseRecord(record)

			// Enviar al worker (bloquea si el channel está lleno)
			// Esto es "backpressure": si workers no dan abasto,
			// el reader espera y no consume más memoria
			select {
			case recordChan <- rec:
			case <-ctx.Done():
				return
			}

			// Incrementar contador atómico
			atomic.AddInt64(&totalProcessed, 1)
		}
	}()

	// ===== GOROUTINE 2: WORKER POOL =====
	// Múltiples workers procesan registros en paralelo
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Resultados locales del worker
			localResults := []Record{}

			for {
				select {
				case <-ctx.Done():
					return

				case rec, ok := <-recordChan:
					// Si channel cerrado, enviar resultados restantes y salir
					if !ok {
						if len(localResults) > 0 {
							resultChan <- localResults
						}
						return
					}

					// Verificar si coincide con búsqueda
					if matchesSearch(rec, params) {
						localResults = append(localResults, rec)
						atomic.AddInt64(&totalMatches, 1)
					}

					// Enviar resultados cada 100 registros
					// Evita acumular muchos en memoria
					if len(localResults) >= 100 {
						resultChan <- localResults
						localResults = nil // Reiniciar slice
					}
				}
			}
		}(w)
	}

	// ===== GOROUTINE 3: CLEANUP =====
	// Espera workers y cierra result channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// ===== GOROUTINE 4: PROGRESS REPORTER =====
	// Muestra progreso cada 2 segundos
	progressCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-progressCtx.Done():
				return
			case <-ticker.C:
				// Cargar valores atómicos de forma segura
				current := atomic.LoadInt64(&totalProcessed)
				matches := atomic.LoadInt64(&totalMatches)
				// \r permite sobreescribir la línea (efecto de consola)
				fmt.Printf("\rProcesados: %d registros, Coincidencias: %d", current, matches)
			}
		}
	}()

	// ===== MAIN: COLLECT RESULTS =====
	// Recolectar resultados de todos los workers
	var allRecords []Record
	for results := range resultChan {
		allRecords = append(allRecords, results...)
	}

	// Cancelar reporter de progreso
	cancel()

	// Verificar errores del reader
	if readerErr != nil && readerErr != io.EOF {
		return allRecords, &SearchStats{
			TotalProcessed: atomic.LoadInt64(&totalProcessed),
			TotalMatches:   atomic.LoadInt64(&totalMatches),
			WorkersUsed:    workers,
		}, readerErr
	}

	// Retornar resultados con estadísticas
	stats := &SearchStats{
		TotalProcessed: atomic.LoadInt64(&totalProcessed),
		TotalMatches:   atomic.LoadInt64(&totalMatches),
		WorkersUsed:    workers,
	}

	return allRecords, stats, nil
}
