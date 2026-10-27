package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// Palette used across the application.
var (
	colBg      = color.NRGBA{R: 0x0b, G: 0x10, B: 0x16, A: 0xff}
	colPanel   = color.NRGBA{R: 0x14, G: 0x1c, B: 0x26, A: 0xff}
	colPanel2  = color.NRGBA{R: 0x1c, G: 0x27, B: 0x34, A: 0xff}
	colFg      = color.NRGBA{R: 0xd7, G: 0xe3, B: 0xd7, A: 0xff}
	colDim     = color.NRGBA{R: 0x7f, G: 0x93, B: 0x8a, A: 0xff}
	colGreen   = color.NRGBA{R: 0x3d, G: 0xdc, B: 0x6a, A: 0xff}
	colAmber   = color.NRGBA{R: 0xf2, G: 0xc1, B: 0x3a, A: 0xff}
	colRed     = color.NRGBA{R: 0xe5, G: 0x48, B: 0x48, A: 0xff}
	colCyan    = color.NRGBA{R: 0x4f, G: 0xd1, B: 0xe8, A: 0xff}
	colSel     = color.NRGBA{R: 0x24, G: 0x4a, B: 0x36, A: 0xff}
	colGreenDark = color.NRGBA{R: 0x1f, G: 0x6b, B: 0x3a, A: 0xff}
	colBtn     = color.NRGBA{R: 0x2a, G: 0x3a, B: 0x4a, A: 0xff}
	colBtnText = color.NRGBA{R: 0xe8, G: 0xf2, B: 0xe8, A: 0xff}
)

func fillBG(gtx C, c color.NRGBA, w layout.Widget) D {
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			paint.FillShape(gtx.Ops, c, clip.Rect{Max: gtx.Constraints.Min}.Op())
			return D{Size: gtx.Constraints.Min}
		}, w)
}

func panel(gtx C, th *material.Theme, title string, w layout.Widget) D {
	return fillBG(gtx, colPanel, func(gtx C) D {
		return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					l := material.Caption(th, strings.ToUpper(title))
					l.Color = colAmber
					l.Font.Weight = 700
					return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, l.Layout)
				}),
				layout.Rigid(w),
			)
		})
	})
}

func label(th *material.Theme, s string, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		l := material.Body2(th, s)
		l.Color = c
		return l.Layout(gtx)
	}
}

func smallLabel(th *material.Theme, s string, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		l := material.Caption(th, s)
		l.Color = c
		return l.Layout(gtx)
	}
}

func button(th *material.Theme, clk *widget.Clickable, txt string) layout.Widget {
	return func(gtx C) D {
		b := material.Button(th, clk, txt)
		b.Background = colBtn
		b.Color = colBtnText
		b.TextSize = unit.Sp(12)
		b.Inset = layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(8), Right: unit.Dp(8)}
		return b.Layout(gtx)
	}
}

func hotButton(th *material.Theme, clk *widget.Clickable, txt string, bg color.NRGBA) layout.Widget {
	return func(gtx C) D {
		b := material.Button(th, clk, txt)
		b.Background = bg
		b.Color = color.NRGBA{A: 0xff, R: 0xff, G: 0xff, B: 0xff}
		b.TextSize = unit.Sp(12)
		b.Inset = layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(8), Right: unit.Dp(8)}
		return b.Layout(gtx)
	}
}

func spacer(w, h int) layout.Widget {
	return func(gtx C) D { return D{Size: image.Pt(gtx.Dp(unit.Dp(w)), gtx.Dp(unit.Dp(h)))} }
}

func editor(th *material.Theme, e *widget.Editor, hint string) layout.Widget {
	return func(gtx C) D {
		return fillBG(gtx, colPanel2, func(gtx C) D {
			return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
				ed := material.Editor(th, e, hint)
				ed.Color = colFg
				ed.HintColor = colDim
				ed.TextSize = unit.Sp(13)
				return ed.Layout(gtx)
			})
		})
	}
}

// listBox is a selectable, scrollable list of strings.
type listBox struct {
	list  widget.List
	clks  []widget.Clickable
	Sel   int
	items []string
}

func newListBox() *listBox {
	l := &listBox{Sel: -1}
	l.list.Axis = layout.Vertical
	return l
}

func (l *listBox) Set(items []string) {
	l.items = items
	if l.Sel >= len(items) {
		l.Sel = -1
	}
}

func (l *listBox) layout(gtx C, th *material.Theme) D {
	for len(l.clks) < len(l.items) {
		l.clks = append(l.clks, widget.Clickable{})
	}
	for i := range l.items {
		if l.clks[i].Clicked(gtx) {
			l.Sel = i
		}
	}
	return material.List(th, &l.list).Layout(gtx, len(l.items), func(gtx C, i int) D {
		bg := colPanel2
		if i == l.Sel {
			bg = colSel
		}
		return material.Clickable(gtx, &l.clks[i], func(gtx C) D {
			return fillBG(gtx, bg, func(gtx C) D {
				return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx C) D {
					t := material.Caption(th, l.items[i])
					t.Color = colFg
					t.MaxLines = 1
					t.TextSize = unit.Sp(12)
					return t.Layout(gtx)
				})
			})
		})
	})
}

// fixedH constrains a widget to a fixed dp height (width stays flexible).
func fixedH(h int, w layout.Widget) layout.Widget {
	return func(gtx C) D {
		px := gtx.Dp(unit.Dp(h))
		gtx.Constraints.Min.Y = px
		gtx.Constraints.Max.Y = px
		return w(gtx)
	}
}

func fixedW(wd int, w layout.Widget) layout.Widget {
	return func(gtx C) D {
		px := gtx.Dp(unit.Dp(wd))
		gtx.Constraints.Min.X = px
		gtx.Constraints.Max.X = px
		return w(gtx)
	}
}

func row(gtx C, ws ...layout.FlexChild) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, ws...) }

func rigid(w layout.Widget) layout.FlexChild { return layout.Rigid(w) }

func gap(w int) layout.FlexChild { return layout.Rigid(spacer(w, 0)) }

var _ = text.Start

func colorWithAlpha(c color.NRGBA, a uint8) color.NRGBA { c.A = a; return c }
