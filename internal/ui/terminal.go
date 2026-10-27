package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	capp "cyberspace-refactor/internal/app"
	"cyberspace-refactor/internal/assets"
	"cyberspace-refactor/internal/dice"
	"cyberspace-refactor/internal/engine"
	"cyberspace-refactor/internal/format"
	"cyberspace-refactor/internal/model"
)

type terminal struct {
	a    *App
	sess *engine.Session

	deckRel, deckerRel, ltgRel string
	deck                       *model.Deck
	decker                     *model.Decker
	matrix                     *model.Matrix
	warnings                   []string

	chooseDeck, chooseDecker, chooseLTG, start, demo, newRun widget.Clickable
	saveLog, saveDeck                                        widget.Clickable
	fwd, back, left, right, wait, jack                       widget.Clickable
	load, unload, exec, del                                  widget.Clickable
	sysops                                                   [6]widget.Clickable

	active, storage, files, ices *listBox
	logList                      widget.List
	arg, poolEd, diceEd          widget.Editor
	manual                       widget.Bool
	cond                         [3][4]widget.Clickable

	mapTag     struct{ _ byte }
	mapPos     map[int]image.Point
	soundMark  int
	lastMatrix *model.Matrix
	opCache    map[*image.RGBA]paint.ImageOp
}

func newTerminal(a *App) *terminal {
	t := &terminal{a: a, active: newListBox(), storage: newListBox(), files: newListBox(), ices: newListBox(),
		mapPos: map[int]image.Point{}, opCache: map[*image.RGBA]paint.ImageOp{}}
	t.logList.Axis = layout.Vertical
	t.logList.ScrollToEnd = true
	t.arg.SingleLine = true
	t.poolEd.SingleLine = true
	t.diceEd.SingleLine = true
	return t
}

func (t *terminal) imageOp(img *image.RGBA) paint.ImageOp {
	if op, ok := t.opCache[img]; ok {
		return op
	}
	if len(t.opCache) > 400 {
		t.opCache = map[*image.RGBA]paint.ImageOp{}
	}
	op := paint.NewImageOp(img)
	t.opCache[img] = op
	return op
}

func (t *terminal) startRun() {
	if t.deck == nil || t.decker == nil || t.matrix == nil {
		t.a.setStatus("Choose a cyberdeck, a decker and a matrix first.")
		return
	}
	s := t.a.ws.Settings
	sess, err := engine.New(engine.Config{
		Matrix: t.matrix, Deck: t.deck, Decker: t.decker,
		Loader:    t.a.ws.LinkLoader(filepath.ToSlash(filepath.Dir(t.ltgRel))),
		PoolMode:  engine.HackingPoolMode(s.PoolMode),
		AutoKarma: s.AutoKarma,
	})
	if err != nil {
		t.a.setStatus("Cannot start: " + err.Error())
		return
	}
	t.sess = sess
	t.soundMark = 0
	t.lastMatrix = nil
	sess.Manual = t.manualRoll
	t.a.ws.Library.SetImageDirs(capp.ImageDirsFor(sess.Matrix, t.ltgRel)...)
	t.a.setStatus("Run started. Good luck, chummer.")
	t.playNew()
	t.refreshLists()
}

func (t *terminal) manualRoll(reason string, n, tn int) []int {
	if !t.manual.Value {
		return nil
	}
	var nums []int
	for _, f := range strings.FieldsFunc(t.diceEd.Text(), func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if v, err := strconv.Atoi(f); err == nil && v >= 1 && v <= 60 {
			nums = append(nums, v)
		}
	}
	if len(nums) == 0 {
		return nil
	}
	t.diceEd.SetText("")
	if len(nums) > n {
		nums = nums[:n]
	}
	if len(nums) < n {
		nums = append(nums, dice.NewRoller(nil).Pool(n-len(nums))...)
	}
	return nums
}

func (t *terminal) do(fn func() error) {
	if t.sess == nil {
		return
	}
	t.sess.PoolAlloc = -1
	if v := strings.TrimSpace(t.poolEd.Text()); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			t.sess.PoolAlloc = n
		}
	}
	if err := fn(); err != nil {
		t.a.setStatus(err.Error())
	} else {
		t.a.setStatus("")
	}
	t.playNew()
	t.refreshLists()
	if t.sess.Matrix != t.lastMatrix {
		t.lastMatrix = t.sess.Matrix
		t.a.ws.Library.SetImageDirs(capp.ImageDirsFor(t.sess.Matrix, t.ltgRel)...)
	}
}

