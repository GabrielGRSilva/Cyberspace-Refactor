package engine

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"cyberspace-refactor/internal/model"
)

// Op is a system operation.
type Op string

// The 16 system operations.
const (
	OpLockout        Op = "Lockout SAN"
	OpBackDoor       Op = "Back Door"
	OpSeeSystem      Op = "See System"
	OpCancelAlert    Op = "Cancel Alert"
	OpChangeNode     Op = "Change Node"
	OpDisplayMap     Op = "Display Map"
	OpShutdown       Op = "Shutdown"
	OpInquiry        Op = "System Inquiry"
	OpDisplayMessage Op = "Display Message"
	OpDisconnect     Op = "Disconnect Port"
	OpControl        Op = "Control"
	OpSensor         Op = "Sensor Readout"
	OpErase          Op = "Erase File"
	OpEdit           Op = "Edit File"
	OpRead           Op = "Read File"
	OpTransfer       Op = "Transfer File"
)

// OpsFor lists the operations offered by a node kind.
func OpsFor(kind string) []Op {
	switch kind {
	case model.SAN:
		return []Op{OpLockout, OpBackDoor, OpSeeSystem}
	case model.CPU:
		return []Op{OpCancelAlert, OpChangeNode, OpDisplayMap, OpShutdown, OpInquiry}
	case model.IOP:
		return []Op{OpDisplayMessage, OpDisconnect}
	case model.SM:
		return []Op{OpControl, OpSensor}
	case model.DS:
		return []Op{OpErase, OpEdit, OpRead, OpTransfer}
	}
	return nil
}

func hasOp(kind string, op Op) bool {
	for _, o := range OpsFor(kind) {
		if o == op {
			return true
		}
	}
	return false
}

// blocked returns a reason if ICE currently prevent system operations.
func (s *Session) blocked() string {
	for _, i := range s.ICE {
		if i.Node != s.Node || i.Crashed() || !s.iceActive(i) {
			continue
		}
		if iceKey(i.Def.Type) == "barrier" {
			return "a Barrier ICE blocks the node"
		}
		switch i.State {
		case StDeceived:
		case StSleazed:
			if i.SleazedTurn != s.Turn {
				return "ICE is watching you"
			}
		default:
			return "ICE is watching you"
		}
	}
	return ""
}

