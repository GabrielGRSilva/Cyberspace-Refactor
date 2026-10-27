// Package model holds the pure data types of the Matrix, cyberdecks and
// deckers. It has no I/O and no rules logic.
package model

import "sort"

// Hard limits inherited from the original program (see CYBER.HLP) and used as
// input-validation bounds by the parsers.
const (
	MaxNodes        = 100
	MaxLinksPerNode = 30
	MaxFilesPerDS   = 20
	MaxICEPerNode   = 101
	MaxICEPerMatrix = 101 * 4
	MaxUtilities    = 200
)

// Node kinds.
const (
	SAN = "SAN"
	SPU = "SPU"
	DS  = "DS"
	SM  = "SM"
	IOP = "IOP"
	CPU = "CPU"
	DLJ = "DLJ"
	LNK = "LNK"
	MAT = "MAT"
)

// ValidKind reports whether k is a known node kind.
func ValidKind(k string) bool {
	switch k {
	case SAN, SPU, DS, SM, IOP, CPU, DLJ, LNK, MAT:
		return true
	}
	return false
}

// Security colors, ordered by difficulty.
const (
	Blue   = "Blue"
	Green  = "Green"
	Orange = "Orange"
	Red    = "Red"
	Purple = "Purple"
	Dark   = "Dark"
)

// ColorFromLetter maps the first letter of a color name (the original only
// tested the first character) to its canonical name.
func ColorFromLetter(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	switch s[0] | 0x20 {
	case 'b':
		return Blue, true
	case 'g':
		return Green, true
	case 'o':
		return Orange, true
	case 'r':
		return Red, true
	case 'p':
		return Purple, true
	case 'd':
		return Dark, true
	}
	return "", false
}

// SuccessesRequired returns the base successes a decker needs on a system test
// in a node of the given color.
func SuccessesRequired(color string) int {
	switch color {
	case Blue:
		return 1
	case Green:
		return 2
	case Orange:
		return 3
	case Red:
		return 4
	case Purple:
		return 5
	case Dark:
		return 6
	}
	return 1
}

// ICE trigger and mobility values.
const (
	TrigDefault = ""
	TrigEver    = "ever"
	TrigPassive = "passive"
	TrigActive  = "active"
	TrigTrigger = "trigger"

	MobDefault  = ""
	MobImmobile = "immobile"
	MobMobile   = "mobile"
	MobPMobile  = "pmobile"
	MobAMobile  = "amobile"
)

// ICE describes one intrusion countermeasure installed in a node.
type ICE struct {
	Class    string // White, Gray, Black
	Type     string // Access, Barrier, Killer, Tar Baby, Trace and Burn, Probe...
	Stage    string // e.g. "M", "PhysM", "SE" (text inside the brackets)
	Rating   int
	Trigger  string   // see Trig*
	Mobility string   // see Mob*
	Extra    []string // unrecognised trailing tokens, preserved for round-trip
}

// File is a data file stored in a node.
type File struct {
	Name     string
	Size     int // megapulses
	Value    int // nuyen
	Password string
}

// Node is one element of a Local Telecommunications Grid.
type Node struct {
	ID        int
	Kind      string
	Color     string
	Rating    int
	X, Y      int
	Links     []int
	Message   string
	Files     []File
	ICE       []ICE
	LinkFile  string // LNK only: target .ltg
	LinkEntry int    // LNK only: entry node in target
}

// Matrix is a Local Telecommunications Grid (.ltg).
type Matrix struct {
	Description  string
	Entry        int
	Range        int    // 2 observation, 1 sensor, 0 contact
	Alert        string // none, passive, active
	ShutdownTics int
	ShutdownPct  int
	ImagePath    string
	ReadOnly     bool
	Nodes        map[int]*Node
}

// NewMatrix returns an empty matrix with sane defaults.
func NewMatrix() *Matrix {
	return &Matrix{Description: "New LTG", Entry: 1, Range: 2, Alert: "none", Nodes: map[int]*Node{}}
}

// IDs returns node IDs in ascending order.
func (m *Matrix) IDs() []int {
	ids := make([]int, 0, len(m.Nodes))
	for id := range m.Nodes {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// Linked reports whether node a links to b.
func (n *Node) Linked(b int) bool {
	for _, l := range n.Links {
		if l == b {
			return true
		}
	}
	return false
}

// Persona holds the cyberdeck persona programs and MPCP.
type Persona struct {
	Model   string
	MPCP    int
	Bod     int
	Evasion int
	Masking int
	Sensor  int
	Damage  int // MPCP damage boxes
	// Raw preserves the original "6+0-0-0" strings keyed by field.
	Raw map[string]string
}

// Utility is a program stored on a deck (or a downloaded data file).
type Utility struct {
	Name   string
	Rating int
	Size   int // megapulses
	Stage  string
	Target string
}

// Deck is a cyberdeck (.dek).
type Deck struct {
	Persona       Persona
	Hardening     int
	ActiveMem     int
	StorageMem    int
	LoadSpeed     int
	IOSpeed       int
	Response      int
	SatUplink     bool
	PureCyber     bool
	RealityFilter bool
	DisplayMode   int
	Utilities     []Utility
}

// Decker is a character (.npc).
type Decker struct {
	Name         string
	Body         int
	Quickness    int
	Strength     int
	Charisma     int
	Intelligence int
	Willpower    int
	Reaction     int
	ReactionDice int
	StunDamage   int
	PhysDamage   int
	Computer     int // best applicable computer skill (decking)
	Etiquette    int
	Karma        int
	HackingPool  int // 0 means "auto"
	TeamPool     int
}
