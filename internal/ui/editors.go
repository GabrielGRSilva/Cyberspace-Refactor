package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"cyberspace-refactor/internal/assets"
	"cyberspace-refactor/internal/engine"
	"cyberspace-refactor/internal/format"
	"cyberspace-refactor/internal/model"
)

// Standard Shadowrun 2nd Ed / VR 1.0 utility names and base size multipliers (Mp = Mult * Rating^2).
var stdUtilities = []struct {
	Name string
	Mult int
}{
	{"Analyze", 3},
	{"Armor", 3},
	{"Attack", 2}, // L=2, M=3, S=4, D=5
	{"Auto Execute", 2},
	{"Browse", 1},
	{"Cloak", 3},
	{"Compressor", 2},
	{"Deception", 2},
	{"Decrypt", 1},
	{"Evaluate", 1},
	{"Lock-On", 3},
	{"Medic", 4},
	{"Mirrors", 3},
	{"Relocate", 2},
	{"Restore", 3},
	{"Satlink", 3},
	{"Shield", 4},
	{"Sleaze", 3},
	{"Slow", 4},
	{"Smoke", 2},
}

func calcUtilSize(name, stage string, rating int) int {
	if rating <= 0 {
		return 10
	}
	mult := 2
	for _, u := range stdUtilities {
		if strings.EqualFold(u.Name, name) {
			mult = u.Mult
			break
		}
	}
	if strings.EqualFold(name, "Attack") {
		switch strings.ToUpper(strings.TrimSpace(stage)) {
		case "L":
			mult = 2
		case "M", "":
			mult = 3
		case "S":
			mult = 4
		case "D":
			mult = 5
		}
	}
	return mult * rating * rating
}

// ============================================================================
// Cyberdeck Designer (CYBDECK)
// ============================================================================

type deckUI struct {
	a    *App
	deck *model.Deck
	rel  string

	newBtn, openBtn, saveBtn, jsonBtn, useBtn widget.Clickable

	modelEd, mpcpEd, bodEd, evaEd, maskEd, senEd, dmgEd widget.Editor
	hardEd, actEd, stoEd, loadEd, ioEd, respEd          widget.Editor
	satChk, pureChk, rfChk                              widget.Bool
	dispBtn, applyBtn                                   widget.Clickable

	utilList                                      *listBox
	uPresetIdx                                    int
	uPresetBtn, addUtilBtn, delUtilBtn, compBtn   widget.Clickable
	uNameEd, uRatEd, uSizeEd, uStageEd, uTargetEd widget.Editor
}

func newDeckUI(a *App) *deckUI {
	u := &deckUI{
		a:        a,
		deck:     defaultDeck(),
		utilList: newListBox(),
	}
	for _, ed := range []*widget.Editor{
		&u.modelEd, &u.mpcpEd, &u.bodEd, &u.evaEd, &u.maskEd, &u.senEd, &u.dmgEd,
		&u.hardEd, &u.actEd, &u.stoEd, &u.loadEd, &u.ioEd, &u.respEd,
		&u.uNameEd, &u.uRatEd, &u.uSizeEd, &u.uStageEd, &u.uTargetEd,
	} {
		ed.SingleLine = true
	}
	u.uNameEd.SetText(stdUtilities[0].Name)
	u.uRatEd.SetText("6")
	u.uSizeEd.SetText(strconv.Itoa(calcUtilSize(stdUtilities[0].Name, "", 6)))
	u.syncFromDeck()
	return u
}

func defaultDeck() *model.Deck {
	return &model.Deck{
		Persona: model.Persona{
			Model:   "Fuchi Cyber-4",
			MPCP:    6,
			Bod:     4,
			Evasion: 4,
			Masking: 5,
			Sensor:  5,
		},
		Hardening:   3,
		ActiveMem:   200,
		StorageMem:  500,
		LoadSpeed:   50,
		IOSpeed:     30,
		Response:    1,
		DisplayMode: 2,
		Utilities: []model.Utility{
			{Name: "Analyze", Rating: 5, Size: 75},
			{Name: "Attack", Rating: 5, Stage: "M", Size: 75},
			{Name: "Sleaze", Rating: 5, Size: 75},
			{Name: "Deception", Rating: 5, Size: 50},
			{Name: "Medic", Rating: 4, Size: 64},
		},
	}
}

func (u *deckUI) syncFromDeck() {
	d := u.deck
	u.modelEd.SetText(d.Persona.Model)
	u.mpcpEd.SetText(strconv.Itoa(d.Persona.MPCP))
	u.bodEd.SetText(strconv.Itoa(d.Persona.Bod))
	u.evaEd.SetText(strconv.Itoa(d.Persona.Evasion))
	u.maskEd.SetText(strconv.Itoa(d.Persona.Masking))
	u.senEd.SetText(strconv.Itoa(d.Persona.Sensor))
	u.dmgEd.SetText(strconv.Itoa(d.Persona.Damage))
	u.hardEd.SetText(strconv.Itoa(d.Hardening))
	u.actEd.SetText(strconv.Itoa(d.ActiveMem))
	u.stoEd.SetText(strconv.Itoa(d.StorageMem))
	u.loadEd.SetText(strconv.Itoa(d.LoadSpeed))
	u.ioEd.SetText(strconv.Itoa(d.IOSpeed))
	u.respEd.SetText(strconv.Itoa(d.Response))
	u.satChk.Value = d.SatUplink
	u.pureChk.Value = d.PureCyber
	u.rfChk.Value = d.RealityFilter
	u.refreshUtils()
}