// SysOp performs a system operation. arg is a file index, node id or
// speed-read factor depending on op; text is used by Edit File.
func (s *Session) SysOp(op Op, arg int, text string) error {
	if err := s.check(); err != nil {
		return err
	}
	n := s.Cur()
	if !hasOp(n.Kind, op) {
		return fmt.Errorf("%s is not available in a %s", op, n.Kind)
	}
	if s.Range != RangeContact {
		return errors.New("system operations need contact range")
	}
	if r := s.blocked(); r != "" {
		s.logf("%s not possible: %s.", op, r)
		return errors.New(r)
	}
	// Argument validation before spending dice.
	switch op {
	case OpErase, OpEdit, OpRead, OpTransfer:
		if arg < 0 || arg >= len(n.Files) {
			return errors.New("no such file")
		}
	case OpChangeNode:
		dn := s.Matrix.Nodes[arg]
		if dn == nil || arg == 0 || dn.Kind == model.MAT {
			return errors.New("invalid destination node")
		}
	}
	factor := 1
	if op == OpRead {
		factor = 1
		if text != "" {
			// text may carry the speed-read factor as "2".."5"
			if v := strings.TrimSpace(text); len(v) == 1 && v[0] >= '1' && v[0] <= '5' {
				factor = int(v[0] - '0')
			}
		}
	}
	key := fmt.Sprintf("%d/%s", s.Node, op)
	tn := n.Rating + s.WoundMod() + 2*s.Failed[key] + factor - 1
	need := model.SuccessesRequired(n.Color)
	dicen := s.Decker.Computer + s.poolDice()
	hits := s.test(string(op), dicen, tn, need)
	if hits < need {
		s.Failed[key]++
		s.logf("%s SysOp failed.", op)
		if hits == 0 && model.SuccessesRequired(n.Color) >= 3 {
			s.setPassive()
		}
		s.scrambleBite()
		s.endTurn()
		return nil
	}
	turns := 1
	switch op {
	case OpLockout:
		s.Locked[s.Node] = s.Turn + 5*hits + 5
		s.logf("SAN locked out for %d turns.", 5*hits+5)
	case OpBackDoor:
		s.logf("Back door passcode hacked for later access.")
	case OpSeeSystem:
		s.logf("%s", s.systemSummary())
		turns = 0
	case OpCancelAlert:
		if s.cancelPassive() {
			s.logf("Passive alert cancelled.")
		} else {
			s.logf("An active alert cannot be cancelled.")
		}
	case OpChangeNode:
		s.leaveNode(s.Node)
		s.teleport(arg)
		turns = 0
	case OpDisplayMap:
		for id, nd := range s.Matrix.Nodes {
			k := s.know(id)
			k.Type, k.Color, k.Rating = true, true, nd.Kind != model.LNK
		}
		s.logf("Display Map.")
	case OpShutdown:
		for _, i := range s.ICE {
			i.State = StCrashed
		}
		s.over("System shutdown: the grid goes dark and you are dumped.")
	case OpInquiry:
		crashed := false
		for _, i := range s.ICE {
			if i.Crashed() {
				crashed = true
			}
		}
		if crashed {
			s.logf("Crashed ICE found in the system!")
			s.setActive()
		} else {
			s.logf("System inquiry: no crashed ICE.")
		}
	case OpDisplayMessage:
		s.logf("Message displayed: %s", n.Message)
	case OpDisconnect:
		s.logf("Port successful disconnected.")
	case OpControl:
		s.logf("Device under control.")
	case OpSensor:
		s.logf("Sensor readout: %s", n.Message)
	case OpErase:
		f := n.Files[arg]
		n.Files = append(n.Files[:arg], n.Files[arg+1:]...)
		s.logf("File %q erased.", f.Name)
	case OpEdit:
		if strings.TrimSpace(text) != "" {
			name := strings.TrimSpace(text)
			if len(name) > 60 {
				name = name[:60]
			}
			n.Files[arg].Name = strings.ReplaceAll(name, "/", "-")
		}
		s.logf("File edited.")
	case OpRead:
		f := n.Files[arg]
		speed := s.Deck.IOSpeed * factor
		if s.findLoaded("browse") != nil || s.findLoaded("sift") != nil {
			speed *= 2
		}
		turns = ceilDiv(f.Size, speed*3)
		s.logf("Reading %s [%d Mp, %d nuyen] takes %d turn(s).", f.Name, f.Size, f.Value, turns)
		s.know(s.Node).Files, s.know(s.Node).Values = true, true
	case OpTransfer:
		f := n.Files[arg]
		if f.Password != "" && !s.know(s.Node).Decrypted {
			s.logf("%s is encrypted; use Decrypt first.", f.Name)
			s.endTurn()
			return errors.New("file is encrypted")
		}
		size := f.Size
		for _, i := range s.ICE {
			if i.Node == s.Node && !i.Crashed() && iceKey(i.Def.Type) == "scramble" {
				size += 10 * i.Def.Rating
				s.logf("The file is downloaded including Scramble ICE code.")
				break
			}
		}
		if s.StorageUsed()+size > s.Deck.StorageMem {
			s.logf("Not enough storage memory.")
			return errors.New("not enough storage memory")
		}
		turns = ceilDiv(size, s.Deck.IOSpeed)
		s.Storage = append(s.Storage, model.Utility{Name: f.Name, Size: size})
		s.logf("Transfer started... Transferring <%s>", f.Name)
	}
	s.spend(turns)
	return nil
}

func (s *Session) scrambleBite() {
	n := s.Cur()
	if n.Kind != model.DS || len(n.Files) == 0 {
		return
	}
	for _, i := range s.ICE {
		if i.Node == s.Node && !i.Crashed() && iceKey(i.Def.Type) == "scramble" && i.State != StDeceived {
			s.logf("Scramble ICE deletes the first file.")
			n.Files = n.Files[1:]
			return
		}
	}
}

func (s *Session) systemSummary() string {
	counts := map[string]int{}
	for _, nd := range s.Matrix.Nodes {
		counts[nd.Kind]++
	}
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	parts := make([]string, len(kinds))
	for i, k := range kinds {
		parts[i] = fmt.Sprintf("%d %s", counts[k], k)
	}
	return "System has: " + strings.Join(parts, ", ")
}
