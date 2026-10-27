package main

import (
	"path/filepath"
	"testing"

	capp "cyberspace-refactor/internal/app"
)

func TestHeadlessSelfCheck(t *testing.T) {
	base := filepath.Clean("../..")
	ws, err := capp.Open(base)
	if err != nil {
		t.Skipf("workspace assets not available at %s: %v", base, err)
	}
	if err := runSelfCheck(ws); err != nil {
		t.Fatalf("runSelfCheck failed: %v", err)
	}
}