func (t *terminal) playNew() {
	if t.sess == nil {
		return
	}
	lines := t.sess.Log
	if t.soundMark > len(lines) {
		t.soundMark = 0
	}
	played := 0
	for _, l := range lines[t.soundMark:] {
		if i := strings.Index(l, " -- "); i >= 0 {
			l = l[i+4:]
		}
		if ev := assets.SoundForLog(l); ev != "" && played < 2 && t.a.ws.Settings.Sound {
			t.a.ws.Library.Play(ev)
			played++
		}
	}
	t.soundMark = len(lines)
}

func (t *terminal) refreshLists() {
	s := t.sess
	if s == nil {
		return
	}
	var act, sto, ice, fl []string
	for _, l := range s.ActiveMem {
		run := ""
		if l.Running {
			run = " *"
		}
		act = append(act, fmt.Sprintf("%s-%d/%d [%dMp]%s", l.Name, l.Cur, l.Rating, l.Size, run))
	}
	for _, u := range s.Storage {
		if u.Rating > 0 {
			sto = append(sto, fmt.Sprintf("%s-%d [%dMp]", u.Name, u.Rating, u.Size))
		} else {
			sto = append(sto, fmt.Sprintf("%s [%dMp] (data)", u.Name, u.Size))
		}
	}
	for _, i := range s.ICEHere() {
		ice = append(ice, fmt.Sprintf("#%d %s [%s]", i.ID, iceLabel(i), stateName(i.State)))
	}
	n := s.Cur()
	if n.Kind == model.DS && s.Range == engine.RangeContact {
		for k, f := range n.Files {
			if s.Know[s.Node] != nil && s.Know[s.Node].Files {
				v := "?"
				if s.Know[s.Node].Values {
					v = strconv.Itoa(f.Value)
				}
				fl = append(fl, fmt.Sprintf("%d. %s %dMp %s¥", k+1, f.Name, f.Size, v))
			} else {
				fl = append(fl, fmt.Sprintf("%d. file", k+1))
			}
		}
	}
	t.active.Set(act)
	t.storage.Set(sto)
	t.ices.Set(ice)
	t.files.Set(fl)
}

func iceLabel(i *engine.ICEInst) string {
	if i.Known.Type {
		s := i.Def.Class + " " + i.Def.Type
		if i.Known.Rating {
			s += fmt.Sprintf("-%d", i.Def.Rating)
		}
		return s
	}
	return "ICE"
}

func stateName(s engine.State) string {
	switch s {
	case engine.StAsking:
		return "wants ID"
	case engine.StDeceived:
		return "deceived"
	case engine.StSleazed:
		return "sleazed"
	case engine.StSuspicious:
		return "suspicious"
	case engine.StYelling:
		return "alarmed"
	case engine.StCrashed:
		return "crashed"
	}
	return "watching"
}

func (t *terminal) selectedICE() int {
	if t.ices.Sel >= 0 && t.ices.Sel < len(t.ices.items) {
		var id int
		_, _ = fmt.Sscanf(t.ices.items[t.ices.Sel], "#%d", &id)
		return id
	}
	return 0
}

func (t *terminal) pickDeck() {
	t.a.pick.show(t.a.ws, "Choose a cyberdeck (.dek / .npc)", false, []string{".dek", ".npc"}, "", func(rel string) {
		d, w, err := t.a.ws.LoadDeck(rel)
		if err != nil {
			t.a.setStatus("Deck: " + err.Error())
			return
		}
		t.deck, t.deckRel, t.warnings = d, rel, w
		t.a.ws.Settings.LastDeck = rel
	})
}

func (t *terminal) pickDecker() {
	t.a.pick.show(t.a.ws, "Choose a decker (.npc)", false, []string{".npc"}, "", func(rel string) {
		d, _, err := t.a.ws.LoadDecker(rel)
		if err != nil {
			t.a.setStatus("Decker: " + err.Error())
			return
		}
		t.decker, t.deckerRel = d, rel
		t.a.ws.Settings.LastDecker = rel
	})
}

func (t *terminal) pickLTG() {
	t.a.pick.show(t.a.ws, "Choose a matrix (.ltg / .json)", false, []string{".ltg", ".json"}, "", func(rel string) {
		m, w, err := t.a.ws.LoadMatrix(rel)
		if err != nil {
			t.a.setStatus("Matrix: " + err.Error())
			return
		}
		t.matrix, t.ltgRel, t.warnings = m, rel, w
		t.a.ws.Settings.LastLTG = rel
	})
}

