package cmd

import (
	"context"
	"fmt"
	"os"

	"go/csv/internal"

	"github.com/spf13/cobra"
)

// dbCmd es el comando padre para operaciones con base de datos SQLite.
//
// Este comando agrupa dos subcomandos:
// - db build: construye una base de datos desde un CSV
// - db search: busca usando la base de datos SQLite
//
// El modo SQLite ofrece:
// - Búsquedas flexibles con soporte SQL
// - Índices para acelerar consultas frecuentes
// - Opcional: FTS5 para búsqueda full-text
// - Persistencia robusta de datos
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Operaciones de base de datos (construir, buscar)",
	Long:  `Construir y buscar usando base de datos SQLite.`,
}

func init() {
	// Agregar como subcomando del raíz
	rootCmd.AddCommand(dbCmd)
}

// dbBuildCmd construye una base de datos SQLite desde un archivo CSV.
//
// Este comando:
// 1. Crea una nueva base de datos SQLite
// 2. Importa todos los registros del CSV
// 3. Crea índices para acelerar búsquedas
// 4. Opcionalmente crea tabla FTS5 para búsqueda full-text
//
// Por qué usar SQLite:
// - Base de datos embebida, no requiere servidor
// - Índices SQL para búsquedas rápidas
// - FTS5 para búsqueda de texto completo
// - Portable: un solo archivo
var dbBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Construir base de datos SQLite desde CSV",
	Long: `Construir una base de datos SQLite desde un archivo CSV.
La base de datos soporta búsquedas rápidas usando índices y FTS5 opcional.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDBBuild()
	},
}

func init() {
	dbCmd.AddCommand(dbBuildCmd)

	// Flags específicos para construir base de datos
	dbBuildCmd.Flags().StringVar(&cfg.CSVPath, "csv", DefaultCSVPath, "Ruta al archivo CSV")
	dbBuildCmd.Flags().StringVar(&cfg.DBPath, "db", DefaultDBPath, "Ruta a la base de datos SQLite")
	dbBuildCmd.Flags().IntVarP(&cfg.Workers, "workers", "w", 0, "Número de workers (0=automático)")
}

// dbSearchCmd busca usando una base de datos SQLite existente.
//
// Este comando:
// 1. Abre la base de datos SQLite
// 2. Compila y ejecuta consultas SQL
// 3. Retorna los registros que coinciden
//
// Ventajas del modo SQLite:
// - SQL completo para consultas complejas
// - Índices optimizados automáticamente
// - FTS5 para búsqueda de texto completo
// -Transacciones para integridad de datos
var dbSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Buscar usando base de datos SQLite",
	Long:  `Buscar registros usando una base de datos SQLite.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDBSearch()
	},
}

func init() {
	dbCmd.AddCommand(dbSearchCmd)

	// Flags para búsqueda en SQLite
	// Nota: SQLite soporta más opciones de patrón que Index
	dbSearchCmd.Flags().StringVar(&cfg.DBPath, "db", DefaultDBPath, "Ruta a la base de datos SQLite")
	dbSearchCmd.Flags().StringVar(&cfg.DNI, "dni", "", "Buscar por DNI")
	// Patrones específicos para cada campo en SQLite
	dbSearchCmd.Flags().StringVar((*string)(&cfg.DNIPattern), "dni-pattern", "exact", "Patrón DNI: exact, contains, startswith")
	dbSearchCmd.Flags().StringVar(&cfg.PrimerNombre, "primer-nombre", "", "Buscar por primer nombre")
	dbSearchCmd.Flags().StringVar((*string)(&cfg.PrimerNombrePattern), "primer-nombre-pattern", "exact", "Patrón primer nombre: exact, contains, startswith")
	dbSearchCmd.Flags().StringVar(&cfg.SegundoNombre, "segundo-nombre", "", "Buscar por segundo nombre")
	dbSearchCmd.Flags().StringVar(&cfg.PrimerApellido, "primer-apellido", "", "Buscar por primer apellido")
	dbSearchCmd.Flags().StringVar(&cfg.SegundoApellido, "segundo-apellido", "", "Buscar por segundo apellido")
	dbSearchCmd.Flags().StringVar((*string)(&cfg.Logic), "logic", "AND", "Lógica de búsqueda: AND, OR")
}

// runDBBuild ejecuta la construcción de la base de datos.
//
// Pasos:
// 1. Verificar que el archivo CSV exista
// 2. Crear el manager de SQLite
// 3. Importar el CSV a la base de datos
// 4. Mostrar estadísticas de la importación
//
// Nota: Usamos defer db.Close() para asegurar que la conexión
// se cierre incluso si ocurre un error.
func runDBBuild() {
	fmt.Printf("Building SQLite database from: %s\n", cfg.CSVPath)
	fmt.Printf("Workers: %d\n", cfg.Workers)

	// Verificar que el archivo CSV exista
	if _, err := os.Stat(cfg.CSVPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("CSV file not found: %s", cfg.CSVPath), nil)
	}

	// Crear manager de SQLite
	// NewSQLiteManager crea/abre la base de datos y inicializa el schema
	db, err := internal.NewSQLiteManager(cfg.DBPath)
	if err != nil {
		exitWithError("Error creating database", err)
	}
	// defer asegura que cerremos la conexión al terminar
	defer db.Close()

	// Importar CSV a la base de datos
	// BuildDBFromCSV lee el CSV y lo importa usando workers paralelos
	ctx := context.Background()
	if err := db.BuildDBFromCSV(ctx, cfg.CSVPath, cfg.Workers); err != nil {
		exitWithError("Error building database", err)
	}

	// Mostrar estadísticas
	count, _ := db.GetRecordCount()
	fmt.Printf("Database built: %d records\n", count)
	fmt.Printf("Database saved to: %s\n", cfg.DBPath)
}

// runDBSearch ejecuta la búsqueda en la base de datos SQLite.
//
// Pasos:
// 1. Verificar que la base de datos exista
// 2. Abrir la base de datos SQLite
// 3. Recopilar las condiciones de búsqueda
// 4. Ejecutar la búsqueda usando SQL
// 5. Mostrar los resultados
func runDBSearch() {
	fmt.Printf("Using database: %s\n", cfg.DBPath)

	// Verificar que la base de datos exista
	if _, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("Database file not found: %s", cfg.DBPath), nil)
	}

	// Abrir la base de datos existente
	db, err := internal.NewSQLiteManager(cfg.DBPath)
	if err != nil {
		exitWithError("Error opening database", err)
	}
	defer db.Close()

	// Recopilar condiciones de búsqueda desde los flags
	conditions := collectConditions()

	// Ejecutar búsqueda
	// SearchAll genera y ejecuta consultas SQL basadas en las condiciones
	results, err := db.SearchAll(conditions, cfg.Logic)
	if err != nil {
		exitWithError("Search error", err)
	}
	printResults(results)
}
