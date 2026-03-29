package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

// Constantes de configuración por defecto.
//
// Estos valores se usan cuando el usuario no especifica una ruta
// o valor particular. Proporcionan una experiencia de usuario
// predecible sin necesidad de configurar todo manualmente.
//
// Por qué usamos这些默认值:
// - Conveniencia: funciona sin argumentos en el directorio correcto
// - Consistencia: todos los comandos usan las mismas rutas por defecto
// - Flexibilidad: el usuario puede sobrescribir cualquier valor
const (
	DefaultCSVPath   = "./nacional.csv" // Ruta por defecto para archivos CSV
	DefaultIndexPath = "./index.json"   // Ruta por defecto para archivos de índice
	DefaultDBPath    = "./data.db"      // Ruta por defecto para bases de datos SQLite
)

// SearchMode representa el modo de operación de la aplicación.
//
// Los tres modos son:
// - ModeCSV: búsqueda directa en archivo CSV (lento para archivos grandes)
// - ModeIndex: búsqueda usando índice en memoria (rápido, requiere construir índice)
// - ModeSQLite: búsqueda en base de datos SQLite (flexible, mejor para consultas complejas)
type SearchMode string

// Modos de búsqueda disponibles.
//
// Cada modo tiene ventajas y desventajas:
// CSV: simple pero lento
// Index: rápido pero requiere construir el índice primero
// SQLite: flexible y rápido para consultas complejas
const (
	ModeCSV    SearchMode = "csv"    // Búsqueda directa en CSV
	ModeIndex  SearchMode = "index"  // Búsqueda con índice en memoria
	ModeSQLite SearchMode = "sqlite" // Búsqueda en SQLite
)

// Config contiene toda la configuración de la CLI.
//
// Esta estructura es el "estado global" de la aplicación y es
// populada por Cobra durante el parseo de argumentos. Se usa
// para compartir configuración entre todos los comandos.
//
// Por qué una estructura global:
// - Cobra automáticamente llena los campos desde flags
// - Evita pasar parámetros a través de múltiples niveles
// - Simple para una CLI pequeña como esta
//
// Para aplicaciones más grandes, considere usar inyección de
// dependencias en lugar de variables globales.
type Config struct {
	Mode       SearchMode  // Modo de búsqueda (csv, index, sqlite)
	CSVPath    string      // Ruta al archivo CSV
	IndexPath  string      // Ruta al archivo de índice JSON
	DBPath     string      // Ruta a la base de datos SQLite
	BuildIndex bool        // Indica si se debe construir el índice
	BuildDB    bool        // Indica si se debe construir la base de datos
	Workers    int         // Número de workers para procesamiento paralelo (0=auto)
	Logic      SearchLogic // Lógica de búsqueda (AND/OR)
	UseFTS     *bool       //nil=auto, true=forzar FTS5, false=desactivar FTS5

	// Parámetros de búsqueda - cada uno puede tener su propio patrón
	DNI                    string        // Cédula de identidad (buscar por DNI)
	DNIPattern             SearchPattern // Patrón de búsqueda para DNI
	PrimerNombre           string        // Primer nombre
	PrimerNombrePattern    SearchPattern // Patrón para primer nombre
	SegundoNombre          string        // Segundo nombre
	SegundoNombrePattern   SearchPattern // Patrón para segundo nombre
	PrimerApellido         string        // Primer apellido
	PrimerApellidoPattern  SearchPattern // Patrón para primer apellido
	SegundoApellido        string        // Segundo apellido
	SegundoApellidoPattern SearchPattern // Patrón para segundo apellido
}

// cfg es la instancia global de configuración.
//
// Se declara a nivel de paquete para que pueda ser accedida por
// todas las funciones de cmd durante la inicialización de Cobra.
// Es populada automáticamente por Cobra cuando se parsean los flags.
var cfg Config

// SearchParams contiene los parámetros de búsqueda para los comandos.
//
// Esta estructura es una vista simplificada de Config que se usa
// para pasar parámetros a las funciones del paquete internal.
// Separamos Config (flags de CLI) de SearchParams (parámetros de búsqueda)
// para mantener separadas las responsabilidades.
type SearchParams struct {
	DNI             string      // Cédula de identidad
	PrimerNombre    string      // Primer nombre
	SegundoNombre   string      // Segundo nombre
	PrimerApellido  string      // Primer apellido
	SegundoApellido string      // Segundo apellido
	Logic           SearchLogic // Lógica de combinación
	Mode            SearchMode  // Modo de búsqueda
	CSVPath         string      // Ruta al CSV
	IndexPath       string      // Ruta al índice
	DBPath          string      // Ruta a la base de datos
	Workers         int         // Workers paralelos
}

// rootCmd es el comando raíz de la aplicación.
//
// rootCmd es el punto de entrada principal de la CLI. Todos los
// demás comandos (search, index, db) se agregan como subcomandos.
// El campo Use define cómo se invoca el comando: "leeCSV"
//
// Longitud del comando:
// El campo Long contiene la ayuda extensa que se muestra con --help.
// Incluye ejemplos de uso para que el usuario entienda rápidamente
// cómo usar la herramienta.
var rootCmd = &cobra.Command{
	Use:   "leeCSV", // Nombre del comando en CLI
	Short: "Buscar y filtrar registros en archivos CSV grandes",
	Long: `leeCSV es una herramienta CLI para buscar y filtrar registros en archivos CSV grandes
con datos de identificación nacional. Soporta múltiples modos de búsqueda: CSV, Índice y SQLite.

Ejemplos:
  # Buscar usando modo CSV
  leeCSV search --dni=12345678

  # Construir y buscar usando índice
  leeCSV index build --csv=data.csv
  leeCSV index search --dni=12345678

  # Construir y buscar usando SQLite
  leeCSV db build --csv=data.csv
  leeCSV db search --dni=12345678`,
	Version: "1.0.0",
}