func atoiDef(ed *widget.Editor, def, lo, hi int) int {
	v, err := strconv.Atoi(strings.TrimSpace(ed.Text()))
	if err != nil {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (u *deckUI) applyToDeck() {
	d := u.deck
	d.Persona.Model = strings.TrimSpace(u.modelEd.Text())
	if d.Persona.Model == "" {
		d.Persona.Model = "Custom Cyberdeck"
	}
	d.Persona.MPCP = atoiDef(&u.mpcpEd, d.Persona.MPCP, 1, 20)
	d.Persona.Bod = atoiDef(&u.bodEd, d.Persona.Bod, 0, 20)
	d.Persona.Evasion = atoiDef(&u.evaEd, d.Persona.Evasion, 0, 20)
	d.Persona.Masking = atoiDef(&u.maskEd, d.Persona.Masking, 0, 20)
	d.Persona.Sensor = atoiDef(&u.senEd, d.Persona.Sensor, 0, 20)
	d.Persona.Damage = atoiDef(&u.dmgEd, d.Persona.Damage, 0, 10)
	d.Hardening = atoiDef(&u.hardEd, d.Hardening, 0, 20)
	d.ActiveMem = atoiDef(&u.actEd, d.ActiveMem, 1, 100000)
	d.StorageMem = atoiDef(&u.stoEd, d.StorageMem, 1, 100000)
	d.LoadSpeed = atoiDef(&u.loadEd, d.LoadSpeed, 1, 100000)
	d.IOSpeed = atoiDef(&u.ioEd, d.IOSpeed, 1, 100000)
	d.Response = atoiDef(&u.respEd, d.Response, 0, 3)
	d.SatUplink = u.satChk.Value
	d.PureCyber = u.pureChk.Value
	d.RealityFilter = u.rfChk.Value
}

func (u *deckUI) refreshUtils() {
	var items []string
	for i, ut := range u.deck.Utilities {
		st := ""
		if ut.Stage != "" {
			st = "[" + ut.Stage + "]"
		}
		tg := ""
		if ut.Target != "" {
			tg = " (" + ut.Target + ")"
		}
		items = append(items, fmt.Sprintf("%d. %s%s%s - Rating %d (%d Mp)", i+1, ut.Name, st, tg, ut.Rating, ut.Size))
	}
	u.utilList.Set(items)
}

func (u *deckUI) deckWarnings() []string {
	d := u.deck
	var w []string
	sum := d.Persona.Bod + d.Persona.Evasion + d.Persona.Masking + d.Persona.Sensor
	if sum > 3*d.Persona.MPCP {
		w = append(w, fmt.Sprintf("Persona total (%d) exceeds 3 x MPCP (%d).", sum, 3*d.Persona.MPCP))
	}
	for _, p := range []struct {
		n string
		v int
	}{
		{"Bod", d.Persona.Bod}, {"Evasion", d.Persona.Evasion},
		{"Masking", d.Persona.Masking}, {"Sensor", d.Persona.Sensor},
	} {
		if p.v > d.Persona.MPCP {
			w = append(w, fmt.Sprintf("%s (%d) exceeds MPCP (%d).", p.n, p.v, d.Persona.MPCP))
		}
	}
	if d.Hardening > d.Persona.MPCP {
		w = append(w, fmt.Sprintf("Hardening (%d) exceeds MPCP (%d).", d.Hardening, d.Persona.MPCP))
	}
	used := 0
	for _, ut := range d.Utilities {
		used += ut.Size
	}
	if used > d.StorageMem {
		w = append(w, fmt.Sprintf("Utilities total (%d Mp) exceeds Storage Memory (%d Mp).", used, d.StorageMem))
	}
	if len(w) == 0 {
		w = append(w, fmt.Sprintf("OK: Persona sum %d / %d max; Storage used %d / %d Mp.", sum, 3*d.Persona.MPCP, used, d.StorageMem))
	}
	return w
}

func (u *deckUI) layout(gtx C) D {
	u.handleActions(gtx)
	th := u.a.th
	dispNames := map[int]string{0: "0 Standard", 1: "1 Advanced", 2: "2 Sculpted"}
	return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return fillBG(gtx, colPanel, func(gtx C) D {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
						return row(gtx,
							rigid(button(th, &u.newBtn, "New Deck")), gap(4),
							rigid(button(th, &u.openBtn, "Open .DEK/.NPC/.JSON")), gap(4),
							rigid(hotButton(th, &u.saveBtn, "Save .DEK", colGreenDark)), gap(4),
							rigid(button(th, &u.jsonBtn, "Export .JSON")), gap(8),
							rigid(hotButton(th, &u.useBtn, "Load Into Terminal", colBtn)),
						)
					})
				})
			}),
			layout.Rigid(spacer(0, 4)),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(fixedW(440, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return panel(gtx, th, "Persona & Hardware Parameters", func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(fieldRow(th, "Deck Model:", &u.modelEd, "Fuchi Cyber-4")),
										layout.Rigid(spacer(0, 4)),
										layout.Rigid(twoFieldRow(th, "MPCP:", &u.mpcpEd, "Hardening:", &u.hardEd)),
										layout.Rigid(spacer(0, 4)),
										layout.Rigid(twoFieldRow(th, "Bod:", &u.bodEd, "Evasion:", &u.evaEd)),
										layout.Rigid(spacer(0, 4)),
										layout.Rigid(twoFieldRow(th, "Masking:", &u.maskEd, "Sensor:", &u.senEd)),
										layout.Rigid(spacer(0, 4)),
										layout.Rigid(twoFieldRow(th, "Active Mem (Mp):", &u.actEd, "Storage (Mp):", &u.stoEd)),
										layout.Rigid(spacer(0, 4)),
										layout.Rigid(twoFieldRow(th, "Load Spd (Mp):", &u.loadEd, "I/O Spd (Mp):", &u.ioEd)),
										layout.Rigid(spacer(0, 4)),
										layout.Rigid(twoFieldRow(th, "Response (0-3):", &u.respEd, "MPCP Damage:", &u.dmgEd)),
										layout.Rigid(spacer(0, 6)),
										layout.Rigid(func(gtx C) D {
											return row(gtx,
												rigid(checkBox(th, &u.satChk, "Satellite Uplink")), gap(6),
												rigid(checkBox(th, &u.pureChk, "Pure Cyber")), gap(6),
												rigid(checkBox(th, &u.rfChk, "Reality Filter")),
											)
										}),
										layout.Rigid(spacer(0, 6)),
										layout.Rigid(func(gtx C) D {
											return row(gtx,
												rigid(button(th, &u.dispBtn, "Display: "+dispNames[u.deck.DisplayMode])),
												gap(8),
												rigid(hotButton(th, &u.applyBtn, "Apply Parameters", colGreenDark)),
											)
										}),
									)
								})
							}),
							layout.Rigid(spacer(0, 4)),
							layout.Flexed(1, func(gtx C) D {
								return panel(gtx, th, "Cyberdeck Design Rule Check (SR2 / VR 1.0)", func(gtx C) D {
									var ch []layout.FlexChild
									for _, msg := range u.deckWarnings() {
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
					})),
					gap(4),
					layout.Flexed(1, func(gtx C) D {
						return panel(gtx, th, fmt.Sprintf("Storage Memory Utilities (%d items)", len(u.deck.Utilities)), func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(fixedH(340, func(gtx C) D { return u.utilList.layout(gtx, th) })),
								layout.Rigid(spacer(0, 6)),
								layout.Rigid(smallLabel(th, "Add Utility to Storage (click Preset to cycle standard SR2 programs):", colDim)),
								layout.Rigid(spacer(0, 3)),
								layout.Rigid(func(gtx C) D {
									return row(gtx,
										rigid(button(th, &u.uPresetBtn, "Preset: "+stdUtilities[u.uPresetIdx].Name)),
										gap(6),
										rigid(smallLabel(th, "Name:", colDim)), gap(3),
										layout.Flexed(1, editor(th, &u.uNameEd, "Utility name")),
									)
								}),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(func(gtx C) D {
									return row(gtx,
										rigid(smallLabel(th, "Rating:", colDim)), gap(3),
										rigid(fixedW(52, editor(th, &u.uRatEd, "6"))), gap(6),
										rigid(smallLabel(th, "Stage (L/M/S/D):", colDim)), gap(3),
										rigid(fixedW(52, editor(th, &u.uStageEd, "M"))), gap(6),
										rigid(smallLabel(th, "Target ICE:", colDim)), gap(3),
										rigid(fixedW(110, editor(th, &u.uTargetEd, "Access"))), gap(6),
										rigid(smallLabel(th, "Size (Mp, 0=Auto):", colDim)), gap(3),
										rigid(fixedW(70, editor(th, &u.uSizeEd, "0"))),
									)
								}),
								layout.Rigid(spacer(0, 6)),
								layout.Rigid(func(gtx C) D {
									return row(gtx,
										rigid(hotButton(th, &u.addUtilBtn, "+Add Utility", colGreenDark)),
										gap(6),
										rigid(button(th, &u.delUtilBtn, "-Remove Selected")),
										gap(6),
										rigid(button(th, &u.compBtn, "Run Compressor on Storage")),
									)
								}),
							)
						})
					}),
				)
			}),
		)
	})
}

