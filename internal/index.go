package internal

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Index representa un índice en memoria para búsquedas rápidas en archivos CSV.
//
// El índice es una estructura de datos que permite búsquedas O(1) en lugar de O(n).
// En lugar de escanear todo el archivo, consultamos el mapa directamente.
//
// Estructura:
//   - Cada campo tiene su propio mapa (map[string][]Record)
//   - La clave es el valor del campo (ej: DNI "12345678")
//   - El valor es una lista de registros que tienen ese valor
//
// Ventajas:
//   - Búsqueda por DNI: < 1ms
//   - Búsqueda por nombre: < 1ms
//   - Persistible a JSON para reuse
//
// Desventajas:
//   - Usa RAM para todo el índice
//   - Debe rebuild si el CSV cambia
//
// Ejemplo de uso:
//
//	// Construir índice
//	index, _ := internal.BuildIndex("datos.csv", 4)
//	index.Save("index.json")
//
//	// Cargar índice existente
//	index, _ := internal.LoadIndex("index.json")
//
//	// Buscar
//	results := index.SearchAll(conditions, internal.LogicAND)
type Index struct {
	// DNI es un mapa donde la clave es el número de cédula
	// y el valor es una lista de registros con ese DNI.
	// Ejemplo: DNI["12345678"] = [{Registro con DNI 12345678}]
	DNI map[string][]Record `json:"dni"`

	// PrimerNombre mapea primer nombre a registros.
	// Permite búsqueda rápida por primer nombre.
	PrimerNombre map[string][]Record `json:"primer_nombre"`

	// SegundoNombre mapea segundo nombre a registros.
	SegundoNombre map[string][]Record `json:"segundo_nombre"`

	// PrimerApellido mapea primer apellido a registros.
	PrimerApellido map[string][]Record `json:"primer_apellido"`

	// SegundoApellido mapea segundo apellido a registros.
	SegundoApellido map[string][]Record `json:"segundo_apellido"`

	// TotalRecords indica cuántos registros hay indexados.
	TotalRecords int `json:"total_records"`

	// mu es un RWMutex para operaciones thread-safe.
	// No se serializa a JSON (tag "json:"-").
	mu sync.RWMutex `json:"-"`
}

// NewIndex crea una nueva estructura de índice vacía.
//
// Inicializa todos los mapas internos con make().
// Debe llamarse antes de agregar registros al índice.
//
// Retorna:
//   - *Index: puntero a un índice vacío y listo para usar
func NewIndex() *Index {
	return &Index{
		// make(map[string][]Record) crea un mapa vacío
		// Go usa mapas dispersos (sparse), no desperdician memoria
		DNI:             make(map[string][]Record),
		PrimerNombre:    make(map[string][]Record),
		SegundoNombre:   make(map[string][]Record),
		PrimerApellido:  make(map[string][]Record),
		SegundoApellido: make(map[string][]Record),
	}
}

// BuildIndex construye un índice a partir de un archivo CSV.
//
// Esta función es un wrapper que llama a BuildIndexStreaming
// con un contexto vacío (sin cancelación).
//
// Para archivos grandes (30M+), BuildIndexStreaming es preferible
// porque no carga todo en memoria.
//
// Parámetros:
//   - csvPath: ruta al archivo CSV
//   - workers: número de workers (actualmente no usado en streaming)
//
// Retorna:
//   - *Index: índice construido
//   - error: error si falla
func BuildIndex(csvPath string, workers int) (*Index, error) {
	return BuildIndexStreaming(context.Background(), csvPath, workers)
}

