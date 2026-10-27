package format

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cyberspace-refactor/internal/model"
)

const dataDir = "../../assets/DATA"

func TestParseICE(t *testing.T) {
	cases := []struct {
		in   string
		want model.ICE
	}{
		{"White Access-4", model.ICE{Class: "White", Type: "Access", Rating: 4}},
		{"Gray Killer (M)-7", model.ICE{Class: "Gray", Type: "Killer", Stage: "M", Rating: 7}},
		{"Black Probe (PhysM)-8 amobile active", model.ICE{Class: "Black", Type: "Probe", Stage: "PhysM", Rating: 8, Mobility: "amobile", Trigger: "active"}},
		{"Gray Trace and Burn-7", model.ICE{Class: "Gray", Type: "Trace and Burn", Rating: 7}},
		{"White Access-3 atrigger -2", model.ICE{Class: "White", Type: "Access", Rating: 3, Trigger: "atrigger", Extra: []string{"-2"}}},
		{"White Access-5 ptrigger 2", model.ICE{Class: "White", Type: "Access", Rating: 5, Trigger: "ptrigger", Extra: []string{"2"}}},
	}
	for _, c := range cases {
		got, err := ParseICE(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %+v want %+v", c.in, got, c.want)
		}
		again, err := ParseICE(FormatICE(got))
		if err != nil || !reflect.DeepEqual(again, got) {
			t.Errorf("%q round trip: %+v / %v", c.in, again, err)
		}
	}
	if _, err := ParseICE("garbage"); err == nil {
		t.Error("garbage should fail")
	}
}