func (u *deckUI) handleActions(gtx C) {
	if u.newBtn.Clicked(gtx) {
		u.deck = defaultDeck()
		u.rel = ""
		u.syncFromDeck()
		u.a.setStatus("Initialized default Cyberdeck.")
	}
	if u.openBtn.Clicked(gtx) {
		u.a.pick.show(u.a.ws, "Open Cyberdeck (.dek / .npc / .json)", false, []string{".dek", ".npc", ".json"}, "", func(rel string) {
			d, _, err := u.a.ws.LoadDeck(rel)
			if err != nil {
				u.a.setStatus("Load deck error: " + err.Error())
				return
			}
			u.deck, u.rel = d, rel
			u.syncFromDeck()
			u.a.setStatus("Loaded deck " + rel)
		})
	}
	if u.saveBtn.Clicked(gtx) {
		u.applyToDeck()
		def := u.rel
		if def == "" || !strings.HasSuffix(strings.ToLower(def), ".dek") {
			def = "DATA/custom.dek"
		}
		u.a.pick.show(u.a.ws, "Save Cyberdeck (.dek)", true, []string{".dek"}, def, func(rel string) {
			if err := u.a.ws.Save(rel, format.WriteDeck(u.deck)); err != nil {
				u.a.setStatus("Save error: " + err.Error())
			} else {
				u.rel = rel
				u.a.setStatus("Saved deck to " + rel)
			}
		})
	}
	if u.jsonBtn.Clicked(gtx) {
		u.applyToDeck()
		def := "DATA/custom_deck.json"
		if u.rel != "" {
			def = strings.TrimSuffix(u.rel, filepath.Ext(u.rel)) + ".json"
		}
		u.a.pick.show(u.a.ws, "Export Cyberdeck as JSON", true, []string{".json"}, def, func(rel string) {
			b, err := format.DeckToJSON(u.deck)
			if err != nil {
				u.a.setStatus("JSON error: " + err.Error())
				return
			}
			if err := u.a.ws.Save(rel, b); err != nil {
				u.a.setStatus("Export error: " + err.Error())
			} else {
				u.a.setStatus("Exported deck JSON to " + rel)
			}
		})
	}
	if u.useBtn.Clicked(gtx) {
		u.applyToDeck()
		u.a.term.deck = u.deck.Clone()
		u.a.term.deckRel = u.rel
		if u.a.term.deckRel == "" {
			u.a.term.deckRel = "(designed in memory)"
		}
		u.a.setStatus("Loaded designed deck into Cyberspace Terminal.")
	}
	if u.dispBtn.Clicked(gtx) {
		u.deck.DisplayMode = (u.deck.DisplayMode + 1) % 3
	}
	if u.applyBtn.Clicked(gtx) {
		u.applyToDeck()
		u.syncFromDeck()
		u.a.setStatus("Updated cyberdeck parameters.")
	}
	if u.uPresetBtn.Clicked(gtx) {
		u.uPresetIdx = (u.uPresetIdx + 1) % len(stdUtilities)
		name := stdUtilities[u.uPresetIdx].Name
		u.uNameEd.SetText(name)
		rat := atoiDef(&u.uRatEd, 6, 1, 20)
		u.uSizeEd.SetText(strconv.Itoa(calcUtilSize(name, u.uStageEd.Text(), rat)))
	}
	if u.addUtilBtn.Clicked(gtx) && len(u.deck.Utilities) < model.MaxUtilities {
		name := strings.TrimSpace(u.uNameEd.Text())
		if name == "" {
			name = stdUtilities[u.uPresetIdx].Name
		}
		rat := atoiDef(&u.uRatEd, 4, 1, 20)
		stg := strings.TrimSpace(u.uStageEd.Text())
		sz := atoiDef(&u.uSizeEd, 0, 0, 100000)
		if sz <= 0 {
			sz = calcUtilSize(name, stg, rat)
		}
		u.deck.Utilities = append(u.deck.Utilities, model.Utility{
			Name:   name,
			Rating: rat,
			Size:   sz,
			Stage:  stg,
			Target: strings.TrimSpace(u.uTargetEd.Text()),
		})
		u.refreshUtils()
	}
	if u.delUtilBtn.Clicked(gtx) {
		if idx := u.utilList.Sel; idx >= 0 && idx < len(u.deck.Utilities) {
			u.deck.Utilities = append(u.deck.Utilities[:idx], u.deck.Utilities[idx+1:]...)
			u.utilList.Sel = -1
			u.refreshUtils()
		}
	}
	if u.compBtn.Clicked(gtx) {
		compRat := 0
		for _, ut := range u.deck.Utilities {
			if strings.EqualFold(ut.Name, "Compressor") && ut.Rating > compRat {
				compRat = ut.Rating
			}
		}
		if compRat == 0 {
			u.a.setStatus("Add a Compressor utility to storage first to compress programs.")
		} else {
			pct := min(50, compRat*10)
			for i := range u.deck.Utilities {
				ut := &u.deck.Utilities[i]
				base := calcUtilSize(ut.Name, ut.Stage, ut.Rating)
				ut.Size = max(1, base*(100-pct)/100)
			}
			u.refreshUtils()
			u.a.setStatus(fmt.Sprintf("Compressed all utilities by %d%% (Compressor-%d).", pct, compRat))
		}
	}
}

