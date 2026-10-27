package engine

import (
	"errors"
	"fmt"
	"strings"
)

func ceilDiv(a, b int) int {
	if b <= 0 {
		b = 1
	}
	return (a + b - 1) / b
}

func (s *Session) findLoaded(name string) *Loaded {
	for _, l := range s.ActiveMem {
		if strings.EqualFold(l.Name, name) {
			return l
		}
	}
	return nil
}

// ActiveUsed returns megapulses used in active memory.
func (s *Session) ActiveUsed() int {
	n := 0
	for _, l := range s.ActiveMem {
		n += l.Size
	}
	return n
}

// StorageUsed returns megapulses used in storage memory.
func (s *Session) StorageUsed() int {
	n := 0
	for _, u := range s.Storage {
		n += u.Size
	}
	return n
}

func refreshable(d Degrade) bool { return d == DegUse || d == DegTurn || d == DegStop }

// Load moves a program from storage into active memory.
func (s *Session) Load(idx int) error {
	if err := s.check(); err != nil {
		return err
	}
	if idx < 0 || idx >= len(s.Storage) {
		return errors.New("no such program")
	}
	u := s.Storage[idx]
	info := Lookup(u.Name)
	if info.Kind == KUnknown || u.Rating == 0 {
		return fmt.Errorf("%s is a data file, not a program", u.Name)
	}
	if ex := s.findLoaded(u.Name); ex != nil {
		if info.Degrade != DegNone && refreshable(info.Degrade) {
			ex.Cur = u.Rating
			s.logf("%s refreshed.", u.Name)
			s.spend(ceilDiv(u.Size, s.Deck.LoadSpeed))
			return nil
		}
		return fmt.Errorf("%s is already loaded", u.Name)
	}
	if s.ActiveUsed()+u.Size > s.Deck.ActiveMem {
		s.logf("Not enough active memory!")
		return errors.New("not enough active memory")
	}
	s.ActiveMem = append(s.ActiveMem, &Loaded{Utility: u, Cur: u.Rating})
	if info.Degrade == DegNone || !refreshable(info.Degrade) && info.Degrade != DegUse {
		// Non-degradable (and non-refreshable degradable) programs leave storage.
		s.Storage = append(s.Storage[:idx], s.Storage[idx+1:]...)
	}
	turns := ceilDiv(u.Size, s.Deck.LoadSpeed)
	s.logf("Loading takes %d turns", turns)
	s.spend(turns)
	return nil
}

// Unload moves a program back to storage.
func (s *Session) Unload(idx int) error {
	if err := s.check(); err != nil {
		return err
	}
	if idx < 0 || idx >= len(s.ActiveMem) {
		return errors.New("no such program")
	}
	l := s.ActiveMem[idx]
	s.ActiveMem = append(s.ActiveMem[:idx], s.ActiveMem[idx+1:]...)
	have := false
	for _, u := range s.Storage {
		if strings.EqualFold(u.Name, l.Name) {
			have = true
		}
	}
	if !have {
		s.Storage = append(s.Storage, l.Utility)
	}
	s.logf("%s unloaded.", l.Name)
	s.endTurn()
	return nil
}

// DeleteActive erases a program from active memory.
func (s *Session) DeleteActive(idx int) error {
	if err := s.check(); err != nil {
		return err
	}
	if idx < 0 || idx >= len(s.ActiveMem) {
		return errors.New("no such program")
	}
	s.logf("%s deleted from active memory.", s.ActiveMem[idx].Name)
	s.ActiveMem = append(s.ActiveMem[:idx], s.ActiveMem[idx+1:]...)
	return nil
}

// DeleteStorage erases an entry from storage memory.
func (s *Session) DeleteStorage(idx int) error {
	if err := s.check(); err != nil {
		return err
	}
	if idx < 0 || idx >= len(s.Storage) {
		return errors.New("no such entry")
	}
	s.logf("%s deleted from storage.", s.Storage[idx].Name)
	s.Storage = append(s.Storage[:idx], s.Storage[idx+1:]...)
	return nil
}

// iceInNode returns a live ICE by id in the current node, or the first one.
func (s *Session) iceInNode(id int) *ICEInst {
	for _, i := range s.ICE {
		if i.Node == s.Node && !i.Crashed() && (id == 0 || i.ID == id) {
			return i
		}
	}
	return nil
}

// ICEHere lists the live ICE in the current node that the decker can see.
func (s *Session) ICEHere() []*ICEInst {
	var out []*ICEInst
	if s.Range > RangeSensor {
		return out
	}
	for _, i := range s.ICE {
		if i.Node == s.Node && !i.Crashed() && s.iceActive(i) {
			out = append(out, i)
		}
	}
	return out
}

func (s *Session) detectTN() int { return s.Deck.Persona.Masking + s.runningRating("cloak") }

func (s *Session) iceDetects(i *ICEInst) int {
	h := s.iceHits(i, s.iceRating(i), s.detectTN())
	if c := s.findLoaded("cloak"); c != nil && c.Running && c.Cur > 0 {
		c.Cur--
	}
	return h
}