func (t *terminal) loadDemo() {
	try := func(rel string, f func(string)) {
		if _, err := t.a.ws.Root.ResolveFold(rel); err == nil {
			f(rel)
		}
	}
	if m, w, err := t.a.ws.LoadMatrix("DATA/FIRSTRUN.LTG"); err == nil {
		t.matrix, t.ltgRel, t.warnings = m, "DATA/FIRSTRUN.LTG", w
	}
	if d, _, err := t.a.ws.LoadDeck("DATA/FUCHI4_2.DEK"); err == nil {
		t.deck, t.deckRel = d, "DATA/FUCHI4_2.DEK"
	}
	if d, _, err := t.a.ws.LoadDecker("DATA/HEAVYHIT.NPC"); err == nil {
		t.decker, t.deckerRel = d, "DATA/HEAVYHIT.NPC"
	}
	_ = try
	t.startRun()
}

func (t *terminal) layout(gtx C) D {
	if t.sess == nil {
		return t.layoutSetup(gtx)
	}
	t.handleInput(gtx)
	th := t.a.th
	return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(fixedW(230, t.leftColumn)),
					gap(4),
					layout.Flexed(1, t.centerColumn),
					gap(4),
					layout.Rigid(fixedW(340, func(gtx C) D {
						return panel(gtx, th, "Matrix map (known nodes)", t.mapWidget)
					})),
				)
			}),
			layout.Rigid(spacer(0, 4)),
			layout.Rigid(fixedH(236, t.bottomRow)),
		)
	})
}

func (t *terminal) layoutSetup(gtx C) D {
	th := t.a.th
	if t.chooseDeck.Clicked(gtx) {
		t.pickDeck()
	}
	if t.chooseDecker.Clicked(gtx) {
		t.pickDecker()
	}
	if t.chooseLTG.Clicked(gtx) {
		t.pickLTG()
	}
	if t.demo.Clicked(gtx) {
		t.loadDemo()
	}
	if t.start.Clicked(gtx) {
		t.startRun()
	}
	show := func(label, v string) layout.Widget {
		if v == "" {
			v = "(none)"
		}
		return label2(th, label, v)
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = gtx.Dp(unit.Dp(720))
		return panel(gtx, th, "Start a Matrix run", func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(smallLabel(th, "Choose a cyberdeck, a decker and a Local Telecommunications Grid, then plug in.", colDim)),
				layout.Rigid(spacer(0, 10)),
				layout.Rigid(func(gtx C) D {
					return row(gtx, rigid(button(th, &t.chooseDeck, "Cyberdeck...")), gap(8), rigid(show("Deck", t.deckRel)))
				}),
				layout.Rigid(spacer(0, 6)),
				layout.Rigid(func(gtx C) D {
					return row(gtx, rigid(button(th, &t.chooseDecker, "Decker...")), gap(8), rigid(show("Decker", t.deckerRel)))
				}),
				layout.Rigid(spacer(0, 6)),
				layout.Rigid(func(gtx C) D {
					return row(gtx, rigid(button(th, &t.chooseLTG, "Matrix (LTG)...")), gap(8), rigid(show("Matrix", t.ltgRel)))
				}),
				layout.Rigid(spacer(0, 10)),
				layout.Rigid(func(gtx C) D {
					return row(gtx, rigid(hotButton(th, &t.start, "Plug in!", colGreenDark)), gap(8),
						rigid(button(th, &t.demo, "Quick demo (First Run)")))
				}),
				layout.Rigid(spacer(0, 8)),
				layout.Rigid(smallLabel(th, strings.Join(firstN(t.warnings, 6), "\n"), colAmber)),
			)
		})
	})
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func label2(th *material.Theme, k, v string) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(label(th, k+": ", colDim)),
			layout.Rigid(label(th, v, colFg)))
	}
}