// ============================================================================
// Decker Biomonitor (DECKERED)
// ============================================================================

type deckerUI struct {
	a      *App
	decker *model.Decker
	rel    string

	newBtn, openBtn, saveBtn, jsonBtn, useBtn, applyBtn widget.Clickable

	nameEd                                               widget.Editor
	bodyEd, quickEd, strEd, chaEd, intEd, willEd         widget.Editor
	reacEd, reacDiceEd, compEd, etiqEd                   widget.Editor
	karmaEd, hackPoolEd, teamPoolEd, stunEd, physEd      widget.Editor
}

func newDeckerUI(a *App) *deckerUI {
	u := &deckerUI{
		a:      a,
		decker: defaultDecker(),
	}
	for _, ed := range []*widget.Editor{
		&u.nameEd, &u.bodyEd, &u.quickEd, &u.strEd, &u.chaEd, &u.intEd, &u.willEd,
		&u.reacEd, &u.reacDiceEd, &u.compEd, &u.etiqEd,
		&u.karmaEd, &u.hackPoolEd, &u.teamPoolEd, &u.stunEd, &u.physEd,
	} {
		ed.SingleLine = true
	}
	u.syncFromDecker()
	return u
}

func defaultDecker() *model.Decker {
	return &model.Decker{
		Name:         "Fastjack",
		Body:         4,
		Quickness:    6,
		Strength:     3,
		Charisma:     5,
		Intelligence: 6,
		Willpower:    6,
		Reaction:     6,
		ReactionDice: 1,
		Computer:     6,
		Etiquette:    4,
		Karma:        5,
		HackingPool:  0,
	}
}

