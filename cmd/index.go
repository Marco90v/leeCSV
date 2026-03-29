package cmd

import (
	"fmt"
	"os"

	"go/csv/internal"

	"github.com/spf13/cobra"
)

// indexCmd es el comando padre para operaciones con índice.
//
// Este comando agrupa dos subcomandos:
// - index build: construye un índice desde un CSV
// - index search: busca usando un índice existente
//
// El modo Index ofrece:
// - Búsquedas muy rápidas (índice en memoria)
// - Requiere construir el índice una vez
// - Ideal para búsquedas frecuentes sobre el mismo dataset
var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Operaciones de índice (construir, buscar)",
	Long:  `Construir y buscar usando índice en memoria para búsquedas rápidas.`,
}

func init() {
	// Agregar como subcomando del raíz
	rootCmd.AddCommand(indexCmd)
}

// indexBuildCmd construye un índice desde un archivo CSV.
//
// Este comando:
// 1. Lee el archivo CSV especificado
// 2. Construye un índice en memoria con todos los registros
// 3. Guarda el índice en un archivo JSON para uso futuro
//
// Por qué dividir en dos pasos:
// - Construcción: solo se hace una vez por archivo
// - Búsqueda: se puede hacer muchas veces sin reconstruir
// - El índice se guarda en disco para reutilización
var indexBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Construir índice desde CSV",
	Long: `Construir un índice en memoria desde un archivo CSV.
El índice permite búsquedas rápidas en búsquedas posteriores.`,
	Run: func(cmd *cobra.Command, args []string) {
		runIndexBuild()
	},
}

func init() {
	// Agregar como subcomando de index
	indexCmd.AddCommand(indexBuildCmd)

	// Flags específicos para construir índice
	indexBuildCmd.Flags().StringVar(&cfg.CSVPath, "csv", DefaultCSVPath, "Ruta al archivo CSV")
	indexBuildCmd.Flags().StringVar(&cfg.IndexPath, "index", DefaultIndexPath, "Ruta al archivo de índice")
	indexBuildCmd.Flags().IntVarP(&cfg.Workers, "workers", "w", 0, "Número de workers (0=automático)")
}

// indexSearchCmd busca usando un índice existente.
//
// Este comando:
// 1. Carga el índice desde el archivo JSON
// 2. Aplica las condiciones de búsqueda especificadas
// 3. Retorna los registros que coinciden
//
// Ventajas sobre CSV mode:
// - Búsqueda casi instantánea (índice en memoria)
// - No requiere leer el CSV completo
// - Muy eficiente para millones de registros
var indexSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Buscar usando índice",
	Long:  `Buscar registros usando un archivo de índice existente.`,
	Run: func(cmd *cobra.Command, args []string) {
		runIndexSearch()
	},
}

func init() {
	indexCmd.AddCommand(indexSearchCmd)

	// Flags para búsqueda con índice
	indexSearchCmd.Flags().StringVar(&cfg.IndexPath, "index", DefaultIndexPath, "Ruta al archivo de índice")
	indexSearchCmd.Flags().StringVar(&cfg.DNI, "dni", "", "Buscar por DNI")
	indexSearchCmd.Flags().StringVar(&cfg.PrimerNombre, "primer-nombre", "", "Buscar por primer nombre")
	indexSearchCmd.Flags().StringVar(&cfg.SegundoNombre, "segundo-nombre", "", "Buscar por segundo nombre")
	indexSearchCmd.Flags().StringVar(&cfg.PrimerApellido, "primer-apellido", "", "Buscar por primer apellido")
	indexSearchCmd.Flags().StringVar(&cfg.SegundoApellido, "segundo-apellido", "", "Buscar por segundo apellido")
	indexSearchCmd.Flags().StringVar((*string)(&cfg.Logic), "logic", "AND", "Lógica de búsqueda: AND, OR")
}