func (t *terminal) handleInput(gtx C) {
	s := t.sess
	if t.fwd.Clicked(gtx) {
		t.do(s.Forward)
	}
	if t.back.Clicked(gtx) {
		t.do(s.Backward)
	}
	if t.left.Clicked(gtx) {
		s.SelectExit(-1)
	}
	if t.right.Clicked(gtx) {
		s.SelectExit(1)
	}
	if t.wait.Clicked(gtx) {
		t.do(s.Wait)
	}
	if t.jack.Clicked(gtx) {
		t.do(s.JackOut)
	}
	if t.load.Clicked(gtx) {
		if i := t.storage.Sel; i >= 0 {
			t.do(func() error { return s.Load(i) })
		}
	}
	if t.unload.Clicked(gtx) {
		if i := t.active.Sel; i >= 0 {
			t.do(func() error { return s.Unload(i) })
		}
	}
	if t.exec.Clicked(gtx) {
		if i := t.active.Sel; i >= 0 {
			id := t.selectedICE()
			t.do(func() error { return s.Execute(i, id) })
		} else {
			t.a.setStatus("Select a program in Active Memory first.")
		}
	}
	if t.del.Clicked(gtx) {
		switch {
		case t.active.Sel >= 0:
			i := t.active.Sel
			t.active.Sel = -1
			t.do(func() error { return s.DeleteActive(i) })
		case t.storage.Sel >= 0:
			i := t.storage.Sel
			t.storage.Sel = -1
			t.do(func() error { return s.DeleteStorage(i) })
		}
	}
	if t.newRun.Clicked(gtx) {
		t.sess = nil
	}
	if t.saveLog.Clicked(gtx) {
		t.a.pick.show(t.a.ws, "Save history log", true, []string{".txt"}, "LOGS/run.txt", func(rel string) {
			if err := t.a.ws.SaveText(rel, []byte(strings.Join(s.Log, "\r\n")+"\r\n")); err != nil {
				t.a.setStatus("Save log: " + err.Error())
			} else {
				t.a.setStatus("Log saved to " + rel)
			}
		})
	}
	if t.saveDeck.Clicked(gtx) {
		t.a.pick.show(t.a.ws, "Save cyberdeck configuration", true, []string{".dek"}, "DATA/saved.dek", func(rel string) {
			d := s.Deck.Clone()
			d.Utilities = nil
			for _, l := range s.ActiveMem {
				d.Utilities = append(d.Utilities, l.Utility)
			}
			d.Utilities = append(d.Utilities, s.Storage...)
			if err := t.a.ws.Save(rel, format.WriteDeck(d)); err != nil {
				t.a.setStatus("Save deck: " + err.Error())
			} else {
				t.a.setStatus("Deck saved to " + rel)
			}
		})
	}
	// Condition monitor: click a box to set/clear that level.
	for r := 0; r < 3; r++ {
		for k := 0; k < 4; k++ {
			if t.cond[r][k].Clicked(gtx) {
				boxes := []int{0, 1, 3, 6, 10}
				p := t.condPtr(r)
				if engine.Level(*p) == k+1 {
					*p = boxes[k]
				} else {
					*p = boxes[k+1]
				}
			}
		}
	}
	// System operations.
	ops := engine.OpsFor(s.Cur().Kind)
	for i := range ops {
		if i < len(t.sysops) && t.sysops[i].Clicked(gtx) {
			op := ops[i]
			arg := 0
			text := strings.TrimSpace(t.arg.Text())
			switch op {
			case engine.OpChangeNode:
				arg, _ = strconv.Atoi(text)
				text = ""
			case engine.OpErase, engine.OpEdit, engine.OpRead, engine.OpTransfer:
				arg = t.files.Sel
				if arg < 0 {
					arg = 0
				}
			}
			t.do(func() error { return s.SysOp(op, arg, text) })
		}
	}
}

func (t *terminal) condPtr(r int) *int {
	switch r {
	case 0:
		return &t.sess.DeckBoxes
	case 1:
		return &t.sess.StunBoxes
	}
	return &t.sess.PhysBoxes
}

func (t *terminal) leftColumn(gtx C) D {
	th := t.a.th
	s := t.sess
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return panel(gtx, th, "System operations", func(gtx C) D {
				ops := engine.OpsFor(s.Cur().Kind)
				if s.Range != engine.RangeContact {
					return smallLabel(th, "Operations are available at contact range.", colDim)(gtx)
				}
				if len(ops) == 0 {
					return smallLabel(th, "This node offers no operations.", colDim)(gtx)
				}
				children := []layout.FlexChild{}
				for i, op := range ops {
					if i >= len(t.sysops) {
						break
					}
					children = append(children, layout.Rigid(button(th, &t.sysops[i], fmt.Sprintf("%d. %s", i+1, op))), layout.Rigid(spacer(0, 2)))
				}
				children = append(children,
					layout.Rigid(smallLabel(th, "Arg (node # for Change Node, new name for Edit, 1-5 speed for Read):", colDim)),
					layout.Rigid(editor(th, &t.arg, "")),
				)
				if len(t.files.items) > 0 {
					children = append(children, layout.Rigid(smallLabel(th, "Files (select):", colDim)),
						layout.Rigid(fixedH(80, func(gtx C) D { return t.files.layout(gtx, th) })))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}),
		layout.Rigid(spacer(0, 4)),
		layout.Rigid(func(gtx C) D {
			return panel(gtx, th, "System warnings", func(gtx C) D {
				lamp := func(on bool, txt string, c color.NRGBA) layout.Widget {
					return func(gtx C) D {
						bg := colPanel2
						if on {
							bg = c
						}
						return fillBG(gtx, bg, func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.UniformInset(unit.Dp(2)).Layout(gtx, smallLabel(th, txt, colFg))
						})
					}
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(lamp(s.ActiveAlert, "Active alert", colRed)),
					layout.Rigid(spacer(0, 2)),
					layout.Rigid(lamp(s.Passive && !s.ActiveAlert, "Passive alert", colorWithAlpha(colAmber, 0xff))),
					layout.Rigid(spacer(0, 2)),
					layout.Rigid(lamp(s.ShutdownOn, fmt.Sprintf("Shutdown (%d)", s.ShutdownLeft), colRed)),
					layout.Rigid(spacer(0, 2)),
					layout.Rigid(lamp(s.Traced, "Entry point traced", colRed)),
				)
			})
		}),
		layout.Rigid(spacer(0, 4)),
		layout.Rigid(func(gtx C) D {
			return panel(gtx, th, "Condition monitor", func(gtx C) D {
				names := []string{"Deck", "Stun", "Phys"}
				rows := []layout.FlexChild{layout.Rigid(smallLabel(th, "           L   M   S   D", colDim))}
				for r := 0; r < 3; r++ {
					r := r
					rows = append(rows, layout.Rigid(func(gtx C) D {
						lvl := engine.Level(*t.condPtr(r))
						kids := []layout.FlexChild{layout.Rigid(fixedW(48, smallLabel(th, names[r], colFg)))}
						for k := 0; k < 4; k++ {
							txt := "_"
							bg := colBtn
							if lvl > k {
								txt, bg = "X", colRed
							}
							kids = append(kids, layout.Rigid(hotButton(th, &t.cond[r][k], txt, bg)), gap(2))
						}
						return row(gtx, kids...)
					}))
				}
				rows = append(rows, layout.Rigid(smallLabel(th, fmt.Sprintf("Wound modifier: +%d", s.WoundMod()), colAmber)))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			})
		}),
	)
}

