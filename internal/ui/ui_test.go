package ui

import (
	"strings"
	"testing"

	"cyberspace-refactor/internal/model"
)

func TestValidateMatrixAndCalcUtilSize(t *testing.T) {
	m := defaultMatrix()
	issues := validateMatrix(m)
	if len(issues) != 1 || !strings.HasPrefix(issues[0], "OK:") {
		t.Fatalf("expected defaultMatrix to validate cleanly, got: %v", issues)
	}

	// Blue node with Black ICE should flag a warning.
	m.Nodes[0].ICE = append(m.Nodes[0].ICE, model.ICE{Class: "Black", Type: "Killer", Rating: 4})
	issues = validateMatrix(m)
	found := false
	for _, s := range issues {
		if strings.Contains(s, "Black ICE") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected warning for Black ICE on Blue node, got: %v", issues)
	}

	if sz := calcUtilSize("Attack", "D", 4); sz != 5*16 {
		t.Fatalf("expected Deadly Attack-4 to be 80 Mp, got %d", sz)
	}
	if sz := calcUtilSize("Medic", "", 5); sz != 4*25 {
		t.Fatalf("expected Medic-5 to be 100 Mp, got %d", sz)
	}
}
