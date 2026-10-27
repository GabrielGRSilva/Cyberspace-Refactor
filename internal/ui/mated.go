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

	"cyberspace-refactor/internal/engine"
	"cyberspace-refactor/internal/format"
	"cyberspace-refactor/internal/model"
)

var (
	allNodeKinds  = [...]string{model.SAN, model.SPU, model.DS, model.SM, model.IOP, model.CPU, model.DLJ, model.LNK, model.MAT}
	allNodeColors = [...]string{model.Blue, model.Green, model.Orange, model.Red, model.Purple, model.Dark}
	allICEClasses = [...]string{"White", "Gray", "Black"}
	allICETypes   = []string{
		"Access", "Barrier", "Scramble", "Probe", "Killer", "Blaster",
		"Acid", "Binder", "Jammer", "Marker", "Tar Baby", "Tar Pit",
		"Trace and Report", "Trace and Dump", "Trace and Burn",
	}
	allICETriggers = []string{"", model.TrigEver, model.TrigPassive, model.TrigActive, model.TrigTrigger}
	allICEMobs     = []string{"", model.MobImmobile, model.MobMobile, model.MobPMobile, model.MobAMobile}
	allAlerts      = []string{"none", "passive", "active"}
)

type matedUI struct {
	a      *App
	matrix *model.Matrix
	rel    string
	selID  int
	mode   int // 0 select/add, 1 toggle link from selID, 2 move selID to clicked cell

	// Toolbar buttons
	newBtn, openBtn, saveBtn, jsonBtn, valBtn, fixBtn widget.Clickable
	modeBtns                                          [3]widget.Clickable

	// Global matrix editors
	descEd, entryEd, shutTicsEd, shutPctEd, imgPathEd widget.Editor
	rangeBtn, alertBtn, applyGlobalBtn                widget.Clickable

	// Selected node editors
	kindBtns                        [len(allNodeKinds)]widget.Clickable
	colorBtns                       [len(allNodeColors)]widget.Clickable
	ratMinus, ratPlus, setEntryBtn  widget.Clickable
	delNodeBtn, applyNodeBtn        widget.Clickable
	msgEd, lnkFileEd, lnkEntryEd    widget.Editor

	// Files editor (for DS nodes)
	fileList                        *listBox
	fNameEd, fSizeEd, fValEd, fPwEd widget.Editor
	addFileBtn, delFileBtn          widget.Clickable

	// ICE editor
	iceList                                            *listBox
	iceClassIdx, iceTypeIdx, iceTrigIdx, iceMobIdx     int
	iceClassBtn, iceTypeBtn, iceTrigBtn, iceMobBtn     widget.Clickable
	iceRatEd, iceStageEd                               widget.Editor
	addICEBtn, delICEBtn                               widget.Clickable

	// Validation messages
	valIssues []string

	// Canvas state
	canvasTag  bool
	gridW      int
	gridH      int
	cellPx     float64
	originX    float64
	originY    float64
}

func newMatedUI(a *App) *matedUI {
	u := &matedUI{
		a:        a,
		matrix:   defaultMatrix(),
		selID:    0,
		fileList: newListBox(),
		iceList:  newListBox(),
	}
	for _, ed := range []*widget.Editor{
		&u.descEd, &u.entryEd, &u.shutTicsEd, &u.shutPctEd, &u.imgPathEd,
		&u.msgEd, &u.lnkFileEd, &u.lnkEntryEd,
		&u.fNameEd, &u.fSizeEd, &u.fValEd, &u.fPwEd,
		&u.iceRatEd, &u.iceStageEd,
	} {
		ed.SingleLine = true
	}
	u.iceRatEd.SetText("4")
	u.iceStageEd.SetText("M")
	u.fSizeEd.SetText("50")
	u.fValEd.SetText("1000")
	u.syncFromMatrix()
	u.syncFromNode()
	return u
}

func defaultMatrix() *model.Matrix {
	m := model.NewMatrix()
	m.Description = "Custom Matrix Grid"
	m.Entry = 0
	m.Range = 2
	m.Alert = "none"
	m.Nodes[0] = &model.Node{ID: 0, Kind: model.SAN, Color: model.Blue, Rating: 3, X: 1, Y: 4, Links: []int{1}}
	m.Nodes[1] = &model.Node{ID: 1, Kind: model.CPU, Color: model.Green, Rating: 4, X: 4, Y: 4, Links: []int{0}}
	return m
}

