package cmd

import (
	"context"
	"fmt"
	"os"

	"go/csv/internal"

	"github.com/spf13/cobra"
)

// searchCmd es el comando de búsqueda directa en CSV.
//
// Este comando implementa el modo de búsqueda más simple:
// lee el archivo CSV directamente y filtra los registros que
// coinciden con los criterios especificados.
//
// Por qué un modo CSV:
// - No requiere preparación previa (no hay que construir índice)
// - Funciona para archivos de cualquier tamaño
// - Ideal para búsquedas únicas o cuando no se tiene índice
//
// Limitaciones:
// - Más lento que Index o SQLite para archivos grandes
// - Lee el archivo completo en cada búsqueda
// - No es óptimo para búsquedas frecuentes
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Buscar registros en archivo CSV",
	Long: `Buscar registros usando lectura directa de CSV con procesamiento paralelo.
Este modo procesa el archivo en chunks usando múltiples workers.
NO carga todos los registros en memoria - ideal para archivos grandes (30M+ registros).`,
	Run: func(cmd *cobra.Command, args []string) {
		runSearch()
	},
}

// init() registra el comando search en la CLI.
//
// Agregamos searchCmd como subcomando de rootCmd y definimos
// los flags específicos para este comando.
//
// Flags definidos:
// - --dni: buscar por número de cédula
// - --primer-nombre: buscar por primer nombre
// - --segundo-nombre: buscar por segundo nombre
// - --primer-apellido: buscar por primer apellido
// - --segundo-apellido: buscar por segundo apellido
// - --logic: cómo combinar las condiciones (AND/OR)
func init() {
	// Agregar este comando al comando raíz
	rootCmd.AddCommand(searchCmd)

	// Flags específicos del comando search
	// Los flags se heredan del rootCmd (csv, workers) automáticamente
	searchCmd.Flags().StringVar(&cfg.DNI, "dni", "", "Buscar por DNI (coincidencia exacta)")
	searchCmd.Flags().StringVar(&cfg.PrimerNombre, "primer-nombre", "", "Buscar por primer nombre")
	searchCmd.Flags().StringVar(&cfg.SegundoNombre, "segundo-nombre", "", "Buscar por segundo nombre")
	searchCmd.Flags().StringVar(&cfg.PrimerApellido, "primer-apellido", "", "Buscar por primer apellido")
	searchCmd.Flags().StringVar(&cfg.SegundoApellido, "segundo-apellido", "", "Buscar por segundo apellido")
	searchCmd.Flags().StringVar((*string)(&cfg.Logic), "logic", "AND", "Lógica de búsqueda: AND, OR")
}

// runSearch ejecuta la búsqueda en el archivo CSV.
//
// Este es el punto de entrada real del comando search.
// Realiza las siguientes operaciones:
//
// 1. Obtiene los parámetros de búsqueda desde la configuración
// 2. Verifica que el archivo CSV exista
// 3. Llama a SearchCSVStreaming del paquete internal
// 4. Imprime estadísticas de la búsqueda
// 5. Muestra los resultados encontrados
//
// Por qué usamos SearchCSVStreaming:
// - Procesa el archivo en streaming (no carga todo en memoria)
// - Usa workers paralelos para acelerar el procesamiento
// - Soporta cancelación vía context
func runSearch() {
	// Obtener parámetros de configuración global
	params := getSearchParams()

	// Mensaje informativo para el usuario
	fmt.Printf("CSV mode (streaming) - searching in: %s\n", params.CSVPath)
	fmt.Printf("Workers: %d\n", params.Workers)

	// Verificar que el archivo existe antes de intentar abrirlo
	// os.Stat retorna error si el archivo no existe
	if _, err := os.Stat(params.CSVPath); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("CSV file not found: %s", params.CSVPath), nil)
	}

	// Crear contexto para soporte de cancelación
	// context.Background() es un contexto vacío pero nos permite
	// cancelarlo si el usuario presiona Ctrl+C
	ctx := context.Background()

	// Convertir SearchParams de cmd a SearchParams de internal
	// Esta separación mantiene las dependencias limpias
	internalParams := internal.SearchParams{
		DNI:             params.DNI,
		PrimerNombre:    params.PrimerNombre,
		SegundoNombre:   params.SegundoNombre,
		PrimerApellido:  params.PrimerApellido,
		SegundoApellido: params.SegundoApellido,
		Workers:         params.Workers,
	}

	// Ejecutar búsqueda en streaming
	// SearchCSVStreaming procesa el CSV sin cargar todo en memoria
	// Retorna: resultados, estadísticas de la búsqueda, y posible error
	results, stats, err := internal.SearchCSVStreaming(ctx, params.CSVPath, internalParams)
	if err != nil {
		exitWithError("Error searching CSV", err)
	}

	// Imprimir estadísticas de la búsqueda
	// Esto ayuda al usuario a entender el rendimiento
	fmt.Printf("\n--- Search Stats ---\n")
	fmt.Printf("Records processed: %d\n", stats.TotalProcessed)
	fmt.Printf("Matches found:    %d\n", stats.TotalMatches)
	fmt.Printf("Workers used:     %d\n", stats.WorkersUsed)
	fmt.Printf("--------------------\n")

	// Imprimir los resultados encontrados
	printResults(results)
}
