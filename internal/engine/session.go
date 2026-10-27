package engine

import (
	"errors"
	"fmt"
	"strings"

	"cyberspace-refactor/internal/dice"
	"cyberspace-refactor/internal/model"
)

// Range values.
const (
	RangeContact     = 0
	RangeSensor      = 1
	RangeObservation = 2
)

// RangeName returns the text for a range.
func RangeName(r int) string {
	switch r {
	case RangeContact:
		return "Contact"
	case RangeSensor:
		return "Sensor"
	}
	return "Observation"
}

// ErrOver is returned by actions after the run ended.
var ErrOver = errors.New("the run is over (jacked out or dumped)")

// RollFunc lets the UI supply physical dice results. Returning nil means
// "roll automatically".
type RollFunc func(reason string, dice, target int) []int

// Loader loads a linked LTG (LNK nodes) by confined relative name.
type Loader func(name string) (*model.Matrix, error)

// Config creates a Session.
type Config struct {
	Matrix    *model.Matrix
	Deck      *model.Deck
	Decker    *model.Decker
	Roller    *dice.Roller
	Loader    Loader
	PoolMode  HackingPoolMode
	AutoKarma bool
}

// Loaded is a program in active memory.
type Loaded struct {
	model.Utility
	Cur     int  // current (possibly degraded) rating
	Running bool // started (for ongoing utilities)
}

// NodeKnow records what the decker has learnt about a node.
type NodeKnow struct {
	Type, Color, Rating bool
	Files, Values       bool
	Decrypted           bool
}

// Session is one decker's run through one or more LTGs.
type Session struct {
	Roller    *dice.Roller
	Manual    RollFunc
	Loader    Loader
	PoolMode  HackingPoolMode
	AutoKarma bool
	PoolAlloc int // max pool dice per test; <0 means all

	Matrix *model.Matrix
	Deck   *model.Deck
	Decker *model.Decker

	Node, Prev int
	Range      int
	ExitSel    int
	Turn       int

	Passive, ActiveAlert bool
	ShutdownLeft         int
	ShutdownOn           bool
	OnSatellite          bool
	Legal                bool // "legal signal": ICE ignores decker (after teleport)

	ActiveMem []*Loaded
	Storage   []model.Utility

	StunBoxes, PhysBoxes, DeckBoxes int
	Karma                           int

	ICE     []*ICEInst
	Know    map[int]*NodeKnow
	Visited map[int]bool
	Locked  map[int]int // SAN id -> locked until turn
	Failed  map[string]int
	Sleazes map[int]int

	traces      []*traceJob
	PersonaOrig model.Persona
	Traced      bool
	Over      bool
	OverMsg   string
	Log       []string
	Rolls     []string
	LinkDepth int
	nextICE   int
}

// New creates a session positioned at the matrix entry node.
func New(c Config) (*Session, error) {
	if c.Matrix == nil || c.Deck == nil || c.Decker == nil {
		return nil, errors.New("engine: matrix, deck and decker are required")
	}
	if len(c.Matrix.Nodes) == 0 {
		return nil, errors.New("engine: matrix has no nodes")
	}
	s := &Session{Roller: c.Roller, Loader: c.Loader, PoolMode: c.PoolMode, AutoKarma: c.AutoKarma,
		PoolAlloc: -1, Deck: c.Deck.Clone(), Decker: &model.Decker{}, Know: map[int]*NodeKnow{},
		Visited: map[int]bool{}, Locked: map[int]int{}, Failed: map[string]int{}, Sleazes: map[int]int{}}
	*s.Decker = *c.Decker
	if s.Roller == nil {
		s.Roller = dice.NewRoller(nil)
	}
	s.StunBoxes, s.PhysBoxes = s.Decker.StunDamage, s.Decker.PhysDamage
	s.DeckBoxes = s.Deck.Persona.Damage
	s.Karma = s.Decker.Karma
	s.Storage = append([]model.Utility(nil), s.Deck.Utilities...)
	s.PersonaOrig = s.Deck.Persona
	s.logf("Logfile on.")
	s.logf("Cyberdeck %s booted: MPCP %d, Bod %d, Evasion %d, Masking %d, Sensor %d.", s.Deck.Persona.Model,
		s.Deck.Persona.MPCP, s.Deck.Persona.Bod, s.Deck.Persona.Evasion, s.Deck.Persona.Masking, s.Deck.Persona.Sensor)
	s.enterMatrix(c.Matrix.Clone(), c.Matrix.Entry)
	return s, nil
}