func (s *Session) deceivable(i *ICEInst) bool { return iceKey(i.Def.Type) != "barrier" }

// Execute runs the active-memory program at idx. target is an ICE id (0 = default).
func (s *Session) Execute(idx, target int) error {
	if err := s.check(); err != nil {
		return err
	}
	if idx < 0 || idx >= len(s.ActiveMem) {
		return errors.New("no such program")
	}
	l := s.ActiveMem[idx]
	info := Lookup(l.Name)
	if l.Cur <= 0 {
		s.logf("%s has degraded to rating 0 and is useless.", l.Name)
		return errors.New("program rating is 0")
	}
	dicen := l.Cur + s.poolDice()
	mod := s.WoundMod()
	n := s.Cur()
	switch info.Kind {
	case KAnalyze:
		if s.Range > RangeSensor {
			return errors.New("move closer (sensor range) to analyze")
		}
		if target > 0 {
			i := s.iceInNode(target)
			if i == nil {
				return errors.New("no such ICE here")
			}
			h := s.test("Analyze ICE", dicen, s.iceRating(i)+mod, 1)
			if h == 0 {
				s.logf("Execution of Analyze failed.")
			} else {
				i.Known.Class, i.Known.Type = true, true
				if h >= 2 {
					i.Known.Rating = true
				}
				s.logf("You see a %s", s.iceName(i))
			}
		} else {
			h := s.test("Analyze node", dicen, n.Rating+mod, 1)
			k := s.know(s.Node)
			if h == 0 {
				s.logf("Execution of Analyze failed.")
			} else {
				k.Type, k.Color = true, true
				if h >= 2 {
					k.Rating = true
				}
				s.logf("You see a %s", s.NodeLabel(s.Node)[strings.Index(s.NodeLabel(s.Node), " ")+1:])
			}
		}
	case KAttack:
		s.doAttack(l, target, dicen)
	case KSlow:
		i := s.iceInNode(target)
		if i == nil || s.Range > RangeSensor {
			return errors.New("no ICE in range")
		}
		if iceKey(i.Def.Type) == "scramble" {
			s.logf("Slow is useless against this ICE-type.")
			break
		}
		if h := s.test("Slow", dicen, s.iceRating(i)+mod, 1); h > 0 {
			i.Frozen += clampInt(h, 1, 6)
			s.logf("ICE is frozen for %d turns.", clampInt(h, 1, 6))
		} else {
			s.logf("Slow failed.")
		}
	case KDeception:
		s.doDeception(l, dicen, mod)
	case KSleaze:
		s.doSleaze(l, dicen, mod)
	case KBrowse, KEvaluate, KDecrypt:
		if err := s.doDataUtil(info.Kind, l, dicen, mod); err != nil {
			return err
		}
	case KMedic, KDefense, KSupport:
		l.Running = !l.Running
		if l.Running {
			s.logf("%s starts running.", l.Name)
			if strings.EqualFold(l.Name, "armor") {
				s.logf("Armor builds up.")
			}
		} else {
			s.logf("%s stopped.", l.Name)
		}
		s.endTurn()
		return nil
	case KRestore:
		s.doRestore(l, dicen)
	case KRelocate:
		if len(s.traces) == 0 {
			s.logf("Nothing to relocate: no trace in progress.")
			break
		}
		worst := 0
		for _, t := range s.traces {
			if r := s.iceRating(t.ice); r > worst {
				worst = r
			}
		}
		if s.test("Relocate", dicen, worst+mod, 1) > 0 {
			s.traces = nil
			s.logf("Relocate successful: the trace is lost.")
		} else {
			s.logf("Relocate failed!")
		}
	default:
		s.logf("%s cannot be executed.", l.Name)
		return errors.New("not executable")
	}
	if info.Degrade == DegUse && l.Cur > 0 {
		l.Cur--
	}
	s.endTurn()
	return nil
}

func (s *Session) doAttack(l *Loaded, target, dicen int) {
	i := s.iceInNode(target)
	if i == nil || s.Range > RangeSensor {
		s.logf("No ICE in range to attack.")
		return
	}
	rating := s.iceRating(i)
	hits := s.test("Attack", dicen, rating+s.WoundMod(), 1)
	base := StageMod
	if l.Stage != "" {
		base = StageFromLetter(l.Stage[0], StageMod)
	}
	res := s.resist(rating, l.Cur)
	st := stageResult(base, hits, res)
	if i.State != StYelling && i.State != StCrashed {
		i.State = StYelling
		i.Engaged = true
		s.setActive()
	}
	if st == StageNone {
		s.logf("Your Attack doesn't hurt the ICE.")
		return
	}
	i.Damage += iceBoxes[st]
	if i.Damage >= 10 {
		i.State = StCrashed
		i.Known.Class, i.Known.Type = true, true
		s.logf("%s crashed!", s.iceName(i))
		return
	}
	s.logf("Your Attack hits the ICE for %s damage.", StageName(st))
}