func (u *deckerUI) syncFromDecker() {
	d := u.decker
	u.nameEd.SetText(d.Name)
	u.bodyEd.SetText(strconv.Itoa(d.Body))
	u.quickEd.SetText(strconv.Itoa(d.Quickness))
	u.strEd.SetText(strconv.Itoa(d.Strength))
	u.chaEd.SetText(strconv.Itoa(d.Charisma))
	u.intEd.SetText(strconv.Itoa(d.Intelligence))
	u.willEd.SetText(strconv.Itoa(d.Willpower))
	u.reacEd.SetText(strconv.Itoa(d.Reaction))
	u.reacDiceEd.SetText(strconv.Itoa(d.ReactionDice))
	u.compEd.SetText(strconv.Itoa(d.Computer))
	u.etiqEd.SetText(strconv.Itoa(d.Etiquette))
	u.karmaEd.SetText(strconv.Itoa(d.Karma))
	u.hackPoolEd.SetText(strconv.Itoa(d.HackingPool))
	u.teamPoolEd.SetText(strconv.Itoa(d.TeamPool))
	u.stunEd.SetText(strconv.Itoa(d.StunDamage))
	u.physEd.SetText(strconv.Itoa(d.PhysDamage))
}

func (u *deckerUI) applyToDecker() {
	d := u.decker
	d.Name = strings.TrimSpace(u.nameEd.Text())
	if d.Name == "" {
		d.Name = "Console Cowboy"
	}
	d.Body = atoiDef(&u.bodyEd, d.Body, 1, 20)
	d.Quickness = atoiDef(&u.quickEd, d.Quickness, 1, 20)
	d.Strength = atoiDef(&u.strEd, d.Strength, 1, 20)
	d.Charisma = atoiDef(&u.chaEd, d.Charisma, 1, 20)
	d.Intelligence = atoiDef(&u.intEd, d.Intelligence, 1, 20)
	d.Willpower = atoiDef(&u.willEd, d.Willpower, 1, 20)
	d.Reaction = atoiDef(&u.reacEd, (d.Quickness+d.Intelligence)/2, 1, 30)
	d.ReactionDice = atoiDef(&u.reacDiceEd, 1, 1, 5)
	d.Computer = atoiDef(&u.compEd, d.Computer, 0, 20)
	d.Etiquette = atoiDef(&u.etiqEd, d.Etiquette, 0, 20)
	d.Karma = atoiDef(&u.karmaEd, d.Karma, 0, 100)
	d.HackingPool = atoiDef(&u.hackPoolEd, d.HackingPool, 0, 50)
	d.TeamPool = atoiDef(&u.teamPoolEd, d.TeamPool, 0, 50)
	d.StunDamage = atoiDef(&u.stunEd, d.StunDamage, 0, 10)
	d.PhysDamage = atoiDef(&u.physEd, d.PhysDamage, 0, 10)
}

