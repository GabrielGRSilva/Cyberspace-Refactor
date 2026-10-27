package engine

import (
	"strconv"
	"strings"

	"cyberspace-refactor/internal/model"
)

// ICE behavioural states (they drive the sprites shown on the vidscreen).
type State int

// ICE states.
const (
	StIdle       State = iota // not aware of the decker
	StSleazed                 // decker sleazed it this turn
	StAsking                  // wants identification (ICLOOK)
	StDeceived                // decker deceived it, it ignores him (ICDECP)
	StSuspicious              // passive alert, will scan again (ICSUSP)
	StYelling                 // active alert, fights (ICYEL*)
	StCrashed
)

// ICEInst is a live ICE.
type ICEInst struct {
	ID          int
	Def         model.ICE
	Node, Home  int
	State       State
	Engaged     bool // actively hunting/at the decker regardless of range
	Triggered   bool // activated by sibling ICE
	Fled        bool // trace ICE retreating to the CPU
	Frozen      int  // turns frozen by Slow
	Damage      int  // boxes
	SleazedTurn int
	Known       struct{ Class, Type, Rating bool }
	traced      bool
}

// Crashed reports whether the ICE is destroyed.
func (i *ICEInst) Crashed() bool { return i.State == StCrashed }

func (i *ICEInst) isBlack() bool { return strings.EqualFold(i.Def.Class, "black") }
func (i *ICEInst) isWhite() bool { return strings.EqualFold(i.Def.Class, "white") }

type traceJob struct {
	ice  *ICEInst
	left int
}

// iceActive reports whether the ICE is currently armed.
func (s *Session) iceActive(i *ICEInst) bool {
	if i.Crashed() {
		return false
	}
	switch strings.ToLower(i.Def.Trigger) {
	case "ever":
		return true
	case "passive", "ptrigger":
		return s.Passive || s.ActiveAlert
	case "active", "atrigger":
		return s.ActiveAlert
	case "trigger", "triggered", "ctrigger", "strigger":
		return i.Triggered || s.ActiveAlert
	}
	if i.isWhite() {
		return true
	}
	return s.Passive || s.ActiveAlert
}

func (s *Session) iceMobile(i *ICEInst) bool {
	m := strings.ToLower(i.Def.Mobility)
	if m == "" {
		if i.isWhite() {
			m = "immobile"
		} else {
			m = "pmobile"
		}
	}
	switch m {
	case "mobile":
		return true
	case "pmobile":
		return s.Passive || s.ActiveAlert
	case "amobile":
		return s.ActiveAlert
	}
	return false
}

// Rating returns the ICE rating with alert modifiers.
func (s *Session) iceRating(i *ICEInst) int { return effRating(i.Def.Rating, s.Passive || s.ActiveAlert) }

func (s *Session) setPassive() {
	if !s.Passive {
		s.Passive = true
		s.logf("*Passive alert! ICE ratings rise by 50%%.")
	}
}

func (s *Session) setActive() {
	s.setPassive()
	if !s.ActiveAlert {
		s.ActiveAlert = true
		s.logf("*Active alert!")
	}
}

// CancelPassive clears a passive alert (CPU operation).
func (s *Session) cancelPassive() bool {
	if s.ActiveAlert {
		return false
	}
	s.Passive = false
	return true
}

func (s *Session) leaveNode(id int) {
	for _, i := range s.ICE {
		if i.Node == id && !i.Crashed() && !(s.iceMobile(i) && s.iceActive(i) && i.State != StDeceived) {
			i.State = StIdle
			i.Engaged = false
		}
	}
}

// nextHop returns the first step of a shortest path (BFS) from->to.
func (s *Session) nextHop(from, to int) (int, bool) {
	if from == to {
		return from, false
	}
	prev := map[int]int{from: -1}
	queue := []int{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		n := s.Matrix.Nodes[cur]
		if n == nil {
			continue
		}
		for _, l := range n.Links {
			if l == 0 {
				continue
			}
			if _, seen := prev[l]; seen || s.Matrix.Nodes[l] == nil {
				continue
			}
			if until, locked := s.Locked[l]; locked && until > s.Turn && s.Matrix.Nodes[l].Kind == model.SAN {
				continue
			}
			prev[l] = cur
			if l == to {
				step := l
				for prev[step] != from {
					step = prev[step]
				}
				return step, true
			}
			queue = append(queue, l)
		}
	}
	return from, false
}