// BuildIndexStreaming construye un índice usando streaming.
//
// Esta es la función PRINCIPAL para construir índices de archivos grandes.
// NO carga todos los registros en memoria - procesa streaming.
//
// Cómo funciona:
//  1. Un goroutine lee el CSV línea por línea
//  2. Cada registro se agrega a TODOS los mapas (DNI, nombres, apellidos)
//  3. Se usa mutex para proteger actualizaciones al mapa compartido
//  4. Cada 2 segundos muestra progreso
//
// La búsqueda es "exacta": el valor debe ser idéntico.
// Para búsquedas parciales (contiene, comienza con), usar SQLite.
//
// Parámetros:
//   - ctx: contexto para cancelación
//   - csvPath: ruta al archivo CSV
//   - workers: número de workers (parámetro para compatibilidad)
//
// Retorna:
//   - *Index: índice construido
//   - error: error si falla
func BuildIndexStreaming(ctx context.Context, csvPath string, workers int) (*Index, error) {
	// Abrir archivo CSV
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir archivo: %w", err)
	}
	defer file.Close()

	// Crear índice vacío
	index := NewIndex()

	// Mutex para proteger actualizaciones al índice compartido
	// Lock() bloquea escritura, RLock() permite lecturas concurrentes
	var mu sync.Mutex

	// Contador atómico de registros
	var totalRecords int64

	// Mutex y timestamp para reporte de progreso
	var progressMu sync.Mutex
	lastProgress := time.Now()

	// Channel para errores del reader
	readerErr := make(chan error, 1)

	// ===== GOROUTINE: READER =====
	// Lee el CSV y construye el índice
	go func() {
		reader := csv.NewReader(file)
		reader.Comma = ';' // Separador del CSV venezolano

		// Saltar header
		if _, err := reader.Read(); err != nil {
			readerErr <- fmt.Errorf("error al leer header: %w", err)
			return
		}

		// Bucle de lectura
		for {
			// Verificar cancelación
			select {
			case <-ctx.Done():
				readerErr <- ctx.Err()
				return
			default:
			}

			// Leer siguiente línea
			record, err := reader.Read()
			if err == io.EOF {
				readerErr <- nil // Signal de terminado
				return
			}
			// Skip errores y registros cortos
			if err != nil || len(record) < 7 {
				continue
			}

			// Convertir a struct Record
			rec := parseRecord(record)

			// AGREGAR AL ÍNDICE (sección crítica)
			// Bloqueamos durante la actualización del mapa
			// para evitar data races
			mu.Lock()

			// Agregar a cada mapa:
			// - Clave: el valor del campo
			// - Valor: append al slice existente (crea nuevo slice)
			index.DNI[rec.DNI] = append(index.DNI[rec.DNI], rec)
			index.PrimerNombre[rec.Primer_Nombre] = append(index.PrimerNombre[rec.Primer_Nombre], rec)
			index.SegundoNombre[rec.Segundo_Nombre] = append(index.SegundoNombre[rec.Segundo_Nombre], rec)
			index.PrimerApellido[rec.Primer_Apellido] = append(index.PrimerApellido[rec.Primer_Apellido], rec)
			index.SegundoApellido[rec.Segundo_Apellido] = append(index.SegundoApellido[rec.Segundo_Apellido], rec)

			mu.Unlock()

			// Incrementar contador
			atomic.AddInt64(&totalRecords, 1)

			// Reportar progreso cada 2 segundos
			progressMu.Lock()
			if time.Since(lastProgress) > 2*time.Second {
				fmt.Printf("\rIndexando: %d registros...", atomic.LoadInt64(&totalRecords))
				lastProgress = time.Now()
			}
			progressMu.Unlock()
		}
	}()

	// Esperar a que termine el reader
	err = <-readerErr
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("error del reader: %w", err)
	}

	// Actualizar total de registros
	index.TotalRecords = int(atomic.LoadInt64(&totalRecords))

	fmt.Printf("\rIndexando: %d registros... ¡hecho!\n", index.TotalRecords)

	return index, nil
}

// buildIndexSequential construye el índice de forma secuencial (sin paralelismo).
//
// Esta función es útil para archivos pequeños o cuando se tienen
// todos los registros en memoria.
//
// Ventaja: simple y sin overhead de goroutines
// Desventaja: no aprovecha múltiples cores
//
// Parámetros:
//   - records: slice con todos los registros
//
// Retorna:
//   - *Index: índice construido
func buildIndexSequential(records []Record) *Index {
	index := NewIndex()
	index.TotalRecords = len(records)

	// Iterar sobre todos los registros
	for _, r := range records {
		// Agregar a cada mapa
		index.DNI[r.DNI] = append(index.DNI[r.DNI], r)
		index.PrimerNombre[r.Primer_Nombre] = append(index.PrimerNombre[r.Primer_Nombre], r)
		index.SegundoNombre[r.Segundo_Nombre] = append(index.SegundoNombre[r.Segundo_Nombre], r)
		index.PrimerApellido[r.Primer_Apellido] = append(index.PrimerApellido[r.Primer_Apellido], r)
		index.SegundoApellido[r.Segundo_Apellido] = append(index.SegundoApellido[r.Segundo_Apellido], r)
	}

	return index
}