func (t *terminal) centerColumn(gtx C) D {
	th := t.a.th
	s := t.sess
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return panel(gtx, th, "Vidscreen display", func(gtx C) D {
				w := gtx.Constraints.Max.X
				if m := gtx.Dp(unit.Dp(560)); w > m {
					w = m
				}
				h := w * 10 / 16
				gtx.Constraints = layout.Exact(image.Pt(w, h))
				return t.vidscreen(gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			react, nd := engine.Initiative(s.Decker, s.Deck, s.OnSatellite)
			ice := len(s.ICEHere())
			info := fmt.Sprintf("Range %d [%s]   ICE in sight: %d   Turn %d (%02d:%02d)   Pool %d   Karma %d   Initiative %d+%dD6",
				s.Range, engine.RangeName(s.Range), ice, s.Turn, s.Turn*3/60, s.Turn*3%60, s.Pool(), s.Karma, react, nd)
			return fillBG(gtx, colPanel, func(gtx C) D {
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, smallLabel(th, info, colCyan))
			})
		}),
		layout.Flexed(1, func(gtx C) D {
			return panel(gtx, th, "History log", func(gtx C) D {
				gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return material.List(th, &t.logList).Layout(gtx, len(s.Log), func(gtx C, i int) D {
							l := material.Caption(th, s.Log[i])
							l.Color = colAmber
							if strings.Contains(s.Log[i], "*") {
								l.Color = colRed
							}
							l.TextSize = unit.Sp(12)
							return l.Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return row(gtx,
							rigid(button(th, &t.saveLog, "Save log...")), gap(4),
							rigid(button(th, &t.saveDeck, "Save deck...")), gap(4),
							rigid(button(th, &t.newRun, "End / new run")))
					}),
				)
			})
		}),
	)
}

func (t *terminal) vidscreen(gtx C) D {
	s := t.sess
	lib := t.a.ws.Library
	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0x1a, B: 0, A: 0xff}, clip.Rect{Max: size}.Op())
	n := s.Cur()
	cands := assets.NodeSprite(n.Kind, s.Range, s.OnSatellite)
	if img, err := lib.Image(cands, false); err == nil {
		t.drawImage(gtx, img, image.Rectangle{Max: size})
	} else {
		l := material.H6(t.a.th, fmt.Sprintf("%s  (no image)", n.Kind))
		l.Color = colGreen
		layout.Center.Layout(gtx, l.Layout)
	}
	ices := s.ICEHere()
	mode := s.Deck.DisplayMode
	for i, ice := range ices {
		if i >= 5 {
			break
		}
		img, err := lib.Image(assets.ICESprite(ice, mode, ice.Known.Type, false), true)
		if err != nil {
			continue
		}
		scale := []float64{0.8, 0.55, 0.35}[s.Range]
		hh := int(float64(size.Y) * scale)
		ww := hh * img.Bounds().Dx() / max(1, img.Bounds().Dy())
		slotW := size.X / max(1, min(5, len(ices)))
		cx := slotW*i + slotW/2
		r := image.Rect(cx-ww/2, size.Y-hh-size.Y/12, cx+ww/2, size.Y-size.Y/12)
		t.drawImage(gtx, img, r)
	}
	if s.Over {
		if img, err := lib.Image([]string{"CRASH"}, false); err == nil && s.DeckBoxes >= 10 {
			t.drawImage(gtx, img, image.Rectangle{Max: size})
		}
		l := material.H5(t.a.th, "RUN OVER: "+strings.TrimPrefix(s.OverMsg, "*"))
		l.Color = colRed
		layout.Center.Layout(gtx, l.Layout)
	}
	return D{Size: size}
}