func (s *Session) firstCPU() (int, bool) {
	for _, id := range s.Matrix.IDs() {
		if s.Matrix.Nodes[id].Kind == model.CPU {
			return id, true
		}
	}
	return 0, false
}

func (s *Session) hops(from, to int) int {
	n, cur := 0, from
	for cur != to && n < 200 {
		nx, ok := s.nextHop(cur, to)
		if !ok {
			break
		}
		cur = nx
		n++
	}
	return n
}

func (s *Session) updateTriggers() {
	for _, i := range s.ICE {
		if i.Crashed() || i.Triggered {
			continue
		}
		for _, o := range s.ICE {
			if o != i && o.Node == i.Node && (o.State == StSuspicious || o.State == StYelling || o.Crashed()) {
				i.Triggered = true
				break
			}
		}
	}
}

// endTurn advances one 3-second combat turn: running utilities, ICE phase,
// alert timers.
func (s *Session) endTurn() {
	if s.Over {
		return
	}
	s.upkeep()
	s.updateTriggers()
	for _, i := range append([]*ICEInst(nil), s.ICE...) {
		if s.Over {
			return
		}
		s.iceAct(i)
	}
	s.traceTurn()
	s.alertTimers()
	s.Turn++
}

func (s *Session) iceAct(i *ICEInst) {
	if i.Crashed() || !s.iceActive(i) {
		return
	}
	if i.Frozen > 0 {
		i.Frozen--
		return
	}
	// Movement.
	if s.iceMobile(i) && i.State != StDeceived {
		target := s.Node
		if i.Fled {
			if cpu, ok := s.firstCPU(); ok {
				target = cpu
			}
		}
		if i.Node != target {
			wasHere := i.Node == s.Node
			if nx, ok := s.nextHop(i.Node, target); ok {
				i.Node = nx
				i.Engaged = false
				if wasHere {
					s.logf("*ICE leaves the node and approaches.")
				}
				if nx == s.Node && !i.Fled {
					i.Engaged = true
					i.State = StIdle
					s.logf("*New ICE approaches this node.")
				}
			}
		}
	}
	if i.Node != s.Node || !(i.Engaged || s.Range == RangeContact) || i.Fled {
		return
	}
	switch iceKey(i.Def.Type) {
	case "barrier":
		return
	}
	switch i.State {
	case StIdle, StSleazed:
		if i.State == StSleazed && i.SleazedTurn == s.Turn {
			s.logf("*ICE does not see you.")
			return
		}
		if s.Legal {
			return
		}
		i.State = StAsking
		s.logf("*ICE wants your identification.")
	case StAsking:
		i.State = StSuspicious
		s.logf("*ICE is suspicious.")
		s.setPassive()
	case StSuspicious:
		i.State = StYelling
		s.logf("*ICE yells!")
		s.setActive()
	case StYelling:
		s.iceFight(i)
	}
}

func (s *Session) iceName(i *ICEInst) string {
	if i.Known.Type {
		n := i.Def.Class + " " + i.Def.Type
		if i.Known.Rating {
			n += "-" + itoa(i.Def.Rating)
		}
		return n
	}
	return "ICE"
}

func itoa(n int) string { return strconv.Itoa(n) }

// baseStage determines the base damage stage of an attacking ICE.
func baseStage(i *ICEInst, black bool) (stage int, phys bool) {
	st := strings.ToUpper(i.Def.Stage)
	phys = strings.HasPrefix(st, "PHYS")
	letter := byte(0)
	if len(st) > 0 {
		letter = st[len(st)-1]
	}
	def := StageNone
	if black {
		def = StageLight
	} else {
		switch r := i.Def.Rating; {
		case r >= 10:
			def = StageDeadly
		case r >= 7:
			def = StageSerious
		case r >= 4:
			def = StageMod
		default:
			def = StageLight
		}
	}
	return StageFromLetter(letter, def), phys
}

// defenseDice is the persona's resistance pool.
func (s *Session) armorRating() int {
	for _, l := range s.ActiveMem {
		if strings.EqualFold(l.Name, "armor") && l.Running {
			return l.Cur
		}
	}
	return 0
}

func (s *Session) runningRating(name string) int {
	for _, l := range s.ActiveMem {
		if strings.EqualFold(l.Name, name) && l.Running {
			return l.Cur
		}
	}
	return 0
}

