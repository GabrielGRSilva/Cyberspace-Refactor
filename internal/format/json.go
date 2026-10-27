package format

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"cyberspace-refactor/internal/model"
)

// matrixJSON is the on-disk JSON shape of a matrix (maps with int keys are
// not stable in encoding/json, so nodes are an ordered array).
type matrixJSON struct {
	Version      int           `json:"version"`
	Description  string        `json:"description"`
	Entry        int           `json:"entry"`
	Range        int           `json:"range"`
	Alert        string        `json:"alert"`
	ShutdownTics int           `json:"shutdownTics,omitempty"`
	ShutdownPct  int           `json:"emergency,omitempty"`
	ImagePath    string        `json:"imagePath,omitempty"`
	ReadOnly     bool          `json:"readOnly,omitempty"`
	Nodes        []*model.Node `json:"nodes"`
}

// MatrixToJSON converts a matrix to indented JSON.
func MatrixToJSON(m *model.Matrix) ([]byte, error) {
	j := matrixJSON{Version: 1, Description: m.Description, Entry: m.Entry, Range: m.Range,
		Alert: m.Alert, ShutdownTics: m.ShutdownTics, ShutdownPct: m.ShutdownPct,
		ImagePath: m.ImagePath, ReadOnly: m.ReadOnly}
	for _, id := range m.IDs() {
		j.Nodes = append(j.Nodes, m.Nodes[id])
	}
	return json.MarshalIndent(j, "", "  ")
}

// MatrixFromJSON parses and validates matrix JSON.
func MatrixFromJSON(data []byte) (*model.Matrix, error) {
	var j matrixJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, err
	}
	if j.Version != 1 {
		return nil, fmt.Errorf("unsupported matrix JSON version %d", j.Version)
	}
	if len(j.Nodes) > model.MaxNodes+1 {
		return nil, errors.New("too many nodes")
	}
	m := model.NewMatrix()
	m.Description, m.Entry, m.Range, m.Alert = j.Description, j.Entry, clamp(j.Range, 0, 2), j.Alert
	m.ShutdownTics, m.ShutdownPct, m.ImagePath, m.ReadOnly = j.ShutdownTics, j.ShutdownPct, j.ImagePath, j.ReadOnly
	for _, n := range j.Nodes {
		if n == nil || n.ID < 0 || n.ID > model.MaxNodes || !model.ValidKind(n.Kind) {
			return nil, errors.New("invalid node in JSON")
		}
		if len(n.Links) > model.MaxLinksPerNode || len(n.ICE) > model.MaxICEPerNode || len(n.Files) > model.MaxFilesPerDS {
			return nil, fmt.Errorf("node %d exceeds limits", n.ID)
		}
		if _, dup := m.Nodes[n.ID]; dup {
			return nil, fmt.Errorf("duplicate node %d", n.ID)
		}
		m.Nodes[n.ID] = n
	}
	FixLinks(m)
	return m, nil
}

// DeckToJSON converts a deck to indented JSON.
func DeckToJSON(d *model.Deck) ([]byte, error) { return json.MarshalIndent(d, "", "  ") }

// DeckFromJSON parses a deck from JSON.
func DeckFromJSON(data []byte) (*model.Deck, error) {
	var d model.Deck
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	if len(d.Utilities) > model.MaxUtilities {
		return nil, errors.New("too many utilities in JSON deck")
	}
	return &d, nil
}

// DeckerToJSON converts a decker to indented JSON.
func DeckerToJSON(d *model.Decker) ([]byte, error) { return json.MarshalIndent(d, "", "  ") }

// DeckerFromJSON parses a decker from JSON.
func DeckerFromJSON(data []byte) (*model.Decker, error) {
	var d model.Decker
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// SortedKeys is a small helper for deterministic output in tools.
func SortedKeys(m map[string]string) []string {
	k := make([]string, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	sort.Strings(k)
	return k
}