// buildIndexParallel construye el índice usando múltiples workers.
//
// Implementa el patrón:
//  1. Particionar registros en chunks
//  2. Workers construyen índices parciales
//  3. Mergear índices parciales en uno solo
//
// Ventaja: aprovecha múltiples cores
// Desventaja: más complejo, overhead de coordinación
//
// Parámetros:
//   - ctx: contexto para cancelación
//   - records: todos los registros
//   - workers: número de workers
//
// Retorna:
//   - *Index: índice mergeado
//   - error: error si falla
func buildIndexParallel(ctx context.Context, records []Record, workers int) (*Index, error) {
	index := NewIndex()
	index.TotalRecords = len(records)

	// Calcular tamaño de chunk
	// Cada worker procesa chunkSize registros
	chunkSize := len(records) / workers
	if chunkSize < 1000 {
		chunkSize = 1000 // Mínimo para evitar overhead
	}

	// partialIndex es un índice temporal creado por cada worker
	type partialIndex struct {
		dni             map[string][]Record
		primerNombre    map[string][]Record
		segundoNombre   map[string][]Record
		primerApellido  map[string][]Record
		segundoApellido map[string][]Record
	}

	// workItem representa un chunk de trabajo
	type workItem struct {
		id    int
		start int
		end   int
	}

	// Channels para comunicación
	workChan := make(chan workItem, workers)
	results := make(chan partialIndex, workers)

	// WaitGroup para esperar workers
	var wg sync.WaitGroup

	// Iniciar workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-workChan:
					if !ok {
						return
					}

					// Crear índice parcial
					pi := partialIndex{
						dni:             make(map[string][]Record),
						primerNombre:    make(map[string][]Record),
						segundoNombre:   make(map[string][]Record),
						primerApellido:  make(map[string][]Record),
						segundoApellido: make(map[string][]Record),
					}

					// Procesar chunk
					for _, r := range records[item.start:item.end] {
						pi.dni[r.DNI] = append(pi.dni[r.DNI], r)
						pi.primerNombre[r.Primer_Nombre] = append(pi.primerNombre[r.Primer_Nombre], r)
						pi.segundoNombre[r.Segundo_Nombre] = append(pi.segundoNombre[r.Segundo_Nombre], r)
						pi.primerApellido[r.Primer_Apellido] = append(pi.primerApellido[r.Primer_Apellido], r)
						pi.segundoApellido[r.Segundo_Apellido] = append(pi.segundoApellido[r.Segundo_Apellido], r)
					}

					// Enviar índice parcial
					results <- pi
				}
			}
		}()
	}

	// Enviar work items
	go func() {
		for i := 0; i < len(records); i += chunkSize {
			end := i + chunkSize
			if end > len(records) {
				end = len(records)
			}
			workChan <- workItem{start: i, end: end}
		}
		close(workChan)
	}()

	// Esperar y cerrar results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Merge: combinar índices parciales en el índice principal
	for pi := range results {
		// Merge DNI
		for k, v := range pi.dni {
			index.DNI[k] = append(index.DNI[k], v...)
		}
		// Merge PrimerNombre
		for k, v := range pi.primerNombre {
			index.PrimerNombre[k] = append(index.PrimerNombre[k], v...)
		}
		// Merge SegundoNombre
		for k, v := range pi.segundoNombre {
			index.SegundoNombre[k] = append(index.SegundoNombre[k], v...)
		}
		// Merge PrimerApellido
		for k, v := range pi.primerApellido {
			index.PrimerApellido[k] = append(index.PrimerApellido[k], v...)
		}
		// Merge SegundoApellido
		for k, v := range pi.segundoApellido {
			index.SegundoApellido[k] = append(index.SegundoApellido[k], v...)
		}
	}

	return index, nil
}

// Save guarda el índice a un archivo JSON.
//
// Serializa el índice completo (excepto mutex) a formato JSON.
// El archivo puede luego cargarse con LoadIndex().
//
// El formato JSON permite:
//   - Persistencia del índice
//   - Compartir índice entre ejecuciones
//   - Inspección manual del índice
//
// Parámetros:
//   - path: ruta donde guardar el archivo JSON
//
// Retorna:
//   - error: error si falla
func (idx *Index) Save(path string) error {
	// Crear archivo (sobrescribe si existe)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error al crear archivo: %w", err)
	}
	defer file.Close()

	// Crear encoder JSON con indentación para legibilidad
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // 2 espacios por nivel

	// El campo `mu` tiene tag `json:"-"` así que se ignora
	return encoder.Encode(idx)
}