func (t *terminal) drawImage(gtx C, img *image.RGBA, dst image.Rectangle) {
	opx := t.imageOp(img)
	sx := float32(dst.Dx()) / float32(img.Bounds().Dx())
	sy := float32(dst.Dy()) / float32(img.Bounds().Dy())
	st := op.Offset(dst.Min).Push(gtx.Ops)
	aff := op.Affine(f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(sx, sy))).Push(gtx.Ops)
	opx.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	aff.Pop()
	st.Pop()
}

func (t *terminal) bottomRow(gtx C) D {
	th := t.a.th
	s := t.sess
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(fixedW(230, func(gtx C) D {
			return panel(gtx, th, "Net navigator", func(gtx C) D {
				ex := s.Exits()
				exTxt := "no exits"
				if len(ex) > 0 {
					sel := s.ExitSel % len(ex)
					exTxt = fmt.Sprintf("Exit %d of %d -> %s", sel+1, len(ex), s.NodeLabel(ex[sel]))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(smallLabel(th, "Node: "+s.NodeLabel(s.Node), colCyan)),
					layout.Rigid(smallLabel(th, exTxt, colFg)),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(func(gtx C) D {
						return row(gtx, rigid(button(th, &t.fwd, "Forward")), gap(4), rigid(button(th, &t.back, "Backward")))
					}),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(func(gtx C) D {
						return row(gtx, rigid(button(th, &t.left, "<")), gap(4), rigid(button(th, &t.right, ">")), gap(4), rigid(button(th, &t.wait, "Wait")))
					}),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(hotButton(th, &t.jack, "Jack out", colRed)),
				)
			})
		})),
		gap(4),
		layout.Flexed(1, func(gtx C) D {
			return panel(gtx, th, fmt.Sprintf("Active memory  %d/%d Mp", s.ActiveUsed(), s.Deck.ActiveMem), func(gtx C) D {
				return t.active.layout(gtx, th)
			})
		}),
		gap(4),
		layout.Rigid(fixedW(110, func(gtx C) D {
			return panel(gtx, th, "Programs", func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(hotButton(th, &t.exec, "Execute", colGreenDark)), layout.Rigid(spacer(0, 3)),
					layout.Rigid(button(th, &t.load, "< Load")), layout.Rigid(spacer(0, 3)),
					layout.Rigid(button(th, &t.unload, "Unload >")), layout.Rigid(spacer(0, 3)),
					layout.Rigid(button(th, &t.del, "Delete")))
			})
		})),
		gap(4),
		layout.Flexed(1, func(gtx C) D {
			return panel(gtx, th, fmt.Sprintf("Storage memory  %d/%d Mp", s.StorageUsed(), s.Deck.StorageMem), func(gtx C) D {
				return t.storage.layout(gtx, th)
			})
		}),
		gap(4),
		layout.Rigid(fixedW(250, func(gtx C) D {
			return panel(gtx, th, "Deck / dice", func(gtx C) D {
				p := s.Deck.Persona
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(smallLabel(th, p.Model, colCyan)),
					layout.Rigid(smallLabel(th, fmt.Sprintf("MPCP %d  Hardening %d  Resp %d", p.MPCP, s.Deck.Hardening, s.Deck.Response), colFg)),
					layout.Rigid(smallLabel(th, fmt.Sprintf("Bod %d  Eva %d  Mask %d  Sens %d", p.Bod, p.Evasion, p.Masking, p.Sensor), colFg)),
					layout.Rigid(smallLabel(th, "ICE target (select, default first):", colDim)),
					layout.Rigid(fixedH(44, func(gtx C) D { return t.ices.layout(gtx, th) })),
					layout.Rigid(smallLabel(th, "Pool dice per test (empty = all):", colDim)),
					layout.Rigid(editor(th, &t.poolEd, "all")),
					layout.Rigid(func(gtx C) D {
						return material.CheckBox(th, &t.manual, "Roll my own dice").Layout(gtx)
					}),
					layout.Rigid(editor(th, &t.diceEd, "e.g. 6 4 3 1 (used by next test)")),
				)
			})
		})),
	)
}

