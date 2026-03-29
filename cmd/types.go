package cmd

import "go/csv/internal"

// Record is an alias for internal.Record for convenience.
type Record = internal.Record

// SearchCondition is an alias for internal.SearchCondition.
type SearchCondition = internal.SearchCondition

// SearchPattern is an alias for internal.SearchPattern.
type SearchPattern = internal.SearchPattern

// SearchLogic is an alias for internal.SearchLogic.
type SearchLogic = internal.SearchLogic

// Pattern constants
var (
	PatternExact      = internal.PatternExact
	PatternContains   = internal.PatternContains
	PatternStartsWith = internal.PatternStartsWith
	PatternRegex      = internal.PatternRegex
)

// Logic constants
var (
	LogicAND = internal.LogicAND
	LogicOR  = internal.LogicOR
)
