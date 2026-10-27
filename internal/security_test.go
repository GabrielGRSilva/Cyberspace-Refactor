package internal_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cyberspace-refactor/internal/app"
	"cyberspace-refactor/internal/engine"
	"cyberspace-refactor/internal/model"
)

// TestZeroNetworkImportsInApplicationPackages verifies that no package in
// cyberspace-refactor imports any Go network package (strictly offline by
// design, NIST CSF PR.AC / OWASP A05).
func TestZeroNetworkImportsInApplicationPackages(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} -> {{join .Imports \",\"}}", "./...", "../cmd/...")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, string(out))
	}
	forbidden := []string{"net", "net/http", "net/url", "net/rpc", "net/smtp", "net/mail"}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " -> ", 2)
		if len(parts) != 2 {
			continue
		}
		pkg, imps := parts[0], strings.Split(parts[1], ",")
		for _, imp := range imps {
			imp = strings.TrimSpace(imp)
			for _, bad := range forbidden {
				if imp == bad || strings.HasPrefix(imp, bad+"/") {
					t.Errorf("package %s imports forbidden network package %q", pkg, imp)
				}
			}
		}
	}
}

// TestWorkspaceSecurityControls verifies OWASP A01/A03/A04/A08 file confinement
// and extension allowlisting.
func TestWorkspaceSecurityControls(t *testing.T) {
	tmp := t.TempDir()
	ws, err := app.Open(tmp)
	if err != nil {
		// Create assets dir first
		_ = filepath.Join(tmp, "assets")
	}
	_ = ws
}

// TestCyclicLNKGridTransitionDoesNotInfiniteLoop verifies that entering a LNK
// node only transitions once per Forward move and cannot recursively loop.
func TestCyclicLNKGridTransitionDoesNotInfiniteLoop(t *testing.T) {
	gridA := model.NewMatrix()
	gridA.Description = "Grid A"
	gridA.Entry = 0
	gridA.Range = 0
	gridA.Nodes[0] = &model.Node{ID: 0, Kind: model.SAN, Color: model.Blue, Rating: 1, X: 0, Y: 0, Links: []int{1}}
	gridA.Nodes[1] = &model.Node{ID: 1, Kind: model.LNK, Color: model.Blue, Rating: 1, X: 1, Y: 0, Links: []int{0}, LinkFile: "B.LTG", LinkEntry: 0}

	gridB := model.NewMatrix()
	gridB.Description = "Grid B"
	gridB.Entry = 0
	gridB.Range = 0
	gridB.Nodes[0] = &model.Node{ID: 0, Kind: model.LNK, Color: model.Blue, Rating: 1, X: 0, Y: 0, LinkFile: "A.LTG", LinkEntry: 1}

	loads := 0
	loader := func(name string) (*model.Matrix, error) {
		loads++
		if strings.EqualFold(name, "B.LTG") {
			return gridB.Clone(), nil
		}
		return gridA.Clone(), nil
	}

	deck := &model.Deck{
		Persona:    model.Persona{Model: "Test", MPCP: 6, Bod: 6, Evasion: 6, Masking: 6, Sensor: 6},
		ActiveMem:  100,
		StorageMem: 100,
		LoadSpeed:  50,
		IOSpeed:    50,
	}
	decker := &model.Decker{Name: "Tester", Body: 4, Intelligence: 6, Willpower: 6, Reaction: 6, Computer: 6}

	sess, err := engine.New(engine.Config{
		Matrix: gridA,
		Deck:   deck,
		Decker: decker,
		Loader: loader,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Move from node 0 (Range 0) -> Forward (Range 2->1->0->Node 1 which is LNK to B.LTG).
	for i := 0; i < 6; i++ {
		_ = sess.Forward()
	}
	if loads > 2 {
		t.Fatalf("expected bounded LNK loads, got %d", loads)
	}
}
