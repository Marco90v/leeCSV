package internal

// Record representa un registro individual del archivo CSV de identificación nacional.
//
// Este struct contiene los 7 campos del formato CSV venezolano:
// Nacionalidad;Cedula;Primer_Apellido;Segundo_Apellido;Primer_Nombre;Segundo_Nombre;Cod_Centro
//
// Uso típico:
//   - Se crea a partir de una línea del CSV usando parseRecord()
//   - Se utiliza para almacenar y mostrar resultados de búsqueda
//   - El campo DNI es el identificador único del registro
type Record struct {
	// Nacionalidad indica la nacionalidad de la persona (V: venezolano, E: extranjero, etc.)
	Nacionalidad string

	// DNI es el número de cédula de identidad - identificador único del registro
	DNI string

	// Primer_Apellido es el primer apellido de la persona
	Primer_Apellido string

	// Segundo_Apellido es el segundo apellido de la persona
	Segundo_Apellido string

	// Primer_Nombre es el primer nombre de la persona
	Primer_Nombre string

	// Segundo_Nombre es el segundo nombre de la persona (puede estar vacío)
	Segundo_Nombre string

	// Cod_Centro es el código del centro de votación/identificación
	Cod_Centro string
}

// SearchCondition representa una condición individual de búsqueda.
//
// Permite especificar:
//   - Qué campo buscar (Field)
//   - Qué valor buscar (Value)
//   - Cómo hacer la búsqueda (Pattern: exacta, contiene, etc.)
//
// Ejemplo de uso:
//
//	condition := SearchCondition{
//	    Field:   "dni",
//	    Value:   "12345678",
//	    Pattern: PatternExact,
//	}
type SearchCondition struct {
	// Field indica el nombre del campo donde buscar.
	// Valores válidos: "dni", "primer_nombre", "segundo_nombre", "primer_apellido", "segundo_apellido"
	Field string

	// Value es el valor que se buscara en el campo especificado.
	Value string

	// Pattern define el tipo de búsqueda a realizar.
	// Puede ser: exact (exacta), contains (contiene), startswith (comienza con)
	Pattern SearchPattern
}

// SearchPattern define cómo se debe realizar el matching (coincidencia) de un término de búsqueda.
//
// Los patrones disponibles son:
//   - Exact: coincidencia exacta del valor
//   - Contains: el valor contiene el término buscado
//   - StartsWith: el valor comienza con el término buscado
//   - Regex: coincidencia mediante expresión regular (para futuro)
type SearchPattern string

// Constantes que definen los tipos de patrón de búsqueda.
//
//nolint:stylecheck // Los nombres en minúscula son convención para constantes en Go
const (
	// PatternExact búsqueda exacta, el valor debe ser idéntico (case-insensitive)
	PatternExact SearchPattern = "exact"

	// PatternContains búsqueda parcial, el valor debe contener el término
	PatternContains SearchPattern = "contains"

	// PatternStartsWith búsqueda por prefijo, el valor debe comenzar con el término
	PatternStartsWith SearchPattern = "startswith"

	// PatternRegex búsqueda mediante expresión regular (no implementado actualmente)
	PatternRegex SearchPattern = "regex"
)

// SearchLogic define cómo se combinan múltiples condiciones de búsqueda.
//
// Uso:
//   - AND: todas las condiciones deben cumplirse (intersección)
//   - OR: al menos una condición debe cumplirse (unión)
type SearchLogic string

// Constantes para la lógica de combinación de búsquedas.
//
//nolint:stylecheck
const (
	// LogicAND requiere que todas las condiciones se cumplan
	LogicAND SearchLogic = "AND"

	// LogicOR requiere que al menos una condición se cumpla
	LogicOR SearchLogic = "OR"
)
