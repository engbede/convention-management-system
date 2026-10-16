package validation

import (
	"testing"

	"convention-management-system/models"
)

func TestApprovedCircuitsCountAndSorting(t *testing.T) {
	circuits := models.ApprovedCircuits
	if len(circuits) != 12 {
		t.Fatalf("expected exactly 12 approved circuits, got %d", len(circuits))
	}

	// Verify alphabetical ordering
	for i := 0; i < len(circuits)-1; i++ {
		if circuits[i] >= circuits[i+1] {
			t.Errorf("circuits not in strict alphabetical order: %s appears before %s", circuits[i], circuits[i+1])
		}
	}

	// Verify all expected circuits are present
	expected := []string{
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

	for i, name := range expected {
		if circuits[i] != name {
			t.Errorf("at index %d expected %q, got %q", i, name, circuits[i])
		}
	}
}

func TestCircuitValidation(t *testing.T) {
	validCases := []string{
		"Adoka",
		"adoka",
		"ADOKA",
		"  Adoka  ",
		"Edikwu",
		"Entekpa",
		"Iga",
		"Oba",
		"Ojantelle",
		"Ojope",
		"Onaje",
		"Trinity",
		"Ugbokpo",
		"Ugboju/Onyagede",
		"ugboju/onyagede",
		"Unmonomi",
	}

	for _, c := range validCases {
		if !models.IsValidCircuit(c) {
			t.Errorf("expected circuit %q to be valid", c)
		}
	}

	invalidCases := []string{
		"",
		"   ",
		"Lagos",
		"Abuja",
		"Makurdi",
		"Otukpo",
		"<script>alert(1)</script>",
		"Adoka Circuit",
		"Unknown Circuit",
	}

	for _, c := range invalidCases {
		if models.IsValidCircuit(c) {
			t.Errorf("expected circuit %q to be INVALID", c)
		}
	}
}

func TestCircuitNormalization(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"adoka", "Adoka"},
		{"ADOKA", "Adoka"},
		{"  edikwu  ", "Edikwu"},
		{"ugboju/onyagede", "Ugboju/Onyagede"},
		{"nonexistent", "nonexistent"},
	}

	for _, tc := range cases {
		normalized := models.NormalizeCircuit(tc.input)
		if normalized != tc.expected {
			t.Errorf("NormalizeCircuit(%q) = %q, expected %q", tc.input, normalized, tc.expected)
		}
	}
}
