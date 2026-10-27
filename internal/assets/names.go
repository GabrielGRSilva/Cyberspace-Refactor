// Package assets resolves and loads the original Cyberspace images (.bmp) and
// sounds (.wav) using their legacy naming conventions (see CYBER.HLP).
package assets

import (
	"strings"

	"cyberspace-refactor/internal/engine"
	"cyberspace-refactor/internal/model"
)

// iceSuffix maps an ICE type to the hexadecimal character used by the
// "advanced"/"sculpted" display modes.
var iceSuffix = map[string]string{
	"access": "0", "barrier": "1", "scramble": "2", "probe": "3", "killer": "4", "blaster": "5",
	"acid": "6", "binder": "7", "jammer": "8", "marker": "9", "tar baby": "A", "tar pit": "B",
	"trace and report": "C", "trace and dump": "D", "trace and burn": "E",
}

// ICESuffix returns the type suffix ("" if unknown).
func ICESuffix(iceType string) string { return iceSuffix[strings.ToLower(strings.TrimSpace(iceType))] }

// NodeSprite returns candidate file names (without extension) for a node image
// at a range; the first existing wins.
func NodeSprite(kind string, rng int, satellite bool) []string {
	base := strings.ToUpper(kind)
	if base == model.LNK {
		base = model.SAN
	}
	r := string(rune('0' + rng))
	out := []string{}
	if satellite {
		out = append(out, base+r+"S")
	}
	out = append(out, base+r)
	if base == model.MAT {
		out = []string{"MAT"}
	}
	return out
}

// ICESprite returns candidate file names for an ICE in a state. mode is the
// deck DisplayMode (0 standard, 1 advanced, 2 sculpted). analysed says
// whether the decker has analysed this ICE's type.
func ICESprite(i *engine.ICEInst, mode int, analysed bool, hit bool) []string {
	var prefix string
	switch {
	case i.Crashed():
		return []string{"CRASH"}
	case hit:
		prefix = "ICHIT"
	case i.Frozen > 0:
		prefix = "ICFROZ"
	case i.State == engine.StDeceived:
		prefix = "ICDECP"
	case i.State == engine.StSleazed:
		prefix = "ICSLZA"
	case i.State == engine.StSuspicious:
		prefix = "ICSUSP"
	case i.State == engine.StYelling:
		switch strings.ToLower(i.Def.Class) {
		case "black":
			prefix = "ICYELB"
		case "gray":
			prefix = "ICYELG"
		default:
			prefix = "ICYELW"
		}
	default:
		prefix = "ICLOOK"
	}
	suf := ICESuffix(i.Def.Type)
	useSuffix := suf != "" && (mode == 2 || (mode == 1 && analysed))
	if useSuffix {
		return []string{prefix + suf, prefix}
	}
	return []string{prefix}
}

// Sound events and their legacy file names.
const (
	SndBoot      = "SNDBOOT"
	SndPlug      = "SNDPLUG"
	SndChooseLTG = "SNDCLTG"
	SndLook      = "SNDLOOK"
	SndSuspicion = "SNDSUSP"
	SndYell      = "SNDYELL"
	SndSleaze    = "SNDSLZA"
	SndDeceive   = "SNDDECP"
	SndShootGray = "SNDSHTG"
	SndHitGray   = "SNDHITG"
	SndHitDecker = "SNDHITD"
	SndCrash     = "SNDCRAS"
	SndFreeze    = "SNDFREZ"
	SndFrozen    = "SNDFROZ"
	SndTar       = "SNDTARB"
	SndBark      = "SNDBARK"
	SndTrace     = "SNDTRAC"
	SndActive    = "SNDACTA"
	SndPassive   = "SNDPASA"
	SndArmor     = "SNDARMG"
)

// SysOpSound maps a system operation to its sound file.
func SysOpSound(op engine.Op) string {
	order := map[engine.Op]string{
		engine.OpLockout: "0", engine.OpBackDoor: "1", engine.OpSeeSystem: "2", engine.OpCancelAlert: "3",
		engine.OpChangeNode: "4", engine.OpDisplayMap: "5", engine.OpShutdown: "6", engine.OpInquiry: "7",
		engine.OpEdit: "8", engine.OpErase: "9", engine.OpRead: "A", engine.OpTransfer: "B",
		engine.OpControl: "C", engine.OpSensor: "D", engine.OpDisplayMessage: "E", engine.OpDisconnect: "F",
	}
	if c, ok := order[op]; ok {
		return "SNDSOP" + c
	}
	return ""
}

// SoundForLog picks a sound for a history-log line ("" for none).
func SoundForLog(line string) string {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "wants your identification"):
		return SndLook
	case strings.Contains(l, "is suspicious"):
		return SndSuspicion
	case strings.Contains(l, "yells"):
		return SndYell
	case strings.Contains(l, "active alert"):
		return SndActive
	case strings.Contains(l, "passive alert"):
		return SndPassive
	case strings.Contains(l, "sleaze successful"):
		return SndSleaze
	case strings.Contains(l, "deception successful"):
		return SndDeceive
	case strings.Contains(l, "crashes") || strings.Contains(l, "crashed"):
		return SndCrash
	case strings.Contains(l, "frozen"):
		return SndFrozen
	case strings.Contains(l, "steals"):
		return SndTar
	case strings.Contains(l, "ice-dog"):
		return SndBark
	case strings.Contains(l, "trace is complete"):
		return SndTrace
	case strings.Contains(l, "hits for") || strings.Contains(l, "blasts") || strings.Contains(l, "damage!"):
		return SndHitGray
	case strings.Contains(l, "your attack hits"):
		return SndHitDecker
	case strings.Contains(l, "armor builds up"):
		return SndArmor
	}
	return ""
}