// ---- Auto map ----

func (t *terminal) mapWidget(gtx C) D {
	s := t.sess
	th := t.a.th
	size := gtx.Constraints.Max
	if size.Y < 100 {
		size.Y = gtx.Dp(unit.Dp(300))
	}
	size.Y = max(size.Y, gtx.Dp(unit.Dp(340)))
	if gtx.Constraints.Max.Y > 0 && gtx.Constraints.Max.Y < 1<<20 {
		size.Y = gtx.Constraints.Max.Y
	}
	gtx.Constraints = layout.Exact(size)

	paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0x10, B: 0, A: 0xff}, clip.Rect{Max: size}.Op())

	vis := map[int]bool{}
	for id := range s.Visited {
		vis[id] = true
		if n := s.Matrix.Nodes[id]; n != nil {
			for _, l := range n.Links {
				if l != 0 && s.Matrix.Nodes[l] != nil {
					vis[l] = true
				}
			}
		}
	}
	vis[s.Node] = true
	t.mapPos = map[int]image.Point{}
	if len(vis) == 0 {
		return D{Size: size}
	}
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1<<30, -1<<30
	for id := range vis {
		n := s.Matrix.Nodes[id]
		minX, maxX = min(minX, n.X), max(maxX, n.X)
		minY, maxY = min(minY, n.Y), max(maxY, n.Y)
	}
	cw := float64(size.X) / float64(maxX-minX+2)
	ch := float64(size.Y) / float64(maxY-minY+2)
	cell := math.Min(math.Min(cw, ch), float64(gtx.Dp(unit.Dp(60))))
	offX := (float64(size.X) - cell*float64(maxX-minX)) / 2
	offY := (float64(size.Y) - cell*float64(maxY-minY)) / 2
	for id := range vis {
		n := s.Matrix.Nodes[id]
		t.mapPos[id] = image.Pt(int(offX+cell*float64(n.X-minX)), int(offY+cell*float64(n.Y-minY)))
	}
	// Datalines.
	for id := range s.Visited {
		n := s.Matrix.Nodes[id]
		if n == nil {
			continue
		}
		for _, l := range n.Links {
			if a, ok := t.mapPos[id]; ok {
				if b, ok := t.mapPos[l]; ok && (id < l || !s.Visited[l]) {
					strokeLine(gtx, f32.Pt(float32(a.X), float32(a.Y)), f32.Pt(float32(b.X), float32(b.Y)), color.NRGBA{R: 0, G: 0x9a, B: 0, A: 0xff}, 2)
				}
			}
		}
	}
	r := float32(math.Max(10, cell*0.32))
	ids := make([]int, 0, len(vis))
	for id := range vis {
		ids = append(ids, id)
	}
	for _, id := range ids {
		n := s.Matrix.Nodes[id]
		p := t.mapPos[id]
		c := f32.Pt(float32(p.X), float32(p.Y))
		k := s.Know[id]
		known := k != nil && k.Type
		if !known {
			diamond(gtx, c, r, color.NRGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xff})
			t.drawText(gtx, p, "?", colFg)
		} else {
			col := nodeColor(n, k)
			drawShape(gtx, n.Kind, c, r, col)
			txt := ""
			if k.Rating && n.Kind != model.LNK {
				txt = strconv.Itoa(n.Rating)
			}
			t.drawText(gtx, p, txt, color.NRGBA{A: 0xff})
		}
		if id == s.Node {
			ring(gtx, c, r+5, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
		}
		t.drawText(gtx, image.Pt(p.X, p.Y+int(r)+8), "#"+strconv.Itoa(id), colDim)
	}
	// Click handling.
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &t.mapTag)
	area.Pop()
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &t.mapTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := ev.(pointer.Event); ok && pe.Kind == pointer.Press {
			t.mapClick(pe.Position, float64(r)+6)
		}
	}
	_ = th
	return D{Size: size}
}

func (t *terminal) mapClick(pos f32.Point, radius float64) {
	best, bd := -1, radius
	for id, p := range t.mapPos {
		d := math.Hypot(float64(pos.X)-float64(p.X), float64(pos.Y)-float64(p.Y))
		if d <= bd {
			best, bd = id, d
		}
	}
	if best < 0 {
		return
	}
	s := t.sess
	for i, e := range s.Exits() {
		if e == best {
			s.ExitSel = i
			t.a.setStatus(fmt.Sprintf("Exit %d selected -> node %d", i+1, best))
			return
		}
	}
	if best == s.Node {
		t.a.setStatus("You are here: " + s.NodeLabel(best))
	} else {
		t.a.setStatus(fmt.Sprintf("Node %d is not directly connected to the current node.", best))
	}
}

