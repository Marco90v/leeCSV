// Package main es el punto de entrada de la aplicación CLI leeCSV.
//
// La función main es mínima por diseño: toda la lógica de comandos
// está encapsulada en el paquete cmd. Esto sigue el principio de
// separación de responsabilidades y facilita el testing.
//
// Por qué es importante:
// - Mantiene el código de entrada simple y limpio
// - Permite que cmd.Execute() maneje errores uniformemente
// - Facilita la reutilización del paquete cmd en otros contextos
package main

import (
	"go/csv/cmd"
)

// main es el punto de entrada de la aplicación.
//
// Delegamos toda la ejecución a cmd.Execute() que:
// 1. Configura los comandos de Cobra
// 2. Parsea los argumentos de línea de comandos
// 3. Ejecuta el comando apropiado
// 4. Maneja errores de forma consistente
func main() {
	// Execute() ya maneja la impresión de errores internamente,
	// por lo que no necesitamos hacer nada más aquí.
	if err := cmd.Execute(); err != nil {
		// El manejo de errores ya está hecho en cmd.Execute()
		// Esta línea existe para complacer al compilador y
		// hacer explícito que ignoramos el error aquí.
	}
}