func (u *deckerUI) layout(gtx C) D {
	u.handleActions(gtx)
	th := u.a.th
	d := u.decker
	sampleDeck := u.a.deckUI.deck
	reac, dice := engine.Initiative(d, sampleDeck, false)
	origPool := engine.HackingPool(engine.PoolOriginal, d, sampleDeck)
	sr2Pool := engine.HackingPool(engine.PoolSR2Core, d, sampleDeck)

	return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return fillBG(gtx, colPanel, func(gtx C) D {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
						return row(gtx,
							rigid(button(th, &u.newBtn, "New Decker")), gap(4),
							rigid(button(th, &u.openBtn, "Open .NPC/.JSON")), gap(4),
							rigid(hotButton(th, &u.saveBtn, "Save .NPC", colGreenDark)), gap(4),
							rigid(button(th, &u.jsonBtn, "Export .JSON")), gap(8),
							rigid(hotButton(th, &u.useBtn, "Load Into Terminal", colBtn)),
						)
					})
				})
			}),
			layout.Rigid(spacer(0, 4)),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(fixedW(460, func(gtx C) D {
						return panel(gtx, th, "Decker Attributes, Skills & Pools", func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(fieldRow(th, "Handle / Name:", &u.nameEd, "Fastjack")),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(twoFieldRow(th, "Body:", &u.bodyEd, "Quickness:", &u.quickEd)),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(twoFieldRow(th, "Strength:", &u.strEd, "Charisma:", &u.chaEd)),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(twoFieldRow(th, "Intelligence:", &u.intEd, "Willpower:", &u.willEd)),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(twoFieldRow(th, "Reaction:", &u.reacEd, "Reaction Dice:", &u.reacDiceEd)),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(twoFieldRow(th, "Computer Skill:", &u.compEd, "Etiquette Skill:", &u.etiqEd)),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(twoFieldRow(th, "Karma Pool:", &u.karmaEd, "Hacking Pool (0=Auto):", &u.hackPoolEd)),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(twoFieldRow(th, "Teamwork Pool:", &u.teamPoolEd, "Stun Boxes (0-10):", &u.stunEd)),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(fieldRow(th, "Physical Boxes (0-10):", &u.physEd, "0")),
								layout.Rigid(spacer(0, 8)),
								layout.Rigid(hotButton(th, &u.applyBtn, "Apply Decker Biomonitor", colGreenDark)),
							)
						})
					})),
					gap(4),
					layout.Flexed(1, func(gtx C) D {
						return panel(gtx, th, "Derived Matrix Biomonitor Preview", func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(label(th, fmt.Sprintf("Decker: %s", d.Name), colAmber)),
								layout.Rigid(spacer(0, 6)),
								layout.Rigid(smallLabel(th, fmt.Sprintf("Paired Deck in Designer: %s (MPCP %d, Response +%d)", sampleDeck.Persona.Model, sampleDeck.Persona.MPCP, sampleDeck.Response), colFg)),
								layout.Rigid(spacer(0, 6)),
								layout.Rigid(smallLabel(th, fmt.Sprintf("Matrix Initiative: %d + %dd6", reac, dice), colGreen)),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(smallLabel(th, fmt.Sprintf("Hacking Pool (Original CYBERSPC formula 2*Comp + 2*Resp): %d dice", origPool), colCyan)),
								layout.Rigid(spacer(0, 4)),
								layout.Rigid(smallLabel(th, fmt.Sprintf("Hacking Pool (SR2 Core Rulebook (Int + MPCP)/3): %d dice", sr2Pool), colCyan)),
								layout.Rigid(spacer(0, 8)),
								layout.Rigid(smallLabel(th, fmt.Sprintf("Pre-existing Condition: Stun %s (%d/10), Physical %s (%d/10)",
									engine.StageName(engine.Level(d.StunDamage)), d.StunDamage,
									engine.StageName(engine.Level(d.PhysDamage)), d.PhysDamage), colFg)),
							)
						})
					}),
				)
			}),
		)
	})
}

