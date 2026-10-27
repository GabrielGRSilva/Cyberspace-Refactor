package engine

import (
	"strings"
	"testing"

	"cyberspace-refactor/internal/dice"
	"cyberspace-refactor/internal/format"
	"cyberspace-refactor/internal/model"
)

const testLTG = `[Matrix]
Description=Test Grid
Nodes=5
Entry=1
Range=2
Alert=none
[Node0]
Type=MAT Green-4 0 0
Links=1
[Node1]
Type=SAN Green-3 4 7
ICEType=White Access-3
Links=0/2
[Node2]
Type=SPU Green-3 4 5
Links=1/3/4/5
[Node3]
Type=DS Blue-2 6 5
Files=Alpha/20/1000 Beta/30/2000
ICEType=White Scramble-3
Links=2
[Node4]
Type=CPU Green-3 2 5
Links=2
[Node5]
Type=SM Green-3 4 3
ICEType=Gray Killer (M)-6
Links=2
`

func newTest(t *testing.T, ltg string, faces ...int) *Session {
	t.Helper()
	m, _, err := format.ParseLTG([]byte(ltg))
	if err != nil {
		t.Fatal(err)
	}
	deck := &model.Deck{Persona: model.Persona{Model: "T", MPCP: 6, Bod: 5, Evasion: 5, Masking: 5, Sensor: 5},
		Hardening: 3, ActiveMem: 400, StorageMem: 20000, LoadSpeed: 100, IOSpeed: 100, Response: 1,
		Utilities: []model.Utility{
			{Name: "Analyze", Rating: 5, Size: 75}, {Name: "Deception", Rating: 5, Size: 50},
			{Name: "Sleaze", Rating: 5, Size: 50}, {Name: "Attack", Rating: 6, Size: 72},
			{Name: "Browse", Rating: 3, Size: 9}, {Name: "Armor", Rating: 3, Size: 27},
			{Name: "Smoke", Rating: 3, Size: 18}, {Name: "Medic", Rating: 4, Size: 64},
			{Name: "Relocate", Rating: 4, Size: 32}, {Name: "Big", Rating: 9, Size: 5000},
		}}
	decker := &model.Decker{Name: "D", Body: 5, Willpower: 5, Intelligence: 5, Quickness: 5, Reaction: 5, Computer: 5, Karma: 2}
	if len(faces) == 0 {
		faces = []int{1}
	}
	s, err := New(Config{Matrix: m, Deck: deck, Decker: decker, Roller: dice.NewRoller(&dice.Fixed{Faces: faces}),
		Loader: func(name string) (*model.Matrix, error) {
			mm, _, err := format.ParseLTG([]byte(strings.Replace(testLTG, "Test Grid", "Linked Grid", 1)))
			return mm, err
		}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// decker always succeeds (5s) or always fails (1s).
func win(_ string, n, _ int) []int  { return fill(n, 12) }
func lose(_ string, n, _ int) []int { return fill(n, 1) }
func fill(n, v int) []int {
	r := make([]int, n)
	for i := range r {
		r[i] = v
	}
	return r
}

func idx(s *Session, name string) int {
	for i, u := range s.Storage {
		if u.Name == name {
			return i
		}
	}
	return -1
}

func loadProg(t *testing.T, s *Session, name string) int {
	t.Helper()
	i := idx(s, name)
	if i < 0 {
		t.Fatalf("%s not in storage", name)
	}
	if err := s.Load(i); err != nil {
		t.Fatal(err)
	}
	for k, l := range s.ActiveMem {
		if l.Name == name {
			return k
		}
	}
	t.Fatalf("%s not active", name)
	return -1
}

func hasLog(s *Session, sub string) bool {
	for _, l := range s.Log {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func TestHackingPoolCalibration(t *testing.T) {
	deck := func(r int) *model.Deck { return &model.Deck{Response: r, Persona: model.Persona{MPCP: 8}} }
	cases := []struct {
		computer, resp, want int
	}{{5, 2, 14}, {5, 1, 12}, {4, 0, 8}} // HEAVYHIT / MAJORLEA / MINORLEA
	for _, c := range cases {
		if got := HackingPool(PoolOriginal, &model.Decker{Computer: c.computer}, deck(c.resp)); got != c.want {
			t.Errorf("pool(%d,%d)=%d want %d", c.computer, c.resp, got, c.want)
		}
	}
	if got := HackingPool(PoolSR2Core, &model.Decker{Intelligence: 6}, deck(0)); got != 4 {
		t.Errorf("SR2 core pool=%d want 4", got)
	}
	if got := HackingPool(PoolOriginal, &model.Decker{Computer: 5, HackingPool: 9}, deck(2)); got != 9 {
		t.Errorf("explicit pool ignored: %d", got)
	}
}

func TestInitiative(t *testing.T) {
	r, d := Initiative(&model.Decker{Reaction: 5}, &model.Deck{Response: 2, PureCyber: true}, true)
	if r != 5 || d != 4 { // 5+2-2, 1+2+1
		t.Fatalf("reaction=%d dice=%d", r, d)
	}
}

func TestRangeNavigation(t *testing.T) {
	s := newTest(t, testLTG)
	if s.Range != RangeObservation || s.Node != 1 {
		t.Fatalf("start %d/%d", s.Node, s.Range)
	}
	_ = s.Forward()
	_ = s.Forward()
	if s.Range != RangeContact {
		t.Fatalf("range %d", s.Range)
	}
	if err := s.Backward(); err != nil || s.Range != RangeSensor {
		t.Fatalf("backward %v %d", err, s.Range)
	}
}

func TestMoveBetweenNodesAndBack(t *testing.T) {
	s := newTest(t, strings.Replace(testLTG, "ICEType=White Access-3\n", "", 1))
	_ = s.Forward()
	_ = s.Forward()
	if err := s.Forward(); err != nil || s.Node != 2 || s.Range != RangeObservation {
		t.Fatalf("move: %v node=%d range=%d", err, s.Node, s.Range)
	}
	if err := s.Backward(); err != nil || s.Node != 1 {
		t.Fatalf("back: %v node=%d", err, s.Node)
	}
	if s.Range != RangeSensor { // ICE-free node is entered at sensor range
		t.Fatalf("range %d", s.Range)
	}
}

func TestICEEscalatesWhenIgnored(t *testing.T) {
	s := newTest(t, testLTG)
	_ = s.Forward() // sensor
	_ = s.Forward() // contact: ICE phase -> Asking
	ice := s.iceInNode(0)
	if ice.State != StAsking || !hasLog(s, "wants your identification") {
		t.Fatalf("state=%v", ice.State)
	}
	_ = s.Wait()
	if ice.State != StSuspicious || !s.Passive {
		t.Fatalf("state=%v passive=%v", ice.State, s.Passive)
	}
	_ = s.Wait()
	if ice.State != StYelling || !s.ActiveAlert {
		t.Fatalf("state=%v active=%v", ice.State, s.ActiveAlert)
	}
}

func TestDeceptionSuccess(t *testing.T) {
	s := newTest(t, testLTG) // ICE dice always 1 -> never detect
	s.Manual = win
	d := loadProg(t, s, "Deception")
	_ = s.Forward()
	_ = s.Forward()
	if err := s.Execute(d, 0); err != nil {
		t.Fatal(err)
	}
	ice := s.iceInNode(0)
	if ice.State != StDeceived {
		t.Fatalf("state=%v log=%v", ice.State, s.Log)
	}
	if s.ActiveMem[d].Cur != 4 {
		t.Fatalf("deception should degrade by use, cur=%d", s.ActiveMem[d].Cur)
	}
	_ = s.Wait()
	if ice.State != StDeceived || s.Passive {
		t.Fatal("deceived ICE should stay quiet")
	}
}

func TestDeceptionFailureEscalates(t *testing.T) {
	s := newTest(t, testLTG)
	s.Manual = lose
	d := loadProg(t, s, "Deception")
	_ = s.Forward()
	_ = s.Forward()
	_ = s.Execute(d, 0)
	if !s.Passive {
		t.Fatal("failed deception should raise passive alert")
	}
	if ice := s.iceInNode(0); ice.State != StYelling || !s.ActiveAlert {
		t.Fatalf("original behaviour: stalemate -> suspicious -> yells in one turn; state=%v", ice.State)
	}
}

func TestSleazeBeforeContact(t *testing.T) {
	s := newTest(t, testLTG)
	s.Manual = win
	sl := loadProg(t, s, "Sleaze")
	_ = s.Forward() // sensor
	if err := s.Execute(sl, 0); err != nil {
		t.Fatal(err)
	}
	ice := s.iceInNode(0)
	if ice.State != StSleazed {
		t.Fatalf("state=%v", ice.State)
	}
	_ = s.Forward() // contact; sleaze expired -> asks
	if ice.State != StAsking {
		t.Fatalf("sleaze must be redone each turn; state=%v", ice.State)
	}
	if err := s.Execute(sl, 0); err != nil || !hasLog(s, "Too late for sleazing") {
		t.Fatalf("expected too-late message: %v", err)
	}
}

func TestSysOpBlockedByICEThenAllowed(t *testing.T) {
	s := newTest(t, testLTG)
	s.Manual = win
	d := loadProg(t, s, "Deception")
	_ = s.Forward()
	_ = s.Forward()
	// Move to the CPU through SPU, unblocked path: kill Access ICE influence first.
	if err := s.SysOp(OpSeeSystem, 0, ""); err == nil {
		t.Fatal("SAN op should be blocked while ICE is watching")
	}
	_ = s.Execute(d, 0)
	if err := s.SysOp(OpSeeSystem, 0, ""); err != nil {
		t.Fatalf("after deception: %v", err)
	}
	if !hasLog(s, "System has:") {
		t.Fatal("summary missing")
	}
}

func TestSysOpWrongNodeAndRange(t *testing.T) {
	s := newTest(t, strings.Replace(testLTG, "ICEType=White Access-3\n", "", 1))
	if err := s.SysOp(OpSeeSystem, 0, ""); err == nil {
		t.Fatal("must need contact range")
	}
	_ = s.Forward()
	_ = s.Forward()
	if err := s.SysOp(OpErase, 0, ""); err == nil {
		t.Fatal("erase not available on a SAN")
	}
}

func goToNode(t *testing.T, s *Session, target int) {
	t.Helper()
	for k := 0; k < 20 && s.Node != target; k++ {
		for s.Range > RangeContact {
			if err := s.Forward(); err != nil {
				t.Fatal(err)
			}
		}
		next, _ := s.nextHop(s.Node, target)
		ex := s.Exits()
		for i, e := range ex {
			if e == next {
				s.ExitSel = i
			}
		}
		if err := s.Forward(); err != nil {
			t.Fatalf("at node %d: %v", s.Node, err)
		}
	}
	if s.Node != target {
		t.Fatalf("could not reach %d", target)
	}
}

func TestDatastoreTransferAndErase(t *testing.T) {
	ltg := strings.ReplaceAll(strings.ReplaceAll(testLTG, "ICEType=White Access-3\n", ""), "ICEType=White Scramble-3\n", "")
	s := newTest(t, ltg)
	s.Manual = win
	goToNode(t, s, 3)
	_ = s.Forward()
	_ = s.Forward()
	if err := s.SysOp(OpTransfer, 0, ""); err != nil {
		t.Fatal(err)
	}
	if idx(s, "Alpha") < 0 {
		t.Fatal("file not in storage")
	}
	if err := s.SysOp(OpErase, 0, ""); err != nil || len(s.Cur().Files) != 1 {
		t.Fatalf("erase: %v files=%d", err, len(s.Cur().Files))
	}
	if err := s.SysOp(OpTransfer, 9, ""); err == nil {
		t.Fatal("bad file index must fail")
	}
}

func TestCPUOpsTeleportAndMap(t *testing.T) {
	ltg := strings.ReplaceAll(testLTG, "ICEType=White Access-3\n", "")
	s := newTest(t, ltg)
	s.Manual = win
	goToNode(t, s, 4)
	_ = s.Forward()
	_ = s.Forward()
	if err := s.SysOp(OpDisplayMap, 0, ""); err != nil {
		t.Fatal(err)
	}
	if s.NodeLabel(3) != "#3 DS Blue-2" {
		t.Fatalf("label %q", s.NodeLabel(3))
	}
	if err := s.SysOp(OpChangeNode, 3, ""); err != nil || s.Node != 3 || !s.Legal {
		t.Fatalf("teleport: %v node=%d legal=%v", err, s.Node, s.Legal)
	}
	if ice := s.iceInNode(0); ice.State != StDeceived {
		t.Fatal("ICE should ignore a legal signal")
	}
	if err := s.SysOp(OpChangeNode, 0, ""); err == nil {
		t.Fatal("teleport to the external matrix must fail")
	}
}

func TestAttackCrashesICE(t *testing.T) {
	s := newTest(t, testLTG)
	s.Manual = win
	a := loadProg(t, s, "Attack")
	_ = s.Forward() // sensor
	for k := 0; k < 6 && s.iceInNode(0) != nil; k++ {
		if err := s.Execute(a, 0); err != nil {
			t.Fatal(err)
		}
	}
	if s.iceInNode(0) != nil {
		t.Fatalf("ICE should have crashed: %v", s.Log)
	}
	if !hasLog(s, "crashed") {
		t.Fatal("crash not logged")
	}
}

func TestAnalyzeReveals(t *testing.T) {
	s := newTest(t, testLTG)
	s.Manual = win
	a := loadProg(t, s, "Analyze")
	_ = s.Forward()
	if err := s.Execute(a, 0); err != nil {
		t.Fatal(err)
	}
	if s.NodeLabel(1) != "#1 SAN Green-3" {
		t.Fatalf("label %q", s.NodeLabel(1))
	}
	ice := s.iceInNode(0)
	_ = s.Execute(a, ice.ID)
	if !ice.Known.Type || !ice.Known.Rating {
		t.Fatalf("ICE knowledge %+v", ice.Known)
	}
	if err := s.Execute(a, 0); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeNeedsSensorRange(t *testing.T) {
	s := newTest(t, testLTG)
	a := loadProg(t, s, "Analyze")
	if err := s.Execute(a, 0); err == nil {
		t.Fatal("observation range cannot analyze")
	}
}

func TestMemoryLimits(t *testing.T) {
	s := newTest(t, testLTG)
	if err := s.Load(idx(s, "Big")); err == nil {
		t.Fatal("5000Mp must not fit")
	}
	before := s.Turn
	loadProg(t, s, "Analyze")
	if s.Turn != before+1 {
		t.Fatalf("75Mp at load speed 100 takes 1 turn, took %d", s.Turn-before)
	}
	if idx(s, "Analyze") >= 0 {
		t.Fatal("non-degradable programs leave storage when loaded")
	}
	loadProg(t, s, "Deception")
	if idx(s, "Deception") < 0 {
		t.Fatal("degradable programs stay in storage")
	}
	if err := s.Load(999); err == nil {
		t.Fatal("bad index")
	}
}

func TestDeceptionRefreshByReload(t *testing.T) {
	s := newTest(t, testLTG)
	d := loadProg(t, s, "Deception")
	s.ActiveMem[d].Cur = 1
	if err := s.Load(idx(s, "Deception")); err != nil || s.ActiveMem[d].Cur != 5 {
		t.Fatalf("reload: %v cur=%d", err, s.ActiveMem[d].Cur)
	}
}

func TestPassiveAlertRaisesICE(t *testing.T) {
	if effRating(6, true) != 9 || effRating(5, true) != 7 || effRating(5, false) != 5 {
		t.Fatal("passive +50% rounded down")
	}
}

func TestBlackICEHurts(t *testing.T) {
	ltg := strings.Replace(testLTG, "ICEType=White Access-3", "ICEType=Black Probe (PhysM)-12 mobile active", 1)
	s := newTest(t, ltg, 5) // ICE dice all 5s: always hits; decker resist dice also 5 -> use TN 12 so resist fails
	s.ActiveAlert, s.Passive = true, true
	_ = s.Forward()
	_ = s.Forward() // contact; ICE phase -> asking
	_ = s.Wait()    // suspicious
	_ = s.Wait()    // yelling
	_ = s.Wait()    // attack
	if s.PhysBoxes == 0 {
		t.Fatalf("black ICE should have caused physical damage: log=%v rolls=%v", s.Log, s.Rolls)
	}
}

func TestTraceAndDumpEndsRun(t *testing.T) {
	ltg := strings.Replace(testLTG, "ICEType=White Access-3", "ICEType=Gray Trace and Dump-6 ever", 1)
	ltg = strings.Replace(ltg, "ICEType=Gray Killer (M)-6\n", "", 1)
	s := newTest(t, ltg, 5)
	_ = s.Forward()
	_ = s.Forward()
	for k := 0; k < 30 && !s.Over; k++ {
		_ = s.Wait()
	}
	if !s.Over || !s.Traced || s.StunBoxes == 0 {
		t.Fatalf("over=%v traced=%v stun=%d log=%v", s.Over, s.Traced, s.StunBoxes, s.Log)
	}
	if err := s.Wait(); err != ErrOver {
		t.Fatalf("actions after the end must fail, got %v", err)
	}
}

func TestSatelliteBlocksTrace(t *testing.T) {
	ltg := strings.Replace(testLTG, "Test Grid", "Crossed Satellite", 1)
	ltg = strings.Replace(ltg, "ICEType=White Access-3", "ICEType=Gray Trace and Dump-6 ever", 1)
	ltg = strings.Replace(ltg, "ICEType=Gray Killer (M)-6\n", "", 1)
	s := newTest(t, ltg, 5)
	s.Deck.SatUplink = true
	s.OnSatellite = true
	_ = s.Forward()
	_ = s.Forward()
	for k := 0; k < 30 && !s.Over; k++ {
		_ = s.Wait()
	}
	if s.Traced {
		t.Fatal("satellite uplink should protect against trace")
	}
}

func TestRelocateCancelsTrace(t *testing.T) {
	ltg := strings.Replace(testLTG, "ICEType=White Access-3", "ICEType=Gray Trace and Dump-4 ever", 1)
	ltg = strings.Replace(ltg, "ICEType=Gray Killer (M)-6\n", "", 1)
	s := newTest(t, ltg, 5)
	s.Manual = win
	r := loadProg(t, s, "Relocate")
	_ = s.Forward()
	_ = s.Forward()
	_ = s.Wait()
	_ = s.Wait()
	_ = s.Wait()
	if len(s.traces) == 0 {
		t.Fatalf("trace should be running: %v", s.Log)
	}
	if err := s.Execute(r, 0); err != nil || len(s.traces) != 0 {
		t.Fatalf("relocate: %v traces=%d", err, len(s.traces))
	}
}

func TestLinkNodeLoadsNextGrid(t *testing.T) {
	ltg := strings.Replace(testLTG, "Type=SM Green-3 4 3", "Type=LNK other.ltg 1 4 3", 1)
	ltg = strings.Replace(ltg, "ICEType=Gray Killer (M)-6\n", "", 1)
	ltg = strings.Replace(ltg, "ICEType=White Access-3\n", "", 1)
	s := newTest(t, ltg)
	s.moveTo(5)
	if s.Matrix.Description != "Linked Grid" || s.Node != 1 {
		t.Fatalf("link not followed: %q node=%d", s.Matrix.Description, s.Node)
	}
}

func TestLinkWithoutLoaderIsSafe(t *testing.T) {
	ltg := strings.Replace(testLTG, "Type=SM Green-3 4 3", "Type=LNK other.ltg 1 4 3", 1)
	s := newTest(t, ltg)
	s.Loader = nil
	s.moveTo(5)
	if !hasLog(s, "cannot be followed") {
		t.Fatal("expected graceful message")
	}
}

func TestHunterICEFollowsDecker(t *testing.T) {
	ltg := strings.Replace(testLTG, "ICEType=Gray Killer (M)-6", "ICEType=Gray Killer (L)-4 mobile ever", 1)
	s := newTest(t, ltg)
	s.Passive = true
	for k := 0; k < 6; k++ {
		_ = s.Wait()
	}
	var killer *ICEInst
	for _, i := range s.ICE {
		if iceKey(i.Def.Type) == "killer" {
			killer = i
		}
	}
	if killer.Node != s.Node {
		t.Fatalf("hunter ICE should have reached the decker: at %d, decker %d", killer.Node, s.Node)
	}
	if !hasLog(s, "New ICE approaches") {
		t.Fatal("arrival not logged")
	}
}

func TestLockedSANStopsICE(t *testing.T) {
	ltg := strings.Replace(testLTG, "ICEType=Gray Killer (M)-6", "ICEType=Gray Killer (L)-4 mobile ever", 1)
	s := newTest(t, ltg)
	s.Locked[1] = 1000
	for k := 0; k < 8; k++ {
		_ = s.Wait()
	}
	for _, i := range s.ICE {
		if iceKey(i.Def.Type) == "killer" && i.Node == 1 {
			t.Fatal("ICE entered a locked SAN")
		}
	}
}

func TestKarmaAutoSpend(t *testing.T) {
	s := newTest(t, testLTG)
	s.AutoKarma = true
	s.Manual = lose
	got := s.test("t", 3, 4, 1)
	if s.Karma != 0 && got == 0 {
		t.Fatalf("karma should be spent on failure: karma=%d hits=%d", s.Karma, got)
	}
}

func TestJackOut(t *testing.T) {
	s := newTest(t, testLTG)
	if err := s.JackOut(); err != nil || !s.Over {
		t.Fatalf("jackout: %v over=%v", err, s.Over)
	}
	if err := s.Forward(); err != ErrOver {
		t.Fatalf("got %v", err)
	}
}

func TestWoundModifier(t *testing.T) {
	s := newTest(t, testLTG)
	s.StunBoxes = 3
	if s.WoundMod() != 2 {
		t.Fatalf("mod=%d", s.WoundMod())
	}
	s.PhysBoxes = 10
	if s.WoundMod() != 3 {
		t.Fatalf("cap=%d", s.WoundMod())
	}
}

func TestLevelBoundaries(t *testing.T) {
	for boxes, want := range map[int]int{0: 0, 1: 1, 2: 1, 3: 2, 5: 2, 6: 3, 9: 3, 10: 4, 15: 4} {
		if got := Level(boxes); got != want {
			t.Errorf("Level(%d)=%d want %d", boxes, got, want)
		}
	}
}

func TestNewValidates(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("empty config must fail")
	}
	if _, err := New(Config{Matrix: model.NewMatrix(), Deck: &model.Deck{}, Decker: &model.Decker{}}); err == nil {
		t.Fatal("empty matrix must fail")
	}
}

func TestSessionDoesNotMutateSourceMatrix(t *testing.T) {
	m, _, _ := format.ParseLTG([]byte(testLTG))
	s, _ := New(Config{Matrix: m, Deck: &model.Deck{ActiveMem: 10}, Decker: &model.Decker{}})
	s.Matrix.Nodes[3].Files = nil
	if len(m.Nodes[3].Files) != 2 {
		t.Fatal("source matrix was mutated")
	}
}