// LoadIndex carga un índice desde un archivo JSON.
//
// Lee el archivo JSON y lo deserializa a un struct Index.
// El índice debe haber sido guardado previamente con Save().
//
// Parámetros:
//   - path: ruta al archivo JSON
//
// Retorna:
//   - *Index: índice cargado
//   - error: error si falla
func LoadIndex(path string) (*Index, error) {
	// Abrir archivo
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error al abrir archivo: %w", err)
	}
	defer file.Close()

	// Crear decoder JSON
	var index Index
	decoder := json.NewDecoder(file)

	// Decode JSON a struct
	// Los campos del JSON se mapean a los campos del struct
	// usando los tags json:"..."
	if err := decoder.Decode(&index); err != nil {
		return nil, fmt.Errorf("error al decodificar JSON: %w", err)
	}

	return &index, nil
}

// SearchAll busca en el índice con múltiples condiciones.
//
// Aplica lógica AND u OR según el parámetro logic:
//   - AND: intersección de resultados (solo registros que cumplen TODAS)
//   - OR: unión de resultados (registros que cumplen AL MENOS UNA)
//
// Si no hay condiciones, retorna nil.
//
// El proceso:
//  1. Obtener resultados de la primera condición
//  2. Para cada condición adicional:
//     - Si AND: intersectar con resultados actuales
//     - Si OR: fusionar con resultados actuales
//
// Parámetros:
//   - conditions: slice de condiciones de búsqueda
//   - logic: lógica de combinación (AND u OR)
//
// Retorna:
//   - []Record: registros que coinciden
func (idx *Index) SearchAll(conditions []SearchCondition, logic SearchLogic) []Record {
	// Sin condiciones = sin resultados
	if len(conditions) == 0 {
		return nil
	}

	// Obtener resultados de la primera condición
	results := idx.searchByCondition(conditions[0])

	// Aplicar condiciones restantes
	for i := 1; i < len(conditions); i++ {
		// Buscar con esta condición
		more := idx.searchByCondition(conditions[i])

		// Combinar según lógica
		if logic == LogicAND {
			// AND: solo los que están en ambos
			results = intersectRecords(results, more)
		} else {
			// OR: todos los de ambos (sin duplicados)
			results = unionRecords(results, more)
		}
	}

	return results
}

// searchByCondition busca por una sola condición.
//
// Realiza una búsqueda directa en el mapa correspondiente.
// O(1) para búsquedas exactas.
//
// Parámetros:
//   - cond: condición de búsqueda
//
// Retorna:
//   - []Record: registros que cumplen la condición
func (idx *Index) searchByCondition(cond SearchCondition) []Record {
	var records []Record

	// Seleccionar mapa según campo
	switch cond.Field {
	case "dni":
		// Búsqueda directa en mapa: O(1)
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

// intersectRecords retorna la intersección de dos slices de registros.
//
// La intersección contiene solo los registros que aparecen en AMBOS slices.
// Se usa cuando la lógica de búsqueda es AND.
//
// Implementación:
//   - Crear un "set" (mapa) con los DNI del slice B
//   - Iterar sobre A y agregar solo los que están en el set
//
// Complejidad: O(n + m) donde n = len(a), m = len(b)
//
// Parámetros:
//   - a: primer slice de registros
//   - b: segundo slice de registros
//
// Retorna:
//   - []Record: registros en ambos slices (sin duplicados)
func intersectRecords(a, b []Record) []Record {
	// Casos especiales
	if len(a) == 0 || len(b) == 0 {
		return nil
	}

	// Crear set con DNI del slice b
	bSet := make(map[string]bool)
	for _, r := range b {
		bSet[r.DNI] = true
	}

	// Encontrar intersección
	var result []Record
	for _, r := range a {
		if bSet[r.DNI] {
			result = append(result, r)
		}
	}
	return result
}

// unionRecords retorna la unión de dos slices de registros.
//
// La unión contiene todos los registros de AMBOS slices,
// sin duplicados (por DNI).
// Se usa cuando la lógica de búsqueda es OR.
//
// Implementación:
//   - Iterar sobre A y agregar los no-vistos
//   - Iterar sobre B y agregar los no-vistos
//
// Complejidad: O(n + m)
//
// Parámetros:
//   - a: primer slice de registros
//   - b: segundo slice de registros
//
// Retorna:
//   - []Record: todos los registros de ambos slices (sin duplicados)
func unionRecords(a, b []Record) []Record {
	// Mapa para trackear ya agregados
	seen := make(map[string]bool)
	var result []Record

	// Agregar registros de A (sin duplicados)
	for _, r := range a {
		if !seen[r.DNI] {
			seen[r.DNI] = true
			result = append(result, r)
		}
	}

	// Agregar registros de B (sin duplicados)
	for _, r := range b {
		if !seen[r.DNI] {
			seen[r.DNI] = true
			result = append(result, r)
		}
	}

	return result
}
