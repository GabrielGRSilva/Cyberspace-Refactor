package model

import "testing"

func TestExitsClockwiseSorting(t *testing.T) {
	m := NewMatrix()
	// Center at (5,5)
	m.Nodes[0] = &Node{ID: 0, Kind: CPU, Color: Green, Rating: 4, X: 5, Y: 5, Links: []int{1, 2, 3, 4}}
	// 1: North (5,2), 2: East (8,5), 3: South/6-o'clock (5,8), 4: West (2,5)
	m.Nodes[1] = &Node{ID: 1, Kind: SPU, Color: Blue, Rating: 2, X: 5, Y: 2}
	m.Nodes[2] = &Node{ID: 2, Kind: SPU, Color: Blue, Rating: 2, X: 8, Y: 5}
	m.Nodes[3] = &Node{ID: 3, Kind: SAN, Color: Blue, Rating: 2, X: 5, Y: 8}
	m.Nodes[4] = &Node{ID: 4, Kind: SPU, Color: Blue, Rating: 2, X: 2, Y: 5}

	ex := m.Exits(m.Nodes[0])
	// Clockwise starting at 6 o'clock (South=3 -> West=4 -> North=1 -> East=2).
	want := []int{3, 4, 1, 2}
	if len(ex) != len(want) {
		t.Fatalf("expected %v, got %v", want, ex)
	}
	for i := range want {
		if ex[i] != want[i] {
			t.Fatalf("exit %d: expected %v, got %v", i, want, ex)
		}
	}
}

func TestMatrixAndDeckDeepClone(t *testing.T) {
	m := NewMatrix()
	m.Nodes[0] = &Node{
		ID:    0,
		Kind:  DS,
		Color: Orange,
		Links: []int{1},
		Files: []File{{Name: "Secret", Size: 50, Value: 1000}},
		ICE:   []ICE{{Class: "Black", Type: "Killer", Rating: 5}},
	}
	cp := m.Clone()
	cp.Nodes[0].Files[0].Name = "Mutated"
	cp.Nodes[0].ICE[0].Rating = 99
	if m.Nodes[0].Files[0].Name != "Secret" || m.Nodes[0].ICE[0].Rating != 5 {
		t.Fatalf("Matrix.Clone did not deep-copy slices")
	}
}