func (u *matedUI) syncFromMatrix() {
	m := u.matrix
	u.descEd.SetText(m.Description)
	u.entryEd.SetText(strconv.Itoa(m.Entry))
	u.shutTicsEd.SetText(strconv.Itoa(m.ShutdownTics))
	u.shutPctEd.SetText(strconv.Itoa(m.ShutdownPct))
	u.imgPathEd.SetText(m.ImagePath)
}

func (u *matedUI) applyGlobal() {
	m := u.matrix
	m.Description = strings.TrimSpace(u.descEd.Text())
	if v, err := strconv.Atoi(strings.TrimSpace(u.entryEd.Text())); err == nil && v >= 0 && v <= model.MaxNodes {
		m.Entry = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(u.shutTicsEd.Text())); err == nil && v >= 0 {
		m.ShutdownTics = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(u.shutPctEd.Text())); err == nil && v >= 0 && v <= 100 {
		m.ShutdownPct = v
	}
	m.ImagePath = strings.TrimSpace(u.imgPathEd.Text())
}

func (u *matedUI) syncFromNode() {
	n := u.matrix.Nodes[u.selID]
	if n == nil {
		u.msgEd.SetText("")
		u.lnkFileEd.SetText("")
		u.lnkEntryEd.SetText("0")
		u.fileList.Set(nil)
		u.iceList.Set(nil)
		return
	}
	u.msgEd.SetText(n.Message)
	u.lnkFileEd.SetText(n.LinkFile)
	u.lnkEntryEd.SetText(strconv.Itoa(n.LinkEntry))
	u.refreshNodeLists()
}

func (u *matedUI) refreshNodeLists() {
	n := u.matrix.Nodes[u.selID]
	if n == nil {
		u.fileList.Set(nil)
		u.iceList.Set(nil)
		return
	}
	var fl []string
	for i, f := range n.Files {
		pw := ""
		if f.Password != "" {
			pw = " [pw:" + f.Password + "]"
		}
		fl = append(fl, fmt.Sprintf("%d. %s (%dMp, %d¥)%s", i+1, f.Name, f.Size, f.Value, pw))
	}
	u.fileList.Set(fl)

	var il []string
	for i, ic := range n.ICE {
		st := ""
		if ic.Stage != "" {
			st = "[" + ic.Stage + "]"
		}
		tr := ic.Trigger
		if tr == "" {
			tr = "def"
		}
		mb := ic.Mobility
		if mb == "" {
			mb = "def"
		}
		il = append(il, fmt.Sprintf("%d. %s %s%s-%d (%s/%s)", i+1, ic.Class, ic.Type, st, ic.Rating, tr, mb))
	}
	u.iceList.Set(il)
}

func (u *matedUI) applyNodeText() {
	n := u.matrix.Nodes[u.selID]
	if n == nil {
		return
	}
	n.Message = strings.TrimSpace(u.msgEd.Text())
	n.LinkFile = strings.TrimSpace(u.lnkFileEd.Text())
	if v, err := strconv.Atoi(strings.TrimSpace(u.lnkEntryEd.Text())); err == nil && v >= 0 {
		n.LinkEntry = v
	}
}

func (u *matedUI) layout(gtx C) D {
	u.handleActions(gtx)
	th := u.a.th
	return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(u.toolbar),
			layout.Rigid(spacer(0, 4)),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(fixedW(300, u.leftPanel)),
					gap(4),
					layout.Flexed(1, func(gtx C) D {
						title := "Matrix Topology Grid (Click cell: select/create | Mode buttons: Link or Move)"
						return panel(gtx, th, title, u.gridCanvas)
					}),
					gap(4),
					layout.Rigid(fixedW(360, u.rightPanel)),
				)
			}),
		)
	})
}