// Execute añade todos los subcomandos al comando raíz y los ejecuta.
//
// Esta función es llamada desde main.go y es el punto de entrada
// a toda la lógica de CLI. Maneja el parseo de argumentos y la
// ejecución del comando apropiado.
//
// Por qué encapsulamos Execute en lugar de llamarr rootCmd.Execute() directamente:
// - Mantiene main.go limpio y mínimo
// - Permite agregar pre/post procesamiento si es necesario
// - Facilita el testing (podemos mockear si fuera necesario)
func Execute() error {
	return rootCmd.Execute()
}

// init() configura la CLI durante la inicialización del paquete.
//
// Cobra llama automáticamente a init() cuando se carga el paquete.
// Aquí registramos:
// - La función de inicialización (initConfig)
// - Los flags persistentes (disponibles para todos los subcomandos)
//
// Por qué usamos PersistentFlags en lugar de Flags:
// - Los flags persistentes se heredan por todos los subcomandos
// - Evita tener que definir --csv, --index, --db en cada comando
// - El usuario puede especificar estos valores una vez y se usan en todos lados
func init() {
	// OnInitialize se ejecuta antes de cada comando
	// Aquí configuramos valores que dependen de la detección automática
	cobra.OnInitialize(initConfig)

	// Flags globales - disponibles para todos los comandos
	// StringVar crea un flag que asigna directamente a cfg.CSVPath
	rootCmd.PersistentFlags().StringVar(&cfg.CSVPath, "csv", "./nacional.csv", "Ruta al archivo CSV")
	rootCmd.PersistentFlags().StringVar(&cfg.IndexPath, "index", "./index.json", "Ruta al archivo de índice")
	rootCmd.PersistentFlags().StringVar(&cfg.DBPath, "db", "./data.db", "Ruta a la base de datos SQLite")
	// workers con -w como shorthand parabreviatura
	rootCmd.PersistentFlags().IntVarP(&cfg.Workers, "workers", "w", 0, "Número de workers (0=automático)")
}

// initConfig se ejecuta antes de cada comando.
//
// Detecta valores automáticamente cuando no se especifican.
// Currently solo detecta el número óptimo de workers.
func initConfig() {
	// Auto-detectar workers si no se específico
	// Usamos <= 0 porque 0 significa "automático"
	if cfg.Workers <= 0 {
		cfg.Workers = getDefaultWorkers()
	}
}

// getDefaultWorkers retorna el número de CPUs disponibles.
//
// Por qué detectar automáticamente:
// - Optimiza el rendimiento para diferentes máquinas
// - En máquinas con muchos núcleos, usa todos
// - En máquinas con pocos núcleos, no satura el sistema
// - El usuario puede sobrescribir con --workers si lo desea
func getDefaultWorkers() int {
	// runtime.NumCPU() retorna el número de CPUs lógicas
	// Esto incluye hyper-threading en CPUs Intel/AMD
	return runtime.NumCPU()
}

// getSearchParams convierte la configuración global a SearchParams.
//
// Transforma la estructura Config (que contiene todos los flags de CLI)
// en SearchParams (que contiene solo los parámetros de búsqueda).
// Esta separación permite que las funciones de internal reciban
// únicamente lo que necesitan.
//
// Por qué hacemos esta conversión:
// - Mantiene las dependencias de cmd hacia internal mínimas
// - Si la configuración interna cambia, solo modificamos esta función
// - Facilita el testing: podemos crear SearchParams directamente
func getSearchParams() SearchParams {
	return SearchParams{
		DNI:             cfg.DNI,
		PrimerNombre:    cfg.PrimerNombre,
		SegundoNombre:   cfg.SegundoNombre,
		PrimerApellido:  cfg.PrimerApellido,
		SegundoApellido: cfg.SegundoApellido,
		Logic:           cfg.Logic,
		Mode:            cfg.Mode,
		CSVPath:         cfg.CSVPath,
		IndexPath:       cfg.IndexPath,
		DBPath:          cfg.DBPath,
		Workers:         cfg.Workers,
	}
}

// printResults muestra los resultados de búsqueda al usuario.
//
// Formatea e imprime cada registro encontrado de manera legible.
// Este formato es simple pero efectivo para depuración rápida.
//
// Por qué no usamos JSON o una tabla:
// - JSON requiere parseo mental para leer
// - Las tablas requieren librerías adicionales
// - Este formato simple funciona bien para la mayoría de casos
func printResults(results []Record) {
	fmt.Printf("Found %d matches\n", len(results))
	for _, r := range results {
		fmt.Printf("%s - %s %s, %s %s\n",
			r.DNI,
			r.Primer_Nombre,
			r.Segundo_Nombre,
			r.Primer_Apellido,
			r.Segundo_Apellido,
		)
	}
}

// exitWithError imprime un mensaje de error y termina el programa.
//
// Esta función es un "helper" de conveniencia que:
// - Imprime el mensaje a stderr (no stdout)
// - Incluye el error si está disponible
// - Termina el programa con código de salida 1
//
// Por qué usamos os.Exit en lugar de returning error:
// - En la CLI, después de mostrar el error no hay nada más que hacer
// - Simplifica el código del comando (no tiene que manejar errores)
// - El código de salida 1 indica error al shell/script que llamó
func exitWithError(msg string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", msg)
	}
	os.Exit(1)
}
