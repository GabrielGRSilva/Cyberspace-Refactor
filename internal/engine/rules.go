// Package engine implements the Shadowrun 2nd Edition / Virtual Realities 1.0
// Matrix rules as the original Cyberspace program did: ranges, ICE behaviour,
// utilities, system operations, alerts and damage. It is completely free of
// I/O and UI so every rule can be unit-tested with scripted dice.
package engine

import (
	"strings"

	"cyberspace-refactor/internal/model"
)

// Damage stages.
const (
	StageNone    = 0
	StageLight   = 1
	StageMod     = 2
	StageSerious = 3
	StageDeadly  = 4
)

// StageName returns a readable stage.
func StageName(s int) string {
	switch s {
	case StageLight:
		return "Light"
	case StageMod:
		return "Moderate"
	case StageSerious:
		return "Serious"
	case StageDeadly:
		return "Deadly"
	}
	return "None"
}

// StageFromLetter parses L/M/S/D.
func StageFromLetter(c byte, def int) int {
	switch c | 0x20 {
	case 'l':
		return StageLight
	case 'm':
		return StageMod
	case 's':
		return StageSerious
	case 'd':
		return StageDeadly
	}
	return def
}

// StageUp raises a stage by n levels (negative lowers), clamped to 0..4.
func StageUp(s, n int) int {
	s += n
	if s < StageNone {
		return StageNone
	}
	if s > StageDeadly {
		return StageDeadly
	}
	return s
}

// ICE damage boxes taken per stage.
var iceBoxes = map[int]int{StageLight: 1, StageMod: 3, StageSerious: 6, StageDeadly: 10}

// HackingPoolMode selects how the hacking pool is derived.
type HackingPoolMode int

const (
	// PoolOriginal reproduces the values documented by the original program's
	// sample deckers: 2 x Computer skill + 2 x Response Increase.
	PoolOriginal HackingPoolMode = iota
	// PoolSR2Core is the Shadowrun 2 core rule: (Intelligence + MPCP) / 3.
	PoolSR2Core
)

// HackingPool computes the pool for a decker on a deck.
func HackingPool(mode HackingPoolMode, d *model.Decker, deck *model.Deck) int {
	if d.HackingPool > 0 {
		return d.HackingPool
	}
	if mode == PoolSR2Core {
		return (d.Intelligence + deck.Persona.MPCP) / 3
	}
	return 2*d.Computer + 2*deck.Response
}

// Initiative returns the Matrix reaction and number of d6 for the decker.
func Initiative(d *model.Decker, deck *model.Deck, onSatellite bool) (reaction, dice int) {
	reaction = d.Reaction + deck.Response
	dice = 1 + deck.Response
	if deck.PureCyber {
		dice++
	}
	if deck.RealityFilter {
		reaction += 2
		dice++
	}
	if onSatellite {
		reaction -= 2
	}
	if reaction < 1 {
		reaction = 1
	}
	return
}

// Kind of utility behaviour.
type UtilKind int

// Utility kinds.
const (
	KUnknown UtilKind = iota
	KAnalyze
	KAttack
	KSlow
	KDeception
	KSleaze
	KBrowse
	KEvaluate
	KDecrypt
	KMedic
	KRestore
	KRelocate
	KDefense // Armor, Cloak, Shield, Mirrors, Smoke
	KSupport // Compressor, Auto Execute, Satlink, Hog...
)

// Degradation modes.
type Degrade int

// Degrade values.
const (
	DegNone Degrade = iota
	DegUse          // each execution (refreshable)
	DegTurn         // each turn while running (refreshable)
	DegTurnFixed    // each turn while running (not refreshable)
	DegBod          // whenever Bod is used (not refreshable)
	DegMask         // whenever Masking is used (not refreshable)
	DegStop         // each time it stops damage (refreshable)
)

// UtilInfo describes the behaviour of a named utility.
type UtilInfo struct {
	Kind    UtilKind
	Degrade Degrade
}

var utilCatalog = map[string]UtilInfo{
	"analyze": {KAnalyze, DegNone}, "attack": {KAttack, DegNone}, "slow": {KSlow, DegNone},
	"deception": {KDeception, DegUse}, "sleaze": {KSleaze, DegUse},
	"browse": {KBrowse, DegNone}, "sift": {KBrowse, DegNone},
	"evaluate": {KEvaluate, DegNone}, "decrypt": {KDecrypt, DegNone},
	"medic": {KMedic, DegNone}, "restore": {KRestore, DegNone}, "relocate": {KRelocate, DegNone},
	"armor": {KDefense, DegBod}, "cloak": {KDefense, DegMask}, "shield": {KDefense, DegStop},
	"mirrors": {KDefense, DegTurnFixed}, "smoke": {KDefense, DegTurn},
	"compressor": {KSupport, DegNone}, "auto execute": {KSupport, DegNone}, "satlink": {KSupport, DegNone},
	"hog": {KSupport, DegNone}, "lock-on": {KSupport, DegNone}, "lock on": {KSupport, DegNone},
}

// Lookup returns the catalog info for a utility name (case-insensitive).
func Lookup(name string) UtilInfo {
	return utilCatalog[strings.ToLower(strings.TrimSpace(name))]
}

// IsProgram reports whether the name is a known executable utility.
func IsProgram(name string) bool { return Lookup(name).Kind != KUnknown }

// ICE classification helpers.
func iceKey(t string) string { return strings.ToLower(strings.TrimSpace(t)) }

func isTrace(t string) bool { return strings.HasPrefix(iceKey(t), "trace") }

// effRating applies the +50% passive alert modifier (rounded down).
func effRating(r int, passive bool) int {
	if passive {
		return r * 3 / 2
	}
	return r
}