func (t *terminal) drawText(gtx C, at image.Point, s string, c color.NRGBA) {
	if s == "" {
		return
	}
	l := material.Caption(t.a.th, s)
	l.Color = c
	l.TextSize = unit.Sp(11)
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(200, 40)}
	dims := l.Layout(g)
	call := m.Stop()
	st := op.Offset(image.Pt(at.X-dims.Size.X/2, at.Y-dims.Size.Y/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	st.Pop()
}

func nodeColor(n *model.Node, k *engine.NodeKnow) color.NRGBA {
	if !k.Color || n.Kind == model.LNK {
		return color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}
	}
	switch n.Color {
	case model.Blue:
		return color.NRGBA{R: 0x30, G: 0x90, B: 0xff, A: 0xff}
	case model.Green:
		return color.NRGBA{R: 0x20, G: 0xc0, B: 0x30, A: 0xff}
	case model.Orange:
		return color.NRGBA{R: 0xff, G: 0x90, B: 0x20, A: 0xff}
	case model.Red:
		return color.NRGBA{R: 0xe0, G: 0x30, B: 0x30, A: 0xff}
	case model.Purple:
		return color.NRGBA{R: 0xb0, G: 0x40, B: 0xe0, A: 0xff}
	}
	return color.NRGBA{R: 0x50, G: 0x50, B: 0x50, A: 0xff}
}

func drawShape(gtx C, kind string, c f32.Point, r float32, col color.NRGBA) {
	switch kind {
	case model.CPU:
		hexagon(gtx, c, r+3, col)
		hexagon(gtx, c, r-3, color.NRGBA{R: col.R / 2, G: col.G / 2, B: col.B / 2, A: 0xff})
	case model.SPU:
		hexagon(gtx, c, r, col)
	case model.SAN:
		fillPoly(gtx, []f32.Point{{X: c.X - r*1.3, Y: c.Y - r*0.8}, {X: c.X + r*1.3, Y: c.Y - r*0.8}, {X: c.X + r*1.3, Y: c.Y + r*0.8}, {X: c.X - r*1.3, Y: c.Y + r*0.8}}, col)
	case model.DS:
		fillPoly(gtx, []f32.Point{{X: c.X - r, Y: c.Y - r}, {X: c.X + r, Y: c.Y - r}, {X: c.X + r, Y: c.Y + r}, {X: c.X - r, Y: c.Y + r}}, col)
	case model.IOP:
		fillPoly(gtx, []f32.Point{{X: c.X, Y: c.Y - r}, {X: c.X + r, Y: c.Y + r*0.8}, {X: c.X - r, Y: c.Y + r*0.8}}, col)
	case model.LNK:
		diamond(gtx, c, r, col)
	default:
		disc(gtx, c, r, col)
	}
}

func hexagon(gtx C, c f32.Point, r float32, col color.NRGBA) {
	pts := make([]f32.Point, 6)
	for i := range pts {
		a := float64(i) * math.Pi / 3
		pts[i] = f32.Pt(c.X+r*float32(math.Cos(a)), c.Y+r*float32(math.Sin(a)))
	}
	fillPoly(gtx, pts, col)
}

func diamond(gtx C, c f32.Point, r float32, col color.NRGBA) {
	fillPoly(gtx, []f32.Point{{X: c.X, Y: c.Y - r}, {X: c.X + r, Y: c.Y}, {X: c.X, Y: c.Y + r}, {X: c.X - r, Y: c.Y}}, col)
}

func fillPoly(gtx C, pts []f32.Point, col color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(pts[0])
	for _, q := range pts[1:] {
		p.LineTo(q)
	}
	p.Close()
	paint.FillShape(gtx.Ops, col, clip.Outline{Path: p.End()}.Op())
}

func disc(gtx C, c f32.Point, r float32, col color.NRGBA) {
	rect := image.Rect(int(c.X-r), int(c.Y-r), int(c.X+r), int(c.Y+r))
	paint.FillShape(gtx.Ops, col, clip.Ellipse(rect).Op(gtx.Ops))
}

func ring(gtx C, c f32.Point, r float32, col color.NRGBA) {
	rect := image.Rect(int(c.X-r), int(c.Y-r), int(c.X+r), int(c.Y+r))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: clip.Ellipse(rect).Path(gtx.Ops), Width: 2}.Op())
}

func strokeLine(gtx C, a, b f32.Point, col color.NRGBA, w float32) {
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(a)
	p.LineTo(b)
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: p.End(), Width: w}.Op())
}