func (s *Session) logf(format string, a ...any) {
	if len(s.Log) > 20000 {
		s.Log = s.Log[len(s.Log)-10000:]
	}
	s.Log = append(s.Log, fmt.Sprintf("%04d -- ", s.Turn%10000)+fmt.Sprintf(format, a...))
}

func (s *Session) rolled(format string, a ...any) {
	if len(s.Rolls) > 2000 {
		s.Rolls = s.Rolls[1000:]
	}
	s.Rolls = append(s.Rolls, fmt.Sprintf(format, a...))
}

// enterMatrix (re)initialises the world state for a (new) LTG.
func (s *Session) enterMatrix(m *model.Matrix, entry int) {
	s.Matrix = m
	s.ICE = nil
	s.Know = map[int]*NodeKnow{}
	s.Visited = map[int]bool{}
	s.Locked = map[int]int{}
	s.Failed = map[string]int{}
	s.Sleazes = map[int]int{}
	s.Passive, s.ActiveAlert = false, false
	s.ShutdownOn, s.ShutdownLeft = false, 0
	s.Traced = false
	s.OnSatellite = s.Deck.SatUplink && strings.Contains(strings.ToLower(m.Description), "satellite")
	for _, id := range m.IDs() {
		for _, def := range m.Nodes[id].ICE {
			s.nextICE++
			s.ICE = append(s.ICE, &ICEInst{ID: s.nextICE, Def: def, Node: id, Home: id})
		}
	}
	switch m.Alert {
	case "passive":
		s.Passive = true
	case "active":
		s.Passive, s.ActiveAlert = true, true
	}
	if _, ok := m.Nodes[entry]; !ok {
		entry = m.IDs()[0]
	}
	s.Node, s.Prev, s.Range = entry, 0, clampInt(m.Range, 0, 2)
	s.ExitSel = 0
	s.Legal = false
	s.Visited[entry] = true
	s.logf("Plugged into %q (entry node %d, %s range).", m.Description, entry, strings.ToLower(RangeName(s.Range)))
	s.describeNode()
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Cur returns the current node.
func (s *Session) Cur() *model.Node { return s.Matrix.Nodes[s.Node] }

func (s *Session) know(id int) *NodeKnow {
	k := s.Know[id]
	if k == nil {
		k = &NodeKnow{}
		s.Know[id] = k
	}
	return k
}

// NodeLabel renders what the decker knows about a node.
func (s *Session) NodeLabel(id int) string {
	n := s.Matrix.Nodes[id]
	if n == nil {
		return "?"
	}
	k := s.Know[id]
	if k == nil || !k.Type {
		return fmt.Sprintf("#%d ???", id)
	}
	switch {
	case k.Rating && n.Kind != model.LNK:
		return fmt.Sprintf("#%d %s %s-%d", id, n.Kind, n.Color, n.Rating)
	case k.Color && n.Kind != model.LNK:
		return fmt.Sprintf("#%d %s %s", id, n.Kind, n.Color)
	}
	return fmt.Sprintf("#%d %s", id, n.Kind)
}

func (s *Session) describeNode() {
	n := s.Cur()
	s.know(s.Node).Type = s.Range == RangeContact || s.know(s.Node).Type
	if s.Range == RangeContact {
		s.know(s.Node).Type = true
		s.logf("%s", kindName(n.Kind))
		if n.Message != "" {
			s.logf("%s", n.Message)
		}
		if n.Kind == model.DS && len(n.Files) > 0 {
			s.logf("Data present (%d file(s)).", len(n.Files))
		}
	}
}

func kindName(k string) string {
	switch k {
	case model.SAN:
		return "System Access Node (SAN)"
	case model.SPU:
		return "Sub-Processing Unit (SPU)"
	case model.DS:
		return "Datastore (DS)"
	case model.SM:
		return "Slave Module (SM)"
	case model.IOP:
		return "I/O Port (IOP)"
	case model.CPU:
		return "Central Processing Unit (CPU)"
	case model.DLJ:
		return "Dataline Junction (DLJ)"
	case model.LNK:
		return "Link to another Matrix (LNK)"
	case model.MAT:
		return "External Matrix (MAT)"
	}
	return k
}

// WoundMod returns the TN modifier from damage.
func (s *Session) WoundMod() int {
	m := Level(s.StunBoxes)
	if l := Level(s.PhysBoxes); l > m {
		m = l
	}
	if l := Level(s.DeckBoxes); l > m {
		m = l
	}
	if m > 3 {
		m = 3
	}
	return m
}

// Level converts damage boxes to a stage (1/3/6/10 boxes).
func Level(boxes int) int {
	switch {
	case boxes >= 10:
		return StageDeadly
	case boxes >= 6:
		return StageSerious
	case boxes >= 3:
		return StageMod
	case boxes >= 1:
		return StageLight
	}
	return StageNone
}

// Pool returns the decker's hacking pool.
func (s *Session) Pool() int { return HackingPool(s.PoolMode, s.Decker, s.Deck) }

func (s *Session) poolDice() int {
	p := s.Pool()
	if s.PoolAlloc >= 0 && s.PoolAlloc < p {
		p = s.PoolAlloc
	}
	return p
}

// test rolls dice vs target and returns successes, applying Karma if enabled.
func (s *Session) test(reason string, n, tn, need int) int {
	if n < 0 {
		n = 0
	}
	if tn < 2 {
		tn = 2
	}
	var res []int
	if s.Manual != nil {
		res = s.Manual(reason, n, tn)
	}
	if res == nil {
		res = s.Roller.Pool(n)
	}
	hits := dice.Successes(res, tn)
	s.rolled("[%s] %d dice vs TN %d: %v => %d (need %d)", reason, n, tn, res, hits, need)
	for hits < need && s.AutoKarma && s.Karma > 0 {
		s.Karma--
		extra := s.Roller.Die()
		res = append(res, extra)
		if extra >= tn {
			hits++
		}
		s.rolled("  Karma die: %d (Karma left %d) => %d", extra, s.Karma, hits)
	}
	return hits
}

func (s *Session) over(msg string) {
	if !s.Over {
		s.Over, s.OverMsg = true, msg
		s.logf("%s", msg)
	}
}

func (s *Session) check() error {
	if s.Over {
		return ErrOver
	}
	return nil
}

// Wait spends one turn doing nothing.
func (s *Session) Wait() error {
	if err := s.check(); err != nil {
		return err
	}
	s.logf("Waiting...")
	s.endTurn()
	return nil
}

func (s *Session) spend(turns int) {
	for i := 0; i < turns && !s.Over; i++ {
		s.endTurn()
	}
}

// Exits returns the numbered exits of the current node.
func (s *Session) Exits() []int { return s.Matrix.Exits(s.Cur()) }

// SelectExit moves the exit selector by delta (wrapping).
func (s *Session) SelectExit(delta int) {
	ex := s.Exits()
	if len(ex) == 0 {
		s.ExitSel = 0
		return
	}
	s.ExitSel = ((s.ExitSel+delta)%len(ex) + len(ex)) % len(ex)
}

// blockingICE reports ICE that block movement/operations in the current node.
func (s *Session) blockingICE() *ICEInst {
	for _, i := range s.ICE {
		if i.Node == s.Node && !i.Crashed() && iceKey(i.Def.Type) == "barrier" && s.iceActive(i) {
			return i
		}
	}
	return nil
}

// Forward moves closer to the node, or, at contact range, through the
// selected exit.
func (s *Session) Forward() error {
	if err := s.check(); err != nil {
		return err
	}
	if s.Range > RangeContact {
		s.Range--
		s.logf("Moving closer: %s range.", strings.ToLower(RangeName(s.Range)))
		s.describeNode()
		s.endTurn()
		return nil
	}
	ex := s.Exits()
	if len(ex) == 0 {
		if s.Cur().Linked(0) {
			return s.JackOut()
		}
		return errors.New("this node has no exit")
	}
	if b := s.blockingICE(); b != nil {
		s.logf("Your way is blocked by a Barrier ICE.")
		s.endTurn()
		return errors.New("blocked by Barrier ICE")
	}
	if s.ExitSel >= len(ex) {
		s.ExitSel = 0
	}
	dest := ex[s.ExitSel]
	if until, locked := s.Locked[dest]; locked && until > s.Turn {
		s.logf("Node %d is locked out.", dest)
		s.endTurn()
		return errors.New("destination locked")
	}
	s.moveTo(dest)
	s.endTurn()
	return nil
}

func (s *Session) moveTo(dest int) {
	s.Prev = s.Node
	s.leaveNode(s.Node)
	s.Node = dest
	s.Range = RangeObservation
	s.Legal = false
	s.Visited[dest] = true
	s.ExitSel = 0
	s.Sleazes[dest] = 0
	s.logf("Moving to node %d.", dest)
	s.describeNode()
	if n := s.Matrix.Nodes[dest]; n != nil && n.Kind == model.LNK && s.LinkDepth < 8 {
		s.followLink(n)
	}
}

func (s *Session) followLink(n *model.Node) {
	if s.Loader == nil {
		s.logf("Link to %s cannot be followed (no loader).", n.LinkFile)
		return
	}
	m, err := s.Loader(n.LinkFile)
	if err != nil {
		s.logf("Link to %s failed: %v", n.LinkFile, err)
		return
	}
	s.LinkDepth++
	defer func() { s.LinkDepth-- }()
	s.logf("Connecting to %s...", n.LinkFile)
	s.enterMatrix(m.Clone(), n.LinkEntry)
}

// Backward moves away from the node; at observation range returns to the
// previous node (if any, and if it is ICE-free it is entered at sensor range).
func (s *Session) Backward() error {
	if err := s.check(); err != nil {
		return err
	}
	if s.Range < RangeObservation {
		s.Range++
		s.logf("Moving back: %s range.", strings.ToLower(RangeName(s.Range)))
		s.endTurn()
		return nil
	}
	if _, ok := s.Matrix.Nodes[s.Prev]; !ok || s.Prev == 0 || s.Prev == s.Node || !s.Cur().Linked(s.Prev) {
		return errors.New("nowhere to go back to")
	}
	from := s.Node
	s.leaveNode(from)
	s.Node, s.Prev = s.Prev, from
	s.Range = RangeObservation
	if s.iceFree(s.Node) {
		s.Range = RangeSensor
	}
	s.Visited[s.Node] = true
	s.logf("Returning to node %d.", s.Node)
	s.describeNode()
	s.endTurn()
	return nil
}

func (s *Session) iceFree(id int) bool {
	for _, i := range s.ICE {
		if i.Node == id && !i.Crashed() && s.iceActive(i) {
			return false
		}
	}
	return true
}

// GoExternal leaves through the link to the external Matrix (node 0) from an
// exit node: equivalent to jacking out of the grid but staying "online".
func (s *Session) JackOut() error {
	if err := s.check(); err != nil {
		return err
	}
	for _, i := range s.ICE {
		if i.Node == s.Node && !i.Crashed() && i.State == StYelling && i.isBlack() {
			if s.test("Jack out vs Black ICE (Willpower)", s.Decker.Willpower+s.poolDice(), effRating(i.Def.Rating, s.Passive)+s.WoundMod(), 1) == 0 {
				s.logf("You fail to disconnect: Black ICE holds on!")
				s.endTurn()
				return errors.New("black ICE prevents jacking out")
			}
			break
		}
	}
	if s.Traced {
		s.logf("You jack out, but ICE has already reported your entry point.")
	}
	s.over("You jack out of the Matrix.")
	return nil
}

// Teleport moves the decker to a node (CPU Change Node). The decker is then a
// "legal signal" and ICE there ignores him.
func (s *Session) teleport(dest int) {
	s.Prev = 0
	s.Node = dest
	s.Range = RangeObservation
	s.Visited[dest] = true
	s.Legal = true
	for _, i := range s.ICE {
		if i.Node == dest && !i.Crashed() {
			i.State = StDeceived
		}
	}
	s.logf("Teleported to node %d.", dest)
	s.describeNode()
}