func (u *deckerUI) handleActions(gtx C) {
	if u.newBtn.Clicked(gtx) {
		u.decker = defaultDecker()
		u.rel = ""
		u.syncFromDecker()
		u.a.setStatus("Initialized default Decker.")
	}
	if u.openBtn.Clicked(gtx) {
		u.a.pick.show(u.a.ws, "Open Decker (.npc / .json)", false, []string{".npc", ".json"}, "", func(rel string) {
			d, _, err := u.a.ws.LoadDecker(rel)
			if err != nil {
				u.a.setStatus("Load decker error: " + err.Error())
				return
			}
			u.decker, u.rel = d, rel
			u.syncFromDecker()
			u.a.setStatus("Loaded decker " + rel)
		})
	}
	if u.saveBtn.Clicked(gtx) {
		u.applyToDecker()
		def := u.rel
		if def == "" || !strings.HasSuffix(strings.ToLower(def), ".npc") {
			def = "DATA/custom.npc"
		}
		u.a.pick.show(u.a.ws, "Save Decker (.npc)", true, []string{".npc"}, def, func(rel string) {
			if err := u.a.ws.Save(rel, format.WriteNPC(u.decker)); err != nil {
				u.a.setStatus("Save error: " + err.Error())
			} else {
				u.rel = rel
				u.a.setStatus("Saved decker to " + rel)
			}
		})
	}
	if u.jsonBtn.Clicked(gtx) {
		u.applyToDecker()
		def := "DATA/custom_decker.json"
		if u.rel != "" {
			def = strings.TrimSuffix(u.rel, filepath.Ext(u.rel)) + ".json"
		}
		u.a.pick.show(u.a.ws, "Export Decker as JSON", true, []string{".json"}, def, func(rel string) {
			b, err := format.DeckerToJSON(u.decker)
			if err != nil {
				u.a.setStatus("JSON error: " + err.Error())
				return
			}
			if err := u.a.ws.Save(rel, b); err != nil {
				u.a.setStatus("Export error: " + err.Error())
			} else {
				u.a.setStatus("Exported decker JSON to " + rel)
			}
		})
	}
	if u.useBtn.Clicked(gtx) {
		u.applyToDecker()
		cp := *u.decker
		u.a.term.decker = &cp
		u.a.term.deckerRel = u.rel
		if u.a.term.deckerRel == "" {
			u.a.term.deckerRel = "(designed in memory)"
		}
		u.a.setStatus("Loaded designed decker into Cyberspace Terminal.")
	}
	if u.applyBtn.Clicked(gtx) {
		u.applyToDecker()
		u.syncFromDecker()
		u.a.setStatus("Updated decker biomonitor parameters.")
	}
}

// ============================================================================
// Settings & About
// ============================================================================

type settingsUI struct {
	a *App

	soundChk, karmaChk                      widget.Bool
	poolOrigBtn, poolSR2Btn                 widget.Clickable
	dispBtns                                [4]widget.Clickable
	testSndBtn, saveSetBtn, batchConvertBtn widget.Clickable
	batchMsg                                string
}

func newSettingsUI(a *App) *settingsUI {
	u := &settingsUI{a: a}
	u.soundChk.Value = a.ws.Settings.Sound
	u.karmaChk.Value = a.ws.Settings.AutoKarma
	return u
}

func (u *settingsUI) layout(gtx C) D {
	u.handleActions(gtx)
	th := u.a.th
	s := &u.a.ws.Settings

	dispLabels := [4]string{"Deck Default (-1)", "Standard (0)", "Advanced (1)", "Sculpted (2)"}
	dispVals := [4]int{-1, 0, 1, 2}
	var dispRow []layout.FlexChild
	for i := range dispLabels {
		i := i
		bg := colBtn
		if s.DisplayMod == dispVals[i] {
			bg = colGreenDark
		}
		dispRow = append(dispRow, rigid(hotButton(th, &u.dispBtns[i], dispLabels[i], bg)), gap(4))
	}

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return panel(gtx, th, "Application & Simulation Settings", func(gtx C) D {
					bgOrig, bgSR2 := colBtn, colBtn
					if s.PoolMode == 0 {
						bgOrig = colGreenDark
					} else {
						bgSR2 = colGreenDark
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return row(gtx,
								rigid(checkBox(th, &u.soundChk, "Enable WAV Sound Effects (winmm.dll)")),
								gap(12),
								rigid(button(th, &u.testSndBtn, "Test Sound (SNDBOOT.WAV)")),
							)
						}),
						layout.Rigid(spacer(0, 6)),
						layout.Rigid(checkBox(th, &u.karmaChk, "Auto-Spend Karma Pool to Avoid Critical Crash / Dump Shock")),
						layout.Rigid(spacer(0, 8)),
						layout.Rigid(smallLabel(th, "Hacking Pool Formula (when Decker HackingPool=0):", colDim)),
						layout.Rigid(spacer(0, 3)),
						layout.Rigid(func(gtx C) D {
							return row(gtx,
								rigid(hotButton(th, &u.poolOrigBtn, "Original CYBERSPC.EXE (2*Computer + 2*Response)", bgOrig)),
								gap(6),
								rigid(hotButton(th, &u.poolSR2Btn, "Shadowrun 2nd Ed Core ((Intelligence + MPCP)/3)", bgSR2)),
							)
						}),
						layout.Rigid(spacer(0, 8)),
						layout.Rigid(smallLabel(th, "Vidscreen Display Mode Override:", colDim)),
						layout.Rigid(spacer(0, 3)),
						layout.Rigid(func(gtx C) D { return row(gtx, dispRow...) }),
						layout.Rigid(spacer(0, 10)),
						layout.Rigid(hotButton(th, &u.saveSetBtn, "Save Settings to settings.json", colGreenDark)),
					)
				})
			}),
			layout.Rigid(spacer(0, 6)),
			layout.Rigid(func(gtx C) D {
				return panel(gtx, th, "Legacy Corpus -> JSON Batch Converter", func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(smallLabel(th, "Converts all legacy .ltg, .dek, and .npc files under assets/DATA into assets/JSON without modifying any original files.", colFg)),
						layout.Rigid(spacer(0, 6)),
						layout.Rigid(func(gtx C) D {
							return row(gtx,
								rigid(hotButton(th, &u.batchConvertBtn, "Convert All Legacy Files to assets/JSON/", colGreenDark)),
								gap(10),
								rigid(smallLabel(th, u.batchMsg, colAmber)),
							)
						}),
					)
				})
			}),
			layout.Rigid(spacer(0, 6)),
			layout.Flexed(1, func(gtx C) D {
				return panel(gtx, th, "About Cyberspace Refactor "+Version, func(gtx C) D {
					lines := []string{
						"Cyberspace Refactor — Clean-room Golang recreation of the classic Cyberspace suite originally created by Stefan Markgraf (1995–1998) for Shadowrun 2nd Ed / Virtual Realities 1.0.",
						"• Unified single-executable architecture integrating Terminal (CYBERSPC), Matrix Construction Set (MATED), Cyberdeck Designer (CYBDECK), and Decker Biomonitor (DECKERED).",
						"• Full backward compatibility with legacy .ltg, .dek, .npc, .bmp (white-key sprites), and .wav sound assets.",
						"• Strictly offline operation (zero network stack in application code) and hardened file confinement (OWASP Top 10 & NIST CSF aligned).",
					}
					var ch []layout.FlexChild
					for _, l := range lines {
						ch = append(ch, layout.Rigid(smallLabel(th, l, colFg)), layout.Rigid(spacer(0, 4)))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, ch...)
				})
			}),
		)
	})
}