func (u *matedUI) toolbar(gtx C) D {
	th := u.a.th
	modes := [3]string{"Mode: Select / Create Node", "Mode: Link / Unlink Dataline", "Mode: Move Selected Node"}
	modeWidgets := make([]layout.FlexChild, 0, 6)
	for i := range modes {
		i := i
		bg := colBtn
		if u.mode == i {
			bg = colGreenDark
		}
		modeWidgets = append(modeWidgets, rigid(hotButton(th, &u.modeBtns[i], modes[i], bg)), gap(4))
	}
	children := []layout.FlexChild{
		rigid(button(th, &u.newBtn, "New Matrix")),
		gap(4),
		rigid(button(th, &u.openBtn, "Open .LTG/.JSON")),
		gap(4),
		rigid(hotButton(th, &u.saveBtn, "Save .LTG", colGreenDark)),
		gap(4),
		rigid(button(th, &u.jsonBtn, "Export .JSON")),
		gap(4),
		rigid(button(th, &u.fixBtn, "Repair Links")),
		gap(4),
		rigid(button(th, &u.valBtn, "Validate Rules")),
		gap(12),
	}
	children = append(children, modeWidgets...)
	return fillBG(gtx, colPanel, func(gtx C) D {
		return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
			return row(gtx, children...)
		})
	})
}

func (u *matedUI) leftPanel(gtx C) D {
	th := u.a.th
	rngLabels := map[int]string{2: "2 (Observation)", 1: "1 (Sensor)", 0: "0 (Contact)"}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return panel(gtx, th, "Global Matrix Settings", func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(smallLabel(th, "Description:", colDim)),
					layout.Rigid(editor(th, &u.descEd, "Matrix description")),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(func(gtx C) D {
						return row(gtx,
							rigid(smallLabel(th, "Entry ID:", colDim)), gap(4),
							layout.Flexed(1, editor(th, &u.entryEd, "0")), gap(6),
							rigid(button(th, &u.rangeBtn, "Range: "+rngLabels[u.matrix.Range])),
						)
					}),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(func(gtx C) D {
						return row(gtx,
							rigid(button(th, &u.alertBtn, "Alert: "+u.matrix.Alert)), gap(6),
							rigid(smallLabel(th, "Tics:", colDim)), gap(2),
							layout.Flexed(1, editor(th, &u.shutTicsEd, "0")), gap(4),
							rigid(smallLabel(th, "Emerg%:", colDim)), gap(2),
							layout.Flexed(1, editor(th, &u.shutPctEd, "0")),
						)
					}),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(smallLabel(th, "Custom Image Folder (optional):", colDim)),
					layout.Rigid(editor(th, &u.imgPathEd, "IMAGES")),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(hotButton(th, &u.applyGlobalBtn, "Apply Global Settings", colGreenDark)),
				)
			})
		}),
		layout.Rigid(spacer(0, 4)),
		layout.Flexed(1, func(gtx C) D {
			return panel(gtx, th, "Consistency & Rule Validator", func(gtx C) D {
				if len(u.valIssues) == 0 {
					return smallLabel(th, "Click 'Validate Rules' to check NODELIM constraints, reachability, entry SAN, and reciprocal links.", colGreen)(gtx)
				}
				var ch []layout.FlexChild
				for _, msg := range u.valIssues {
					col := colAmber
					if strings.HasPrefix(msg, "OK:") {
						col = colGreen
					}
					ch = append(ch, layout.Rigid(smallLabel(th, "• "+msg, col)), layout.Rigid(spacer(0, 2)))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, ch...)
			})
		}),
	)
}

