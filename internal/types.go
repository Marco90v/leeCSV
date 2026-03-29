package internal

// Record represents a single CSV record from the national ID database.
// CSV format: Nacionalidad;Cedula;Primer_Apellido;Segundo_Apellido;Primer_Nombre;Segundo_Nombre;Cod_Centro
type Record struct {
	Nacionalidad     string
	DNI              string
	Primer_Apellido  string
	Segundo_Apellido string
	Primer_Nombre    string
	Segundo_Nombre   string
	Cod_Centro       string
}

// SearchCondition represents a single search condition.
type SearchCondition struct {
	Field   string
	Value   string
	Pattern SearchPattern
}

// SearchPattern defines how a search term should be matched.
type SearchPattern string

const (
	PatternExact      SearchPattern = "exact"
	PatternContains   SearchPattern = "contains"
	PatternStartsWith SearchPattern = "startswith"
	PatternRegex      SearchPattern = "regex"
)

// SearchLogic defines how multiple search conditions are combined.
type SearchLogic string

const (
	LogicAND SearchLogic = "AND"
	LogicOR  SearchLogic = "OR"
)