func (u *settingsUI) handleActions(gtx C) {
	s := &u.a.ws.Settings
	s.Sound = u.soundChk.Value
	s.AutoKarma = u.karmaChk.Value

	if u.testSndBtn.Clicked(gtx) {
		u.a.ws.Library.Play(assets.SndBoot)
		u.a.setStatus("Triggered SNDBOOT.WAV")
	}
	if u.poolOrigBtn.Clicked(gtx) {
		s.PoolMode = 0
	}
	if u.poolSR2Btn.Clicked(gtx) {
		s.PoolMode = 1
	}
	dispVals := [4]int{-1, 0, 1, 2}
	for i := range u.dispBtns {
		if u.dispBtns[i].Clicked(gtx) {
			s.DisplayMod = dispVals[i]
		}
	}
	if u.saveSetBtn.Clicked(gtx) {
		if err := u.a.ws.SaveSettings(); err != nil {
			u.a.setStatus("Save settings failed: " + err.Error())
		} else {
			u.a.setStatus("Saved settings.json")
		}
	}
	if u.batchConvertBtn.Clicked(gtx) {
		okCount, errCount := 0, 0
		for _, rel := range u.a.ws.ListFiles(".ltg", ".dek", ".npc") {
			ext := strings.ToLower(filepath.Ext(rel))
			base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
			var out []byte
			var err error
			switch ext {
			case ".ltg":
				if m, _, e := u.a.ws.LoadMatrix(rel); e == nil {
					out, err = format.MatrixToJSON(m)
				} else {
					err = e
				}
			case ".dek":
				if d, _, e := u.a.ws.LoadDeck(rel); e == nil {
					out, err = format.DeckToJSON(d)
				} else {
					err = e
				}
			case ".npc":
				if d, _, e := u.a.ws.LoadDecker(rel); e == nil {
					out, err = format.DeckerToJSON(d)
				} else {
					err = e
				}
			}
			if err != nil {
				errCount++
				continue
			}
			dst := fmt.Sprintf("JSON/%s_%s.json", base, ext[1:])
			if err := u.a.ws.Save(dst, out); err != nil {
				errCount++
			} else {
				okCount++
			}
		}
		u.batchMsg = fmt.Sprintf("Converted %d files to assets/JSON/ (%d skipped/invalid).", okCount, errCount)
		u.a.setStatus(u.batchMsg)
	}
}

// ============================================================================
// Form row helpers
// ============================================================================

func fieldRow(th *material.Theme, lbl string, ed *widget.Editor, hint string) layout.Widget {
	return func(gtx C) D {
		return row(gtx,
			rigid(fixedW(140, smallLabel(th, lbl, colDim))),
			layout.Flexed(1, editor(th, ed, hint)),
		)
	}
}

func twoFieldRow(th *material.Theme, l1 string, e1 *widget.Editor, l2 string, e2 *widget.Editor) layout.Widget {
	return func(gtx C) D {
		return row(gtx,
			rigid(fixedW(120, smallLabel(th, l1, colDim))),
			layout.Flexed(1, editor(th, e1, "")),
			gap(8),
			rigid(fixedW(110, smallLabel(th, l2, colDim))),
			layout.Flexed(1, editor(th, e2, "")),
		)
	}
}

func checkBox(th *material.Theme, b *widget.Bool, txt string) layout.Widget {
	return func(gtx C) D {
		cb := material.CheckBox(th, b, txt)
		cb.Color = colFg
		cb.IconColor = colGreen
		cb.TextSize = unit.Sp(12)
		return cb.Layout(gtx)
	}
}