func (u *matedUI) rightPanel(gtx C) D {
	th := u.a.th
	n := u.matrix.Nodes[u.selID]
	if n == nil {
		return panel(gtx, th, "Node Inspector", smallLabel(th, "No node selected. Click any grid cell to create or select a node.", colDim))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			title := fmt.Sprintf("Node #%d (%s %s-%d @ %d,%d)", n.ID, n.Kind, n.Color, n.Rating, n.X, n.Y)
			return panel(gtx, th, title, func(gtx C) D {
				// Kind selector row 1 & 2
				var kindRow1, kindRow2 []layout.FlexChild
				for i, k := range allNodeKinds {
					i, k := i, k
					bg := colBtn
					if n.Kind == k {
						bg = colGreenDark
					}
					btn := rigid(hotButton(th, &u.kindBtns[i], k, bg))
					if i < 5 {
						kindRow1 = append(kindRow1, btn, gap(3))
					} else {
						kindRow2 = append(kindRow2, btn, gap(3))
					}
				}
				var colRow []layout.FlexChild
				for i, c := range allNodeColors {
					i, c := i, c
					bg := colBtn
					if n.Color == c {
						bg = colGreenDark
					}
					colRow = append(colRow, rigid(hotButton(th, &u.colorBtns[i], c[:3], bg)), gap(3))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return row(gtx, kindRow1...) }),
					layout.Rigid(spacer(0, 3)),
					layout.Rigid(func(gtx C) D { return row(gtx, kindRow2...) }),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(func(gtx C) D { return row(gtx, colRow...) }),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(func(gtx C) D {
						return row(gtx,
							rigid(smallLabel(th, fmt.Sprintf("Rating: %d", n.Rating), colFg)),
							gap(4),
							rigid(button(th, &u.ratMinus, "-")),
							gap(2),
							rigid(button(th, &u.ratPlus, "+")),
							gap(8),
							rigid(button(th, &u.setEntryBtn, "Set Entry")),
							gap(4),
							rigid(hotButton(th, &u.delNodeBtn, "Delete", colRed)),
						)
					}),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(smallLabel(th, "Node Message (Mesg=):", colDim)),
					layout.Rigid(editor(th, &u.msgEd, "Text shown on arrival / Inquiry")),
					layout.Rigid(func(gtx C) D {
						if n.Kind != model.LNK {
							return D{}
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(spacer(0, 3)),
							layout.Rigid(smallLabel(th, "LNK Target (.ltg file & entry node):", colAmber)),
							layout.Rigid(func(gtx C) D {
								return row(gtx,
									layout.Flexed(1, editor(th, &u.lnkFileEd, "OTHER.LTG")),
									gap(4),
									rigid(fixedW(50, editor(th, &u.lnkEntryEd, "0"))),
								)
							}),
						)
					}),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(hotButton(th, &u.applyNodeBtn, "Save Node Text / LNK", colGreenDark)),
				)
			})
		}),
		layout.Rigid(spacer(0, 4)),
		layout.Flexed(1, func(gtx C) D {
			return panel(gtx, th, fmt.Sprintf("Installed ICE (%d)", len(n.ICE)), func(gtx C) D {
				trName := allICETriggers[u.iceTrigIdx]
				if trName == "" {
					trName = "default"
				}
				mbName := allICEMobs[u.iceMobIdx]
				if mbName == "" {
					mbName = "default"
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(fixedH(95, func(gtx C) D { return u.iceList.layout(gtx, th) })),
					layout.Rigid(spacer(0, 4)),
					layout.Rigid(func(gtx C) D {
						return row(gtx,
							rigid(button(th, &u.iceClassBtn, allICEClasses[u.iceClassIdx])),
							gap(3),
							rigid(button(th, &u.iceTypeBtn, allICETypes[u.iceTypeIdx])),
							gap(3),
							rigid(smallLabel(th, "R:", colDim)),
							rigid(fixedW(36, editor(th, &u.iceRatEd, "4"))),
							gap(3),
							rigid(smallLabel(th, "Stg:", colDim)),
							rigid(fixedW(48, editor(th, &u.iceStageEd, "M"))),
						)
					}),
					layout.Rigid(spacer(0, 3)),
					layout.Rigid(func(gtx C) D {
						return row(gtx,
							rigid(button(th, &u.iceTrigBtn, "Trig:"+trName)),
							gap(3),
							rigid(button(th, &u.iceMobBtn, "Mob:"+mbName)),
							gap(4),
							rigid(hotButton(th, &u.addICEBtn, "+ICE", colGreenDark)),
							gap(3),
							rigid(button(th, &u.delICEBtn, "-ICE")),
						)
					}),
				)
			})
		}),
		layout.Rigid(spacer(0, 4)),
		layout.Rigid(func(gtx C) D {
			if n.Kind != model.DS {
				return D{}
			}
			return panel(gtx, th, fmt.Sprintf("Datastore Files (%d)", len(n.Files)), func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(fixedH(72, func(gtx C) D { return u.fileList.layout(gtx, th) })),
					layout.Rigid(spacer(0, 3)),
					layout.Rigid(func(gtx C) D {
						return row(gtx,
							layout.Flexed(1, editor(th, &u.fNameEd, "Paydata")),
							gap(3),
							rigid(fixedW(48, editor(th, &u.fSizeEd, "50"))),
							gap(3),
							rigid(fixedW(60, editor(th, &u.fValEd, "1000"))),
							gap(3),
							rigid(fixedW(58, editor(th, &u.fPwEd, "pw"))),
						)
					}),
					layout.Rigid(spacer(0, 3)),
					layout.Rigid(func(gtx C) D {
						return row(gtx,
							rigid(hotButton(th, &u.addFileBtn, "+Add File", colGreenDark)),
							gap(4),
							rigid(button(th, &u.delFileBtn, "-Remove File")),
						)
					}),
				)
			})
		}),
	)
}