func TestParseFiles(t *testing.T) {
	got := ParseFiles("Design Data/40/5000 Illegal/110/6000 Bribe/30/2500 Secret/10/1/pw ")
	want := []model.File{
		{Name: "Design Data", Size: 40, Value: 5000},
		{Name: "Illegal", Size: 110, Value: 6000},
		{Name: "Bribe", Size: 30, Value: 2500},
		{Name: "Secret", Size: 10, Value: 1, Password: "pw"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	multi := ParseFiles("S01: Ofertas da semana./80/5000\nS01:Relação/100/8000 ")
	if len(multi) != 2 || multi[0].Name != "S01: Ofertas da semana." {
		t.Fatalf("multi %+v", multi)
	}
	if len(ParseFiles("")) != 0 {
		t.Fatal("empty")
	}
}

func TestParseUtility(t *testing.T) {
	cases := []struct {
		in   string
		want model.Utility
	}{
		{"Analyze-5-75", model.Utility{Name: "Analyze", Rating: 5, Size: 75}},
		{"Auto Execute-6-36", model.Utility{Name: "Auto Execute", Rating: 6, Size: 36}},
		{"Attack (ME)-6-128", model.Utility{Name: "Attack", Rating: 6, Size: 128, Stage: "ME", Target: "E"}},
		{"PV2005-120", model.Utility{Name: "PV2005", Size: 120}},
		{"F4447-80-80", model.Utility{Name: "F4447", Rating: 80, Size: 80}},
	}
	for _, c := range cases {
		got, err := ParseUtility(c.in)
		if err != nil || got != c.want {
			t.Errorf("%q: %+v %v want %+v", c.in, got, err, c.want)
		}
	}
	if _, err := ParseUtility("ramble-x"); err == nil {
		t.Error("expected error")
	}
}

func TestParseLTGFirstRun(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(dataDir, "FIRSTRUN.LTG"))
	if err != nil {
		t.Skip(err)
	}
	m, _, err := ParseLTG(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.Description != "Firstrun-Matrix" || len(m.Nodes) != 8 || m.Entry != 1 {
		t.Fatalf("unexpected: %+v", m)
	}
	n5 := m.Nodes[5]
	if n5.Kind != "CPU" || n5.Color != "Red" || n5.Rating != 5 || n5.X != 4 || n5.Y != 3 {
		t.Fatalf("node5 %+v", n5)
	}
	if len(n5.ICE) != 1 || n5.ICE[0].Type != "Trace and Burn" {
		t.Fatalf("ice %+v", n5.ICE)
	}
	if f := m.Nodes[6].Files; len(f) != 3 || f[0].Name != "Design Data" {
		t.Fatalf("files %+v", f)
	}
}

func TestParseLTGLink(t *testing.T) {
	src := "[Matrix]\nDescription=x\nNodes=2\nEntry=1\nRange=2\nAlert=none\n[Node1]\nType=SAN Green-4 1 1\nLinks=0/2\n[Node2]\nType=LNK other.ltg 06 4 2\nLinks=1\n"
	m, _, err := ParseLTG([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	n := m.Nodes[2]
	if n.Kind != "LNK" || n.LinkFile != "other.ltg" || n.LinkEntry != 6 || n.X != 4 || n.Y != 2 {
		t.Fatalf("%+v", n)
	}
}

func TestParseLTGRejects(t *testing.T) {
	if _, _, err := ParseLTG([]byte("hello")); err != ErrNotLTG {
		t.Fatalf("got %v", err)
	}
	if _, _, err := ParseLTG(make([]byte, 4096)); err != ErrNotLTG {
		t.Fatalf("nul data: %v", err)
	}
}

func TestFixLinksAddsBackLink(t *testing.T) {
	src := "[Matrix]\nEntry=1\n[Node1]\nType=SAN Green-4 1 1\nLinks=2/9\n[Node2]\nType=SPU Green-4 1 2\n"
	m, warns, _ := ParseLTG([]byte(src))
	if !m.Nodes[2].Linked(1) {
		t.Fatal("back-link missing")
	}
	if m.Nodes[1].Linked(9) {
		t.Fatal("dangling link kept")
	}
	if len(warns) == 0 {
		t.Fatal("expected warnings")
	}
}

func TestExitsClockwise(t *testing.T) {
	m := model.NewMatrix()
	c := &model.Node{ID: 1, Kind: "SPU", X: 5, Y: 5, Links: []int{2, 3, 4, 5}}
	m.Nodes[1] = c
	m.Nodes[2] = &model.Node{ID: 2, Kind: "IOP", X: 5, Y: 4, Links: []int{1}} // up (12)
	m.Nodes[3] = &model.Node{ID: 3, Kind: "IOP", X: 5, Y: 6, Links: []int{1}} // down (6)
	m.Nodes[4] = &model.Node{ID: 4, Kind: "IOP", X: 4, Y: 5, Links: []int{1}} // left (9)
	m.Nodes[5] = &model.Node{ID: 5, Kind: "IOP", X: 6, Y: 5, Links: []int{1}} // right (3)
	got := m.Exits(c)
	want := []int{3, 4, 2, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestLTGRoundTripSynthetic(t *testing.T) {
	m := model.NewMatrix()
	m.Description = "Round Trip ñ"
	m.Nodes[0] = &model.Node{ID: 0, Kind: "MAT", Color: "Green", Rating: 4, Links: []int{1}}
	m.Nodes[1] = &model.Node{ID: 1, Kind: "SAN", Color: "Orange", Rating: 4, X: 4, Y: 7, Links: []int{0, 2},
		Message: "Hello", ICE: []model.ICE{{Class: "White", Type: "Access", Rating: 4}, {Class: "Gray", Type: "Killer", Stage: "M", Rating: 7, Mobility: "mobile"}}}
	m.Nodes[2] = &model.Node{ID: 2, Kind: "DS", Color: "Red", Rating: 4, X: 2, Y: 4, Links: []int{1},
		Files: []model.File{{Name: "A B", Size: 10, Value: 5, Password: "x"}}}
	back, _, err := ParseLTG(WriteLTG(m))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, back) {
		t.Fatalf("round trip mismatch\n%+v\n%+v", m.Nodes[1], back.Nodes[1])
	}
}

func TestDeckAndNPCRoundTrip(t *testing.T) {
	d := &model.Deck{Persona: model.Persona{Model: "Test", MPCP: 6, Bod: 5, Evasion: 5, Masking: 4, Sensor: 6, Damage: 1},
		Hardening: 3, ActiveMem: 300, StorageMem: 2000, LoadSpeed: 20, IOSpeed: 20, Response: 1, SatUplink: true, DisplayMode: 1,
		Utilities: []model.Utility{{Name: "Analyze", Rating: 5, Size: 75}, {Name: "Attack", Rating: 6, Size: 128, Stage: "ME", Target: "E"}, {Name: "PV2005", Size: 120}}}
	back, _, err := ParseDeck(WriteDeck(d))
	if err != nil {
		t.Fatal(err)
	}
	back.Persona.Raw = nil
	d.Persona.Raw = nil
	if !reflect.DeepEqual(d, back) {
		t.Fatalf("deck mismatch\n%+v\n%+v", d, back)
	}
	n := &model.Decker{Name: "Rainbow", Body: 5, Quickness: 5, Strength: 5, Charisma: 8, Intelligence: 5, Willpower: 5, Reaction: 5, ReactionDice: 1, Computer: 5, Karma: 15}
	nb, _, err := ParseNPC(WriteNPC(n))
	if err != nil || !reflect.DeepEqual(n, nb) {
		t.Fatalf("npc mismatch %+v %v", nb, err)
	}
}

func TestParseRealRainbow(t *testing.T) {
	dir := filepath.Join(dataDir, "Aventura Senhor das Armas")
	if b, err := os.ReadFile(filepath.Join(dir, "RAINBOW.DEK")); err == nil {
		d, _, err := ParseDeck(b)
		if err != nil || d.Persona.MPCP != 6 || d.ActiveMem != 300 || d.StorageMem != 2000 || len(d.Utilities) != 29 {
			t.Fatalf("deck %+v err=%v", d, err)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "RAINBOW.NPC")); err == nil {
		n, _, err := ParseNPC(b)
		if err != nil || n.Name != "Rainbow" || n.Computer != 5 || n.Karma != 15 || n.ReactionDice != 1 {
			t.Fatalf("npc %+v err=%v", n, err)
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(dataDir, "FIRSTRUN.LTG"))
	if err != nil {
		t.Skip(err)
	}
	m, _, _ := ParseLTG(data)
	js, err := MatrixToJSON(m)
	if err != nil {
		t.Fatal(err)
	}
	back, err := MatrixFromJSON(js)
	if err != nil || !reflect.DeepEqual(m, back) {
		t.Fatalf("json round trip failed: %v", err)
	}
	if _, err := MatrixFromJSON([]byte(`{"version":2}`)); err == nil {
		t.Error("version check")
	}
	if _, err := MatrixFromJSON([]byte(`{"version":1,"nodes":[{"ID":1,"Kind":"BAD"}]}`)); err == nil {
		t.Error("kind check")
	}
}

// TestCorpus parses every real legacy file: nothing may panic, a clear
// majority must parse, and every parsed LTG must survive a write/read cycle.
func TestCorpus(t *testing.T) {
	if _, err := os.Stat(dataDir); err != nil {
		t.Skip("corpus not present")
	}
	var total, ok, bad int
	_ = filepath.WalkDir(dataDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".ltg":
			total++
			m, _, err := ParseLTG(b)
			if err != nil {
				bad++
				return nil
			}
			ok++
			again, _, err := ParseLTG(WriteLTG(m))
			if err != nil {
				t.Errorf("%s: rewrite unparsable: %v", p, err)
				return nil
			}
			if len(again.Nodes) != len(m.Nodes) {
				t.Errorf("%s: node count changed %d -> %d", p, len(m.Nodes), len(again.Nodes))
			}
			for id, n := range m.Nodes {
				a := again.Nodes[id]
				if a == nil || a.Kind != n.Kind || a.Color != n.Color || a.Rating != n.Rating || len(a.ICE) != len(n.ICE) || len(a.Files) != len(n.Files) {
					t.Errorf("%s: node %d changed after round trip", p, id)
				}
			}
		case ".dek":
			total++
			if d, _, err := ParseDeck(b); err == nil {
				ok++
				if _, _, err := ParseDeck(WriteDeck(d)); err != nil {
					t.Errorf("%s: deck rewrite unparsable", p)
				}
			} else {
				bad++
			}
		case ".npc":
			total++
			if n, _, err := ParseNPC(b); err == nil {
				ok++
				if _, _, err := ParseNPC(WriteNPC(n)); err != nil {
					t.Errorf("%s: npc rewrite unparsable", p)
				}
			} else {
				bad++
			}
		}
		return nil
	})
	t.Logf("corpus: %d files, %d parsed, %d rejected", total, ok, bad)
	if total == 0 || ok*100/total < 90 {
		t.Fatalf("too few files parsed: %d/%d", ok, total)
	}
}

func FuzzParseLTG(f *testing.F) {
	f.Add([]byte("[Matrix]\nEntry=1\n[Node1]\nType=SAN Green-4 1 1\nICEType=White Access-4\nLinks=1/2\nFiles=A/1/2 "))
	f.Fuzz(func(t *testing.T, b []byte) {
		m, _, err := ParseLTG(b)
		if err == nil {
			_ = WriteLTG(m)
		}
	})
}

func FuzzParseDeck(f *testing.F) {
	f.Add([]byte("[Persona]\nModel=x\nMPCP=6+0-0-0\n[Utilities]\nProgram1=Attack (ME)-6-128"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if d, _, err := ParseDeck(b); err == nil {
			_ = WriteDeck(d)
		}
	})
}

func FuzzParseNPC(f *testing.F) {
	f.Add([]byte("[Attributes]\nBody=5\nReaction=5+1D\n[Skills]\nComputer=5"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if n, _, err := ParseNPC(b); err == nil {
			_ = WriteNPC(n)
		}
	})
}
