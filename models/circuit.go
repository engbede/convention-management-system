package models

import "strings"

type CircuitStat struct {
	Circuit string
	Count   int
}

// ApprovedCircuits defines all 12 official circuits in alphabetical order.
var ApprovedCircuits = []string{
	"Adoka",
	"Edikwu",
	"Entekpa",
	"Iga",
	"Oba",
	"Ojantelle",
	"Ojope",
	"Onaje",
	"Trinity",
	"Ugboju/Onyagede",
	"Ugbokpo",
	"Unmonomi",
}

// IsValidCircuit validates if a submitted circuit name is in the approved circuit list (case-insensitive).
func IsValidCircuit(circuit string) bool {
	cTrim := strings.TrimSpace(circuit)
	if cTrim == "" {
		return false
	}
	for _, a := range ApprovedCircuits {
		if strings.EqualFold(a, cTrim) {
			return true
		}
	}
	return false
}

// NormalizeCircuit returns the canonical casing of the approved circuit, or the trimmed original.
func NormalizeCircuit(circuit string) string {
	cTrim := strings.TrimSpace(circuit)
	for _, a := range ApprovedCircuits {
		if strings.EqualFold(a, cTrim) {
			return a
		}
	}
	return cTrim
}