func (u *matedUI) gridCanvas(gtx C) D {
	size := gtx.Constraints.Max
	if size.Y < 200 {
		size.Y = 480
	}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0x07, G: 0x0b, B: 0x0f, A: 0xff}, clip.Rect{Max: size}.Op())

	// Determine bounding grid size (minimum 12x10).
	gw, gh := 12, 10
	for _, n := range u.matrix.Nodes {
		if n.X+2 > gw {
			gw = n.X + 2
		}
		if n.Y+2 > gh {
			gh = n.Y + 2
		}
	}
	u.gridW, u.gridH = gw, gh
	pad := 28.0
	cw := (float64(size.X) - 2*pad) / float64(max(1, gw-1))
	ch := (float64(size.Y) - 2*pad) / float64(max(1, gh-1))
	cell := math.Min(cw, ch)
	if cell < 22 {
		cell = 22
	}
	u.cellPx = cell
	u.originX = (float64(size.X) - cell*float64(gw-1)) / 2
	u.originY = (float64(size.Y) - cell*float64(gh-1)) / 2

	ptFor := func(x, y int) f32.Point {
		return f32.Pt(float32(u.originX+float64(x)*cell), float32(u.originY+float64(y)*cell))
	}

	// Draw subtle grid dots.
	dotCol := color.NRGBA{R: 0x22, G: 0x32, B: 0x3a, A: 0xff}
	for gx := 0; gx < gw; gx++ {
		for gy := 0; gy < gh; gy++ {
			p := ptFor(gx, gy)
			disc(gtx, p, 2, dotCol)
		}
	}

	// Draw datalines.
	for _, id := range u.matrix.IDs() {
		n := u.matrix.Nodes[id]
		a := ptFor(n.X, n.Y)
		for _, nb := range n.Links {
			if bNode := u.matrix.Nodes[nb]; bNode != nil && id < nb {
				b := ptFor(bNode.X, bNode.Y)
				strokeLine(gtx, a, b, color.NRGBA{R: 0x28, G: 0xb8, B: 0x55, A: 0xff}, 2)
			}
		}
	}

	// Draw nodes.
	r := float32(math.Max(10, cell*0.30))
	allKnow := &engine.NodeKnow{Type: true, Color: true, Rating: true}
	for _, id := range u.matrix.IDs() {
		n := u.matrix.Nodes[id]
		c := ptFor(n.X, n.Y)
		col := nodeColor(n, allKnow)
		drawShape(gtx, n.Kind, c, r, col)
		if id == u.matrix.Entry {
			ring(gtx, c, r+7, colAmber)
		}
		if id == u.selID {
			ring(gtx, c, r+4, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
		}
		u.drawCanvasText(gtx, image.Pt(int(c.X), int(c.Y)), strconv.Itoa(n.Rating), color.NRGBA{A: 0xff})
		lbl := fmt.Sprintf("#%d %s", id, n.Kind)
		if len(n.ICE) > 0 {
			lbl += fmt.Sprintf("(%d)", len(n.ICE))
		}
		u.drawCanvasText(gtx, image.Pt(int(c.X), int(c.Y+r+9)), lbl, colFg)
	}

	// Pointer input on grid.
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &u.canvasTag)
	area.Pop()
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &u.canvasTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := ev.(pointer.Event); ok && pe.Kind == pointer.Press {
			u.onCanvasClick(pe.Position)
		}
	}
	return D{Size: size}
}