func (s *Session) doDeception(l *Loaded, dicen, mod int) {
	hit := false
	for _, i := range s.ICEHere() {
		if i.State == StDeceived {
			continue
		}
		hit = true
		if !s.deceivable(i) {
			s.logf("Useless against this ICE-type?")
			continue
		}
		d := s.test("Deception", dicen, s.iceRating(i)+mod, 1)
		e := s.iceDetects(i)
		switch {
		case d > e:
			i.State = StDeceived
			s.logf("Deception successful!")
			s.logf("*ICE ignores you.")
		case d == e:
			s.logf("Deception stalemate!")
			s.failDeceive(i)
		default:
			s.logf("Deception failed!")
			s.failDeceive(i)
		}
	}
	if !hit {
		s.logf("There is no ICE to deceive.")
	}
}

func (s *Session) failDeceive(i *ICEInst) {
	if i.State == StSuspicious {
		i.State = StYelling
		s.logf("*ICE yells!")
		s.setActive()
		return
	}
	i.State = StSuspicious
	s.logf("*ICE is suspicious.")
	s.setPassive()
}

func (s *Session) doSleaze(l *Loaded, dicen, mod int) {
	hit := false
	for _, i := range s.ICEHere() {
		hit = true
		if !s.deceivable(i) {
			s.logf("Useless against this ICE-type?")
			continue
		}
		if i.State == StAsking || i.State == StSuspicious || i.State == StYelling {
			s.logf("Too late for sleazing.")
			continue
		}
		s.Sleazes[s.Node]++
		d := s.test("Sleaze", dicen, s.iceRating(i)+mod+s.Sleazes[s.Node]-1, 1)
		e := s.iceDetects(i)
		if d > e {
			i.State, i.SleazedTurn = StSleazed, s.Turn
			s.logf("Sleaze successful!")
			s.logf("*ICE does not see you. Must do a resleaze...")
		} else {
			i.State = StAsking
			s.logf("Sleaze failed!")
		}
	}
	if !hit {
		s.logf("There is no ICE to sleaze.")
	}
}

func (s *Session) doDataUtil(kind UtilKind, l *Loaded, dicen, mod int) error {
	n := s.Cur()
	if n.Kind != "DS" || s.Range > RangeSensor {
		return errors.New("needs a datastore at sensor range or closer")
	}
	k := s.know(s.Node)
	switch kind {
	case KBrowse:
		if s.test("Browse", dicen, n.Rating+mod, 1) == 0 {
			s.logf("Browse failed.")
			return nil
		}
		k.Files = true
		s.logf("You see %d datafile(s):", len(n.Files))
		for _, f := range n.Files {
			s.logf("  %s %dMp", f.Name, f.Size)
		}
		s.logf("Cannot determine their values.")
	case KEvaluate:
		if s.test("Evaluate", dicen, n.Rating+mod, 1) == 0 {
			s.logf("Evaluate failed.")
			return nil
		}
		k.Files, k.Values = true, true
		for _, f := range n.Files {
			s.logf("  %s %dMp / %d nuyen", f.Name, f.Size, f.Value)
		}
	case KDecrypt:
		if s.test("Decrypt", dicen, n.Rating+mod, 1) == 0 {
			s.logf("Decrypt failed.")
			return nil
		}
		k.Decrypted = true
		s.logf("Decrypt successful: passwords bypassed.")
	}
	return nil
}

func (s *Session) doRestore(l *Loaded, dicen int) {
	h := s.test("Restore", dicen, 4, 1)
	if h == 0 {
		s.logf("Restore failed.")
		return
	}
	p, o := &s.Deck.Persona, s.PersonaOrig
	for _, pr := range []struct{ cur, orig *int }{{&p.Bod, &o.Bod}, {&p.Evasion, &o.Evasion}, {&p.Masking, &o.Masking}, {&p.Sensor, &o.Sensor}} {
		if h > 0 && *pr.cur < *pr.orig {
			*pr.cur++
			h--
		}
	}
	s.logf("Restore repairs your persona programs.")
}

// upkeep runs ongoing utilities once per turn.
func (s *Session) upkeep() {
	for _, l := range s.ActiveMem {
		if !l.Running || l.Cur <= 0 {
			continue
		}
		info := Lookup(l.Name)
		switch info.Degrade {
		case DegTurn, DegTurnFixed:
			l.Cur--
			if l.Cur == 0 {
				s.logf("%s fades away.", l.Name)
			}
		}
		if info.Kind == KMedic && s.DeckBoxes > 0 {
			res := s.Roller.Pool(l.Cur)
			h := 0
			for _, v := range res {
				if v >= 4 {
					h++
				}
			}
			if h > 0 {
				s.DeckBoxes -= h
				if s.DeckBoxes < 0 {
					s.DeckBoxes = 0
				}
				s.logf("Medic repairs your deck (%d box(es)).", h)
			}
		}
	}
}
