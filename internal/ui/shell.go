package ui

import (
	"image"
	"path/filepath"
	"strings"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	capp "cyberspace-refactor/internal/app"
)

// Version of the application.
const Version = "1.0.0"

type picker struct {
	open   bool
	title  string
	save   bool
	exts   []string
	filter widget.Editor
	name   widget.Editor
	list   *listBox
	all    []string
	ok     widget.Clickable
	cancel widget.Clickable
	onPick func(rel string)
	lastSel int
}

func newPicker() *picker {
	p := &picker{list: newListBox()}
	p.filter.SingleLine = true
	p.name.SingleLine = true
	return p
}

func (p *picker) show(ws *capp.Workspace, title string, save bool, exts []string, defName string, cb func(string)) {
	p.open, p.title, p.save, p.exts, p.onPick = true, title, save, exts, cb
	p.all = ws.ListFiles(exts...)
	p.filter.SetText("")
	p.name.SetText(defName)
	p.list.Sel = -1
	p.refilter()
}

func (p *picker) refilter() {
	f := strings.ToLower(strings.TrimSpace(p.filter.Text()))
	var items []string
	for _, s := range p.all {
		if f == "" || strings.Contains(strings.ToLower(s), f) {
			items = append(items, s)
		}
	}
	p.list.Set(items)
}

func (p *picker) layout(gtx C, th *material.Theme) D {
	if !p.open {
		return D{}
	}
	// Input handling.
	if p.cancel.Clicked(gtx) {
		p.open = false
		return D{}
	}
	prevFilter := p.filter.Text()
	if p.ok.Clicked(gtx) {
		choice := ""
		if p.save {
			choice = strings.TrimSpace(p.name.Text())
		} else if p.list.Sel >= 0 && p.list.Sel < len(p.list.items) {
			choice = p.list.items[p.list.Sel]
		}
		if choice != "" {
			if p.save && filepath.Ext(choice) == "" && len(p.exts) > 0 {
				choice += p.exts[0]
			}
			p.open = false
			p.onPick(choice)
			return D{}
		}
	}
	if p.save && p.list.Sel != p.lastSel {
		p.lastSel = p.list.Sel
		if p.list.Sel >= 0 && p.list.Sel < len(p.list.items) {
			p.name.SetText(p.list.items[p.list.Sel])
		}
	}
	paint.FillShape(gtx.Ops, colorWithAlpha(colBg, 0xe0), clip.Rect{Max: gtx.Constraints.Max}.Op())
	d := layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = gtx.Dp(unit.Dp(640))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(520))
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		return fillBG(gtx, colPanel, func(gtx C) D {
			return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						l := material.H6(th, p.title)
						l.Color = colAmber
						return l.Layout(gtx)
					}),
					layout.Rigid(spacer(0, 6)),
					layout.Rigid(editor(th, &p.filter, "filter...")),
					layout.Rigid(spacer(0, 6)),
					layout.Flexed(1, func(gtx C) D { return p.list.layout(gtx, th) }),
					layout.Rigid(spacer(0, 6)),
					layout.Rigid(func(gtx C) D {
						if !p.save {
							return D{}
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(smallLabel(th, "File name (inside the assets folder, e.g. DATA/My Matrix.ltg):", colDim)),
							layout.Rigid(editor(th, &p.name, "DATA/name")),
							layout.Rigid(spacer(0, 6)))
					}),
					layout.Rigid(func(gtx C) D {
						ok := "Open"
						if p.save {
							ok = "Save"
						}
						return row(gtx, layout.Flexed(1, spacer(0, 0)),
							rigid(hotButton(th, &p.ok, ok, colGreenDark)), gap(8), rigid(button(th, &p.cancel, "Cancel")))
					}),
				)
			})
		})
	})
	if p.filter.Text() != prevFilter {
		p.refilter()
	}
	return d
}

// App is the top-level window state.
type App struct {
	ws     *capp.Workspace
	th     *material.Theme
	tab    int
	tabs   [5]widget.Clickable
	status string

	pick   *picker
	term   *terminal
	mated  *matedUI
	deckUI *deckUI
	dkrUI  *deckerUI
	set    *settingsUI
}

var tabNames = [5]string{"Cyberspace Terminal", "Matrix Construction Set", "Cyberdeck Designer", "Decker Biomonitor", "Settings & About"}

// Run opens the main window and blocks until it is closed.
func Run(ws *capp.Workspace) error {
	w := new(app.Window)
	w.Option(app.Title("Cyberspace Refactor "+Version), app.Size(unit.Dp(1360), unit.Dp(860)))
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette.Bg = colBg
	th.Palette.Fg = colFg
	th.Palette.ContrastBg = colGreenDark
	th.Palette.ContrastFg = colFg

	a := &App{ws: ws, th: th, pick: newPicker()}
	a.term = newTerminal(a)
	a.mated = newMatedUI(a)
	a.deckUI = newDeckUI(a)
	a.dkrUI = newDeckerUI(a)
	a.set = newSettingsUI(a)

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) layout(gtx C) D {
	paint.FillShape(gtx.Ops, colBg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	for i := range a.tabs {
		if a.tabs[i].Clicked(gtx) {
			a.tab = i
		}
	}
	d := layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.tabBar),
				layout.Flexed(1, func(gtx C) D {
					switch a.tab {
					case 0:
						return a.term.layout(gtx)
					case 1:
						return a.mated.layout(gtx)
					case 2:
						return a.deckUI.layout(gtx)
					case 3:
						return a.dkrUI.layout(gtx)
					}
					return a.set.layout(gtx)
				}),
				layout.Rigid(a.statusBar),
			)
		}),
		layout.Stacked(func(gtx C) D { return a.pick.layout(gtx, a.th) }),
	)
	return d
}

func (a *App) tabBar(gtx C) D {
	children := make([]layout.FlexChild, 0, len(a.tabs)*2)
	for i := range a.tabs {
		i := i
		bg := colBtn
		if i == a.tab {
			bg = colGreenDark
		}
		children = append(children, layout.Rigid(hotButton(a.th, &a.tabs[i], tabNames[i], bg)), gap(4))
	}
	return fillBG(gtx, colPanel, func(gtx C) D {
		return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D { return row(gtx, children...) })
	})
}

func (a *App) statusBar(gtx C) D {
	return fillBG(gtx, colPanel, func(gtx C) D {
		return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx,
			func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return smallLabel(a.th, a.status, colAmber)(gtx)
			})
	})
}

func (a *App) setStatus(s string) { a.status = s }

var _ = image.Pt
