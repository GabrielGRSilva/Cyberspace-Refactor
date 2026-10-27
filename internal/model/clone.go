package model

// Clone returns a deep copy of the matrix (runtime sessions mutate their own
// copy so the source file is never affected).
func (m *Matrix) Clone() *Matrix {
	c := *m
	c.Nodes = make(map[int]*Node, len(m.Nodes))
	for id, n := range m.Nodes {
		cn := *n
		cn.Links = append([]int(nil), n.Links...)
		cn.Files = append([]File(nil), n.Files...)
		cn.ICE = make([]ICE, len(n.ICE))
		for i, ice := range n.ICE {
			ice.Extra = append([]string(nil), ice.Extra...)
			cn.ICE[i] = ice
		}
		c.Nodes[id] = &cn
	}
	return &c
}

// Clone returns a deep copy of the deck.
func (d *Deck) Clone() *Deck {
	c := *d
	c.Utilities = append([]Utility(nil), d.Utilities...)
	c.Persona.Raw = nil
	return &c
}
