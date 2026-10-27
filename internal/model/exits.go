package model

import (
	"math"
	"sort"
)

// Exits returns the node's numbered exits: links excluding the external
// Matrix (node 0), sorted clockwise starting at 6 o'clock, as the original
// Net Navigator did. Nodes without coordinates keep file order.
func (m *Matrix) Exits(n *Node) []int {
	type ex struct {
		id    int
		angle float64
	}
	var list []ex
	for _, l := range n.Links {
		t, ok := m.Nodes[l]
		if !ok || l == 0 {
			continue
		}
		dx, dy := float64(t.X-n.X), float64(t.Y-n.Y)
		a := math.Atan2(-dx, dy)
		if a < 0 {
			a += 2 * math.Pi
		}
		list = append(list, ex{l, a})
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].angle < list[j].angle })
	out := make([]int, len(list))
	for i, e := range list {
		out[i] = e.id
	}
	return out
}

// LeadsToExternal reports whether the node has a link to the external Matrix.
func (n *Node) LeadsToExternal() bool { return n.Linked(0) || n.Kind == SAN && n.Linked(0) }