func (s *Session) iceFight(i *ICEInst) {
	rating := s.iceRating(i)
	name := s.iceName(i)
	key := iceKey(i.Def.Type)
	if i.isBlack() {
		s.fightBlack(i, rating)
		return
	}
	switch key {
	case "access", "probe":
		s.setActive()
		return
	case "scramble":
		if n := s.Cur(); n.Kind == model.DS && len(n.Files) > 0 {
			s.logf("Scramble ICE deletes the first file.")
			n.Files = n.Files[1:]
		}
		return
	case "killer", "blaster", "acid", "binder", "jammer", "marker":
		s.fightGray(i, key, rating, name)
	case "tar baby", "tar pit":
		s.fightTar(i, key, rating, name)
	default:
		if isTrace(key) {
			s.startTrace(i, key, rating)
		}
	}
}

func (s *Session) iceHits(i *ICEInst, rating, tn int) int {
	res := s.Roller.Pool(rating)
	h := 0
	for _, v := range res {
		if v >= max2(tn, 2) {
			h++
		}
	}
	s.rolled("[ICE %s attack] %d dice vs TN %d: %v => %d", i.Def.Type, rating, tn, res, h)
	return h
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *Session) resist(dicePool, tn int) int {
	res := s.Roller.Pool(dicePool)
	h := 0
	for _, v := range res {
		if v >= max2(tn, 2) {
			h++
		}
	}
	s.rolled("[Resist] %d dice vs TN %d: %v => %d", dicePool, tn, res, h)
	return h
}

// stageResult computes net stage after attack hits and resistance.
func stageResult(base, hits, resist int) int {
	if hits <= 0 {
		return StageNone
	}
	st := StageUp(base, (hits-1)/2)
	return StageUp(st, -(resist / 2))
}

func (s *Session) personaTN() int {
	return s.Deck.Persona.Evasion + s.runningRating("smoke")/2 + s.runningRating("mirrors")/2
}

func (s *Session) fightGray(i *ICEInst, key string, rating int, name string) {
	hits := s.iceHits(i, rating, s.personaTN())
	if hits == 0 {
		s.logf("*%s attacks and misses.", name)
		return
	}
	base, _ := baseStage(i, false)
	def := s.Deck.Persona.Bod + s.armorRating() + s.Deck.Hardening
	res := s.resist(def, rating)
	if arm := s.findLoaded("armor"); arm != nil && arm.Running && arm.Cur > 0 {
		arm.Cur--
		s.logf("*Armor is bitten (now %d).", arm.Cur)
	}
	st := stageResult(base, hits, res)
	if st == StageNone {
		s.logf("*%s hits your persona but you shrug it off.", name)
		return
	}
	if sh := s.findLoaded("shield"); sh != nil && sh.Running && sh.Cur > 0 {
		sh.Cur--
		st = StageUp(st, -1)
		s.logf("*Shield absorbs some damage (now %d).", sh.Cur)
		if st == StageNone {
			return
		}
	}
	switch key {
	case "killer":
		s.logf("*%s hits for %s damage!", name, StageName(st))
		s.damageDeck(iceBoxes[st])
	case "blaster":
		s.logf("*%s blasts your MPCP for %s damage!", name, StageName(st))
		s.damageDeck(iceBoxes[st])
		if st >= StageMod {
			s.burnPersona()
		}
	case "acid":
		s.degradePersona("Bod", st)
	case "binder":
		s.degradePersona("Evasion", st)
	case "jammer":
		s.degradePersona("Masking", st)
	case "marker":
		s.degradePersona("Sensor", st)
	}
}

func (s *Session) degradePersona(prog string, st int) {
	p := &s.Deck.Persona
	var f *int
	switch prog {
	case "Bod":
		f = &p.Bod
	case "Evasion":
		f = &p.Evasion
	case "Masking":
		f = &p.Masking
	default:
		f = &p.Sensor
	}
	*f -= st
	if *f < 0 {
		*f = 0
	}
	s.logf("*Your %s program is damaged: now %d.", prog, *f)
}

func (s *Session) burnPersona() {
	progs := []string{"Bod", "Evasion", "Masking", "Sensor"}
	s.degradePersona(progs[s.Roller.Src.D6()%4], StageLight)
}