// runIndexBuild ejecuta la construcción del índice.
//
// Pasos:
// 1. Verificar que el archivo CSV exista
// 2. Construir el índice en memoria
// 3. Guardar el índice a disco
// 4. Mostrar estadísticas del índice construido
func runIndexBuild() {
	fmt.Printf("Building index from: %s\n", cfg.CSVPath)
	fmt.Printf("Workers: %d\n", cfg.Workers)

	// Verificar que el archivo CSV exista
	if _, err := os.Stat(cfg.CSVPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("CSV file not found: %s", cfg.CSVPath), nil)
	}

	// Construir el índice
	// BuildIndex lee el CSV y crea estructuras de datos optimizadas
	// para búsqueda rápida
	index, err := internal.BuildIndex(cfg.CSVPath, cfg.Workers)
	if err != nil {
		exitWithError("Error building index", err)
	}

	// Mostrar cuántos registros se indexaron
	fmt.Printf("Index built: %d records\n", index.TotalRecords)

	// Guardar el índice a disco para reutilización futura
	// El índice se guarda en formato JSON
	if err := index.Save(cfg.IndexPath); err != nil {
		exitWithError("Error saving index", err)
	}
	fmt.Printf("Index saved to: %s\n", cfg.IndexPath)
}

// runIndexSearch ejecuta la búsqueda usando el índice.
//
// Pasos:
// 1. Cargar el índice desde el archivo JSON
// 2. Recopilar las condiciones de búsqueda
// 3. Ejecutar la búsqueda en el índice
// 4. Mostrar los resultados
func runIndexSearch() {
	fmt.Printf("Loading index from: %s\n", cfg.IndexPath)

	// Verificar que el índice exista
	if _, err := os.Stat(cfg.IndexPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("Index file not found: %s", cfg.IndexPath), nil)
	}

	// Cargar el índice desde disco
	// El índice se mantiene en memoria para búsquedas rápidas
	index, err := internal.LoadIndex(cfg.IndexPath)
	if err != nil {
		exitWithError("Error loading index", err)
	}
	fmt.Printf("Index loaded: %d records\n", index.TotalRecords)

	// Recopilar las condiciones de búsqueda desde los flags
	conditions := collectConditions()

	// Ejecutar búsqueda en el índice
	// SearchAll retorna todos los registros que coinciden
	results := index.SearchAll(conditions, cfg.Logic)
	printResults(results)
}

// collectConditions recopila las condiciones de búsqueda desde la configuración.
//
// Itera sobre todos los campos de búsqueda posibles (DNI, nombres, apellidos)
// y crea una lista de SearchCondition solo para aquellos que fueron especificados.
//
// Por qué no buscar todos los campos siempre:
// - El usuario puede especificar solo los campos relevantes
// - Evita buscar por campos vacíos
// - Permite combinaciones flexibles (buscar solo por apellido, etc.)
//
// Si no se especifica un patrón, usa PatternExact por defecto.
func collectConditions() []SearchCondition {
	var conditions []SearchCondition

	// Procesar cada campo de búsqueda potencial
	// Solo agregamos condición si el usuario especificó un valor

	if cfg.DNI != "" {
		pattern := cfg.DNIPattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "dni", Value: cfg.DNI, Pattern: pattern})
	}

	if cfg.PrimerNombre != "" {
		pattern := cfg.PrimerNombrePattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "primer_nombre", Value: cfg.PrimerNombre, Pattern: pattern})
	}

	if cfg.SegundoNombre != "" {
		pattern := cfg.SegundoNombrePattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "segundo_nombre", Value: cfg.SegundoNombre, Pattern: pattern})
	}

	if cfg.PrimerApellido != "" {
		pattern := cfg.PrimerApellidoPattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "primer_apellido", Value: cfg.PrimerApellido, Pattern: pattern})
	}

	if cfg.SegundoApellido != "" {
		pattern := cfg.SegundoApellidoPattern
		if pattern == "" {
			pattern = PatternExact
		}
		conditions = append(conditions, SearchCondition{Field: "segundo_apellido", Value: cfg.SegundoApellido, Pattern: pattern})
	}

	return conditions
}
