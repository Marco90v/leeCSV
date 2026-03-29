package main

import (
	"go/csv/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// Execute handles its own error printing
	}
}