func (s *Session) fightBlack(i *ICEInst, rating int) {
	hits := s.iceHits(i, rating, s.personaTN())
	if hits == 0 {
		s.logf("*Black ICE strikes at you and misses.")
		return
	}
	base, phys := baseStage(i, true)
	var def int
	if phys {
		def = s.Decker.Body
	} else {
		def = s.Decker.Willpower
	}
	res := s.resist(def, rating)
	st := stageResult(base, hits, res)
	if st == StageNone {
		s.logf("*Black ICE biofeedback resisted.")
		return
	}
	if phys {
		s.logf("*Black ICE does %s physical damage!", StageName(st))
		s.PhysBoxes += iceBoxes[st]
	} else {
		s.logf("*Black ICE does %s stun damage!", StageName(st))
		s.StunBoxes += iceBoxes[st]
	}
	s.checkDecker()
}

func (s *Session) fightTar(i *ICEInst, key string, rating int, name string) {
	hits := s.iceHits(i, rating, s.personaTN())
	if hits == 0 || len(s.ActiveMem) == 0 {
		return
	}
	idx := s.Roller.Src.D6() % len(s.ActiveMem)
	p := s.ActiveMem[idx]
	s.ActiveMem = append(s.ActiveMem[:idx], s.ActiveMem[idx+1:]...)
	s.logf("*%s steals your %s program!", name, p.Name)
	if key == "tar pit" {
		for k, u := range s.Storage {
			if strings.EqualFold(u.Name, p.Name) {
				s.Storage = append(s.Storage[:k], s.Storage[k+1:]...)
				s.logf("*The copy in storage is destroyed too.")
				break
			}
		}
	}
}

func (s *Session) startTrace(i *ICEInst, key string, rating int) {
	if i.traced {
		return
	}
	i.traced = true
	s.logf("*An ICE-dog runs away from its owner and leaves the node.")
	i.Fled = true
	if s.OnSatellite {
		s.logf("*The satellite uplink scrambles the trace.")
		return
	}
	cpuNode := s.Node
	if cpu, ok := s.firstCPU(); ok {
		cpuNode = cpu
	}
	left := 2 + s.hops(i.Home, cpuNode)
	if left < 3 {
		left = 3
	}
	s.traces = append(s.traces, &traceJob{ice: i, left: left})
}

func (s *Session) traceTurn() {
	var live []*traceJob
	for _, t := range s.traces {
		t.left--
		if t.left > 0 {
			live = append(live, t)
			continue
		}
		key := iceKey(t.ice.Def.Type)
		s.Traced = true
		s.logf("*The trace is complete: your entry point is reported!")
		s.setActive()
		switch {
		case strings.HasSuffix(key, "dump"):
			s.StunBoxes += iceBoxes[StageMod]
			s.over("*Trace and Dump: you are dumped from the Matrix (dump shock).")
			s.checkDecker()
		case strings.HasSuffix(key, "burn"):
			s.damageDeck(iceBoxes[StageSerious])
			s.StunBoxes += iceBoxes[StageMod]
			s.over("*Trace and Burn: your deck is burned and you are dumped.")
			s.checkDecker()
		}
	}
	s.traces = live
}

func (s *Session) damageDeck(boxes int) {
	s.DeckBoxes += boxes
	if s.DeckBoxes >= 10 {
		s.DeckBoxes = 10
		s.over("*Your cyberdeck crashes! You are dumped from the Matrix.")
		s.StunBoxes += iceBoxes[StageLight]
	}
}

func (s *Session) checkDecker() {
	if s.PhysBoxes >= 10 {
		s.PhysBoxes = 10
		s.over("*You are killed by biofeedback.")
	} else if s.StunBoxes >= 10 {
		s.StunBoxes = 10
		s.over("*You are knocked unconscious and dumped from the Matrix.")
	}
}

func (s *Session) alertTimers() {
	if !s.ActiveAlert || s.Matrix.ShutdownTics <= 0 {
		return
	}
	if !s.ShutdownOn {
		crashed, total := 0, len(s.ICE)
		for _, i := range s.ICE {
			if i.Crashed() {
				crashed++
			}
		}
		pct := 0
		if total > 0 {
			pct = crashed * 100 / total
		}
		if pct >= s.Matrix.ShutdownPct {
			s.ShutdownOn = true
			s.ShutdownLeft = s.Matrix.ShutdownTics
			s.logf("*System shutdown in progress...")
		}
		return
	}
	s.ShutdownLeft--
	if s.ShutdownLeft <= 0 {
		s.StunBoxes += iceBoxes[StageLight]
		s.over("*The system shuts down: you are dumped.")
	}
}