func (u *matedUI) drawCanvasText(gtx C, at image.Point, s string, c color.NRGBA) {
	if s == "" {
		return
	}
	l := material.Caption(u.a.th, s)
	l.Color = c
	l.TextSize = unit.Sp(10)
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(180, 30)}
	dims := l.Layout(g)
	call := m.Stop()
	st := op.Offset(image.Pt(at.X-dims.Size.X/2, at.Y-dims.Size.Y/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	st.Pop()
}

func (u *matedUI) onCanvasClick(pos f32.Point) {
	if u.cellPx <= 0 {
		return
	}
	gx := int(math.Round((float64(pos.X) - u.originX) / u.cellPx))
	gy := int(math.Round((float64(pos.Y) - u.originY) / u.cellPx))
	if gx < 0 || gy < 0 || gx >= u.gridW || gy >= u.gridH {
		return
	}
	// Find node at (gx, gy).
	hitID := -1
	for _, id := range u.matrix.IDs() {
		n := u.matrix.Nodes[id]
		if n.X == gx && n.Y == gy {
			hitID = id
			break
		}
	}

	switch u.mode {
	case 0: // Select or create node
		if hitID >= 0 {
			u.applyNodeText()
			u.selID = hitID
			u.syncFromNode()
			u.a.setStatus(fmt.Sprintf("Selected node #%d", hitID))
			return
		}
		// Create new node at (gx, gy) with lowest unused ID.
		newID := -1
		for id := 0; id <= model.MaxNodes; id++ {
			if u.matrix.Nodes[id] == nil {
				newID = id
				break
			}
		}
		if newID < 0 {
			u.a.setStatus("Maximum node count (101) reached.")
			return
		}
		nd := &model.Node{ID: newID, Kind: model.SPU, Color: model.Green, Rating: 3, X: gx, Y: gy}
		u.matrix.Nodes[newID] = nd
		if prev := u.matrix.Nodes[u.selID]; prev != nil && len(prev.Links) < model.MaxLinksPerNode {
			prev.Links = append(prev.Links, newID)
			nd.Links = append(nd.Links, prev.ID)
		}
		u.selID = newID
		u.syncFromNode()
		u.a.setStatus(fmt.Sprintf("Created node #%d at (%d,%d)", newID, gx, gy))

	case 1: // Link / Unlink from selID
		if hitID < 0 || hitID == u.selID {
			return
		}
		a := u.matrix.Nodes[u.selID]
		b := u.matrix.Nodes[hitID]
		if a == nil || b == nil {
			return
		}
		if a.Linked(hitID) {
			a.Links = removeInt(a.Links, hitID)
			b.Links = removeInt(b.Links, u.selID)
			u.a.setStatus(fmt.Sprintf("Removed dataline between #%d and #%d", u.selID, hitID))
		} else {
			a.Links = append(a.Links, hitID)
			if !b.Linked(u.selID) {
				b.Links = append(b.Links, u.selID)
			}
			u.a.setStatus(fmt.Sprintf("Connected #%d <-> #%d", u.selID, hitID))
		}

	case 2: // Move selected node
		if hitID >= 0 {
			u.a.setStatus("Target cell is already occupied by another node.")
			return
		}
		if n := u.matrix.Nodes[u.selID]; n != nil {
			n.X, n.Y = gx, gy
			u.a.setStatus(fmt.Sprintf("Moved node #%d to (%d,%d)", u.selID, gx, gy))
		}
	}
}

func removeInt(s []int, v int) []int {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func (u *matedUI) handleActions(gtx C) {
	for i := range u.modeBtns {
		if u.modeBtns[i].Clicked(gtx) {
			u.mode = i
		}
	}
	if u.newBtn.Clicked(gtx) {
		u.matrix = defaultMatrix()
		u.rel = ""
		u.selID = 0
		u.valIssues = nil
		u.syncFromMatrix()
		u.syncFromNode()
		u.a.setStatus("Created new Matrix grid.")
	}
	if u.openBtn.Clicked(gtx) {
		u.a.pick.show(u.a.ws, "Open Matrix (.ltg / .json)", false, []string{".ltg", ".json"}, "", func(rel string) {
			m, w, err := u.a.ws.LoadMatrix(rel)
			if err != nil {
				u.a.setStatus("Open error: " + err.Error())
				return
			}
			u.matrix, u.rel = m, rel
			u.selID = m.Entry
			if u.matrix.Nodes[u.selID] == nil {
				if ids := m.IDs(); len(ids) > 0 {
					u.selID = ids[0]
				}
			}
			u.valIssues = w
			u.syncFromMatrix()
			u.syncFromNode()
			u.a.setStatus("Loaded " + rel)
		})
	}
	if u.saveBtn.Clicked(gtx) {
		u.applyGlobal()
		u.applyNodeText()
		format.FixLinks(u.matrix)
		def := u.rel
		if def == "" || strings.HasSuffix(strings.ToLower(def), ".json") {
			def = "DATA/custom.ltg"
		}
		u.a.pick.show(u.a.ws, "Save Matrix (.ltg)", true, []string{".ltg"}, def, func(rel string) {
			if err := u.a.ws.Save(rel, format.WriteLTG(u.matrix)); err != nil {
				u.a.setStatus("Save failed: " + err.Error())
			} else {
				u.rel = rel
				u.a.setStatus("Saved Matrix to " + rel)
			}
		})
	}
	if u.jsonBtn.Clicked(gtx) {
		u.applyGlobal()
		u.applyNodeText()
		format.FixLinks(u.matrix)
		def := "DATA/custom.json"
		if u.rel != "" {
			ext := filepath.Ext(u.rel)
			def = strings.TrimSuffix(u.rel, ext) + ".json"
		}
		u.a.pick.show(u.a.ws, "Export Matrix as JSON (.json)", true, []string{".json"}, def, func(rel string) {
			b, err := format.MatrixToJSON(u.matrix)
			if err != nil {
				u.a.setStatus("JSON error: " + err.Error())
				return
			}
			if err := u.a.ws.Save(rel, b); err != nil {
				u.a.setStatus("Export failed: " + err.Error())
			} else {
				u.a.setStatus("Exported JSON to " + rel)
			}
		})
	}
	if u.fixBtn.Clicked(gtx) {
		n := format.FixLinks(u.matrix)
		u.a.setStatus(fmt.Sprintf("Repaired %d one-way/invalid dataline link(s).", len(n)))
	}
	if u.valBtn.Clicked(gtx) {
		u.applyGlobal()
		u.applyNodeText()
		u.valIssues = validateMatrix(u.matrix)
		u.a.setStatus("Matrix validation complete.")
	}
	if u.rangeBtn.Clicked(gtx) {
		u.matrix.Range = (u.matrix.Range + 2) % 3
	}
	if u.alertBtn.Clicked(gtx) {
		for i, a := range allAlerts {
			if a == u.matrix.Alert {
				u.matrix.Alert = allAlerts[(i+1)%len(allAlerts)]
				break
			}
		}
	}
	if u.applyGlobalBtn.Clicked(gtx) {
		u.applyGlobal()
		u.a.setStatus("Global matrix settings updated.")
	}

	n := u.matrix.Nodes[u.selID]
	if n == nil {
		return
	}
	for i, k := range allNodeKinds {
		if u.kindBtns[i].Clicked(gtx) {
			n.Kind = k
		}
	}
	for i, c := range allNodeColors {
		if u.colorBtns[i].Clicked(gtx) {
			n.Color = c
		}
	}
	if u.ratMinus.Clicked(gtx) && n.Rating > 1 {
		n.Rating--
	}
	if u.ratPlus.Clicked(gtx) && n.Rating < 20 {
		n.Rating++
	}
	if u.setEntryBtn.Clicked(gtx) {
		u.matrix.Entry = n.ID
		u.entryEd.SetText(strconv.Itoa(n.ID))
		u.a.setStatus(fmt.Sprintf("Node #%d set as Matrix Entry.", n.ID))
	}
	if u.delNodeBtn.Clicked(gtx) {
		if len(u.matrix.Nodes) <= 1 {
			u.a.setStatus("Cannot delete the only node in the Matrix.")
			return
		}
		delID := n.ID
		delete(u.matrix.Nodes, delID)
		for _, other := range u.matrix.Nodes {
			other.Links = removeInt(other.Links, delID)
		}
		ids := u.matrix.IDs()
		u.selID = ids[0]
		if u.matrix.Entry == delID {
			u.matrix.Entry = u.selID
			u.entryEd.SetText(strconv.Itoa(u.selID))
		}
		u.syncFromNode()
		u.a.setStatus(fmt.Sprintf("Deleted node #%d.", delID))
		return
	}
	if u.applyNodeBtn.Clicked(gtx) {
		u.applyNodeText()
		u.a.setStatus(fmt.Sprintf("Updated Node #%d text/link fields.", n.ID))
	}

	// ICE controls
	if u.iceClassBtn.Clicked(gtx) {
		u.iceClassIdx = (u.iceClassIdx + 1) % len(allICEClasses)
	}
	if u.iceTypeBtn.Clicked(gtx) {
		u.iceTypeIdx = (u.iceTypeIdx + 1) % len(allICETypes)
	}
	if u.iceTrigBtn.Clicked(gtx) {
		u.iceTrigIdx = (u.iceTrigIdx + 1) % len(allICETriggers)
	}
	if u.iceMobBtn.Clicked(gtx) {
		u.iceMobIdx = (u.iceMobIdx + 1) % len(allICEMobs)
	}
	if u.addICEBtn.Clicked(gtx) && len(n.ICE) < model.MaxICEPerNode {
		rat, err := strconv.Atoi(strings.TrimSpace(u.iceRatEd.Text()))
		if err != nil || rat < 1 {
			rat = 4
		}
		n.ICE = append(n.ICE, model.ICE{
			Class:    allICEClasses[u.iceClassIdx],
			Type:     allICETypes[u.iceTypeIdx],
			Stage:    strings.TrimSpace(u.iceStageEd.Text()),
			Rating:   rat,
			Trigger:  allICETriggers[u.iceTrigIdx],
			Mobility: allICEMobs[u.iceMobIdx],
		})
		u.refreshNodeLists()
	}
	if u.delICEBtn.Clicked(gtx) {
		if idx := u.iceList.Sel; idx >= 0 && idx < len(n.ICE) {
			n.ICE = append(n.ICE[:idx], n.ICE[idx+1:]...)
			u.iceList.Sel = -1
			u.refreshNodeLists()
		}
	}

	// File controls
	if u.addFileBtn.Clicked(gtx) && n.Kind == model.DS && len(n.Files) < model.MaxFilesPerDS {
		name := strings.TrimSpace(u.fNameEd.Text())
		if name == "" {
			name = fmt.Sprintf("File_%d", len(n.Files)+1)
		}
		sz, _ := strconv.Atoi(strings.TrimSpace(u.fSizeEd.Text()))
		if sz <= 0 {
			sz = 20
		}
		val, _ := strconv.Atoi(strings.TrimSpace(u.fValEd.Text()))
		n.Files = append(n.Files, model.File{
			Name:     name,
			Size:     sz,
			Value:    val,
			Password: strings.TrimSpace(u.fPwEd.Text()),
		})
		u.fNameEd.SetText("")
		u.refreshNodeLists()
	}
	if u.delFileBtn.Clicked(gtx) {
		if idx := u.fileList.Sel; idx >= 0 && idx < len(n.Files) {
			n.Files = append(n.Files[:idx], n.Files[idx+1:]...)
			u.fileList.Sel = -1
			u.refreshNodeLists()
		}
	}
}

// validateMatrix performs structural and SR2 / MATED rule checks (NODELIM.DAT).
func validateMatrix(m *model.Matrix) []string {
	var out []string
	if len(m.Nodes) == 0 {
		return []string{"Matrix has no nodes."}
	}
	entry := m.Nodes[m.Entry]
	if entry == nil {
		out = append(out, fmt.Sprintf("Entry node #%d does not exist.", m.Entry))
	} else if entry.Kind != model.SAN {
		out = append(out, fmt.Sprintf("Entry node #%d is %s (conventionally SAN).", m.Entry, entry.Kind))
	}

	hasCPU := false
	for _, id := range m.IDs() {
		n := m.Nodes[id]
		if n.Kind == model.CPU {
			hasCPU = true
		}
		if len(n.Links) == 0 && len(m.Nodes) > 1 {
			out = append(out, fmt.Sprintf("Node #%d (%s) is isolated (no datalines).", id, n.Kind))
		}
		if n.Color == model.Blue {
			for _, ic := range n.ICE {
				if strings.EqualFold(ic.Class, "Black") {
					out = append(out, fmt.Sprintf("Node #%d is Blue security but installs Black ICE (%s).", id, ic.Type))
				}
			}
		}
		if n.Kind == model.LNK && strings.TrimSpace(n.LinkFile) == "" {
			out = append(out, fmt.Sprintf("LNK Node #%d has no target .ltg file.", id))
		}
	}
	if !hasCPU {
		out = append(out, "Matrix has no CPU node (System Operations at CPU unavailable).")
	}
	if len(out) == 0 {
		out = append(out, fmt.Sprintf("OK: %d node(s) verified. All links, entry point, and security constraints are valid.", len(m.Nodes)))
	}
	return out
}
