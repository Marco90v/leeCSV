// Package cmd contiene los comandos de CLI de la aplicación.
//
// Este paquete utiliza Cobra para construir una CLI intuitiva con
// subcomandos para cada modo de búsqueda (CSV, Index, SQLite).
//
// Estructura de comandos:
//   - rootCmd: comando raíz con configuración global
//   - searchCmd: búsqueda directa en CSV
//   - indexCmd: operaciones con índice en memoria
//   - dbCmd: operaciones con base de datos SQLite
//
// Por qué usamos aliases de tipos:
//   - Re-exportamos tipos del paquete internal para mantener la API
//     cohesiva y evitar import.go/csv/internal repetitivo en cmd
//   - Permite cambiar la implementación interna sin afectar la CLI
//   - Facilita el testing: podemos mockear en cmd sin depender de internal
package cmd

import "go/csv/internal"

// Record es un alias para internal.Record para conveniencia en el paquete cmd.
//
// Definir este alias nos permite:
// - Usar Record directamente en funciones de cmd sin importar internal
// - Mantener la consistencia con la API del paquete internal
// - Facilitar refactorizaciones futuras
type Record = internal.Record

// SearchCondition es un alias para internal.SearchCondition.
//
// Representa una condición de búsqueda individual que se aplica
// a un campo específico de un registro.
type SearchCondition = internal.SearchCondition

// SearchPattern es un alias para internal.SearchPattern.
//
// Define el tipo de patrón de búsqueda a utilizar:
// - PatternExact: coincide exactamente con el valor
// - PatternContains: el valor contiene la búsqueda
// - PatternStartsWith: el valor comienza con la búsqueda
// - PatternRegex: usa expresión regular (más lento)
type SearchPattern = internal.SearchPattern

// SearchLogic es un alias para internal.SearchLogic.
//
// Define cómo se combinan múltiples condiciones de búsqueda:
// - LogicAND: todas las condiciones deben cumplirse
// - LogicOR: al menos una condición debe cumplirse
type SearchLogic = internal.SearchLogic

// Constantes de patrones de búsqueda.
//
// Estos valores corresponden a los definidos en internal y se
// re-exportan para uso convenientes en cmd.
// Por qué re-exportamos: evita que el usuario de la CLI necesite
// conocer los detalles del paquete internal.
var (
	PatternExact      = internal.PatternExact
	PatternContains   = internal.PatternContains
	PatternStartsWith = internal.PatternStartsWith
	PatternRegex      = internal.PatternRegex
)

// Constantes de lógica de búsqueda.
//
// Definen cómo se combinan múltiples condiciones.
// Por defecto usamos LogicAND (todas las condiciones deben cumplirse).
// Re-exportamos desde internal para mantener coherencia en la API.
var (
	LogicAND = internal.LogicAND
	LogicOR  = internal.LogicOR
)
