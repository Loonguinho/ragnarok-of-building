package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// The single window (the client's inventory, full screen): big side tabs
// on the left; inside, the character (info and attributes) fixed on the
// left, the Resumo fixed on the right and the tab's window in the middle.
// Nothing scrolls but the three columns, each inside itself.

type shellState struct {
	undo   [2]widget.Clickable
	btns   []widget.Clickable
	cur    *Window
	left   ScrollList
	mid    ScrollList
	right  ScrollList
	skill  Select
	pick   skillPick
	target widget.Clickable
	cmds   [5]widget.Clickable
	midFor *Window // the tab the middle scroll is at
	// weaponEle: the summary's "use the weapon's element" button.
	weaponEle widget.Clickable
}

type shellTab struct {
	w     *Window
	label string
	icon  int // item whose icon the tab shows
}

func (a *App) shellTabs() []shellTab {
	return []shellTab{
		{a.wSkills, "Habilidades", 717}, {a.wEquip, "Equipamento", 2301}, {a.wBuffs, "Buffs", 505},
		{a.wDamage, "Cálculos", 1101}, {a.wOptimize, "Otimizador", 984}, {a.wRdmg, "Replay", 603},
		{a.wItems, "Banco de itens", 644}, {a.wSaved, "Builds", 617}, {a.wShare, "Compartilhar", 610},
		{a.wSettings, "Configurações", 7036},
	}
}

// shellSync keeps one tab window open: one opened from elsewhere (a hotkey,
// a button) becomes the tab shown; the one shown stays open.
func (a *App) shellSync() {
	s := &a.shell
	tabs := a.shellTabs()
	if s.cur == nil {
		s.cur = tabs[0].w
	}
	prev := s.cur
	for _, t := range tabs {
		if t.w.Open && t.w != prev {
			s.cur = t.w
			a.Desk.changed = true
		}
	}
	for _, t := range tabs {
		t.w.Open = t.w == s.cur
	}
}

// shellSelect shows tab window w.
func (a *App) shellSelect(w *Window) {
	for _, t := range a.shellTabs() {
		t.w.Open = t.w == w
	}
	a.shell.cur = w
	a.Desk.changed = true
}

const (
	shellTabW  = unit.Dp(150)
	shellTabH  = unit.Dp(40)
	shellLeftW = unit.Dp(380)
	shellResW  = unit.Dp(390)
)

// shellLayout draws the single window over the whole desk.
func (a *App) shellLayout(gtx C) D {
	a.shellSync()
	s := &a.shell
	tabs := a.shellTabs()
	if len(s.btns) != len(tabs) {
		s.btns = make([]widget.Clickable, len(tabs))
	}
	for i, t := range tabs {
		if s.btns[i].Clicked(gtx) && s.cur != t.w {
			a.shellSelect(t.w)
		}
	}
	for i := range s.cmds {
		if s.cmds[i].Clicked(gtx) {
			a.menuCommand(i)
		}
	}
	if s.target.Clicked(gtx) {
		a.shellSelect(a.wDamage)
	}

	size := gtx.Constraints.Max
	full := image.Rectangle{Max: size} // the whole screen, no border
	footH := gtx.Dp(36)
	Fill(gtx, full, ColBody)

	// Footer: the build commands.
	fr := image.Rect(full.Min.X, full.Max.Y-footH, full.Max.X, full.Max.Y)
	Fill(gtx, fr, ColBodyAlt)
	Fill(gtx, image.Rect(fr.Min.X, fr.Min.Y, fr.Max.X, fr.Min.Y+1), ColSep)
	a.at(gtx, fr.Min.Add(image.Pt(gtx.Dp(10), gtx.Dp(7))), image.Pt(fr.Dx()-gtx.Dp(20), footH), a.shellFooter)

	// Tabs and the panel they open into.
	body := image.Rect(full.Min.X, full.Min.Y, full.Max.X, fr.Min.Y)
	tabX := body.Min.X + gtx.Dp(10)
	panel := image.Rect(tabX+gtx.Dp(shellTabW), body.Min.Y+gtx.Dp(10), body.Max.X-gtx.Dp(10), body.Max.Y-gtx.Dp(10))
	FillRound(gtx, panel, ColBody, 0, gtx.Dp(4), gtx.Dp(4), gtx.Dp(4))
	Border(gtx, panel, ColListBorder, 1, gtx.Dp(4))
	for i, t := range tabs {
		y := panel.Min.Y + gtx.Dp(6) + i*(gtx.Dp(shellTabH)+gtx.Dp(4))
		r := image.Rectangle{Min: image.Pt(tabX, y), Max: image.Pt(panel.Min.X+1, y+gtx.Dp(shellTabH))}
		a.bigTab(gtx, &s.btns[i], r, t, t.w == s.cur)
	}

	// The three columns.
	in := panel.Inset(gtx.Dp(10))
	gap := gtx.Dp(12)
	leftW, resW := gtx.Dp(shellLeftW), gtx.Dp(shellResW)
	if in.Dx() < leftW+resW+gap*2+gtx.Dp(420) { // a small screen: narrower sides
		leftW, resW = gtx.Dp(330), gtx.Dp(300)
	}
	h := in.Dy()
	a.at(gtx, in.Min, image.Pt(leftW, h), func(gtx C) D { return a.shellColumn(gtx, &s.left, a.shellLeft) })
	midX := in.Min.X + leftW + gap
	midW := in.Max.X - resW - gap - midX
	if s.midFor != s.cur {
		s.midFor = s.cur
		s.mid.List.Position = layout.Position{}
	}
	a.at(gtx, image.Pt(midX, in.Min.Y), image.Pt(midW, h), a.shellMiddle)
	a.at(gtx, image.Pt(in.Max.X-resW, in.Min.Y), image.Pt(resW, h), func(gtx C) D { return a.shellColumn(gtx, &s.right, a.resumoBody) })
	return D{Size: size}
}

// at lays w at p with exactly size sz.
func (a *App) at(gtx C, p, sz image.Point, w layout.Widget) {
	st := op.Offset(p).Push(gtx.Ops)
	g := gtx
	g.Constraints = layout.Exact(sz)
	g.Constraints.Min.Y = 0
	w(g)
	st.Pop()
}

// shellColumn scrolls its content only when it is taller than the column.
func (a *App) shellColumn(gtx C, sl *ScrollList, w layout.Widget) D {
	h := gtx.Constraints.Max.Y
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints.Max.Y = 1 << 20
	d := w(g)
	call := m.Stop()
	if d.Size.Y <= h && sl.List.Position.First == 0 && sl.List.Position.Offset == 0 {
		call.Add(gtx.Ops)
		return D{Size: image.Pt(gtx.Constraints.Max.X, h)}
	}
	gtx.Constraints.Min.Y = h
	return sl.Layout(gtx, 1, func(gtx C, _ int) D { return w(gtx) })
}

// card is a white box (the mockup's panels).
func card(gtx C, w layout.Widget) D {
	return Box{Bg: ColPanel, Line: ColListBorder, Radius: 3, Pad: Inset(8, 10, 8, 10)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return w(gtx)
	})
}

func (a *App) bigTab(gtx C, cl *widget.Clickable, r image.Rectangle, t shellTab, on bool) {
	th := a.Th
	st := op.Offset(r.Min).Push(gtx.Ops)
	defer st.Pop()
	sz := r.Size()
	g := gtx
	g.Constraints = layout.Exact(sz)
	cl.Layout(g, func(gtx C) D {
		rr := image.Rectangle{Max: sz}
		switch {
		case on:
			FillRound(gtx, rr, ColBody, gtx.Dp(5), 0, 0, gtx.Dp(5))
			Border(gtx, image.Rectangle{Max: image.Pt(sz.X+gtx.Dp(5), sz.Y)}, ColListBorder, 1, gtx.Dp(5))
			Fill(gtx, image.Rect(sz.X-1, 1, sz.X+gtx.Dp(5), sz.Y-1), ColBody)
		default:
			top, bot := colTabOff, colTabOffB
			if cl.Hovered() {
				top, bot = ColHover, colTabOff
			}
			VGradient(gtx, image.Rectangle{Max: image.Pt(sz.X-1, sz.Y)}, top, bot, gtx.Dp(5), 0, 0, gtx.Dp(5))
			Border(gtx, image.Rectangle{Max: image.Pt(sz.X-1, sz.Y)}, ColListBorder, 1, gtx.Dp(5))
		}
		inner := op.Offset(image.Pt(gtx.Dp(8), (sz.Y-gtx.Dp(24))/2)).Push(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(sz.X-gtx.Dp(12), gtx.Dp(24))}
		label := T(th, a.T(t.label)).C(ColLabel)
		if on {
			label = B(th, a.T(t.label)).C(ColTitleText)
		}
		Row(6,
			func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(t.icon), 24) },
			func(gtx C) D {
				return layout.Center.Layout(gtx, label.Lines(1).W())
			},
		)(g)
		inner.Pop()
		pointerCursor(gtx, sz)
		return D{Size: sz}
	})
}

// shellLeft: the character, always on screen, and the chat under it.
func (a *App) shellLeft(gtx C) D {
	return Col(10,
		func(gtx C) D { return card(gtx, a.basicBody) },
		func(gtx C) D { return card(gtx, a.statusBody) },
		a.chatBox,
	)(gtx)
}

// shellMiddle is the current tab's window: its title, body and footer.
func (a *App) shellMiddle(gtx C) D {
	th, s := a.Th, &a.shell
	w := s.cur
	h := gtx.Constraints.Max.Y
	// Habilidades keeps its points and Resetar right under the tree (in the
	// body below), not at the bottom of the column.
	underBody := w == a.wSkills
	footer := func(gtx C) D {
		if w.Footer == nil || underBody {
			return D{}
		}
		return Inset(6, 0, 0, 0).Layout(gtx, w.Footer)
	}
	// The footer stays under the scrolling body.
	fm := op.Record(gtx.Ops)
	fd := footer(gtx)
	fcall := fm.Stop()
	head := func(gtx C) D {
		return Col(4, B(th, w.Title).C(ColTitleText).W(), Sep)(gtx)
	}
	hm := op.Record(gtx.Ops)
	hd := head(gtx)
	hcall := hm.Stop()
	hcall.Add(gtx.Ops)
	bodyH := h - hd.Size.Y - fd.Size.Y - gtx.Dp(4)
	func() {
		st := op.Offset(image.Pt(0, hd.Size.Y+gtx.Dp(4))).Push(gtx.Ops)
		defer st.Pop()
		g := gtx
		g.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, max(bodyH, 0)))
		g.Constraints.Min.Y = 0
		a.shellColumn(g, &s.mid, func(gtx C) D {
			if w.Body == nil {
				return D{}
			}
			if w == a.wEquip { // the gear and what it gives, together
				return Col(8, w.body, Sep, a.bonusBody)(gtx)
			}
			if underBody && w.Footer != nil {
				return Col(8, w.body, w.Footer)(gtx)
			}
			return w.body(gtx)
		})
	}()
	st := op.Offset(image.Pt(0, h-fd.Size.Y)).Push(gtx.Ops)
	fcall.Add(gtx.Ops)
	st.Pop()
	return D{Size: image.Pt(gtx.Constraints.Max.X, h)}
}

func (a *App) shellFooter(gtx C) D {
	th, s := a.Th, &a.shell
	labels := []string{"Salvar", "Builds", "Novo", "Compartilhar", "Importar replay"}
	var ws []layout.Widget
	ws = append(ws, func(gtx C) D {
		info := a.T("Build não salvo")
		if a.savedID != "" {
			info = a.T("Build salvo")
		}
		return T(th, info).C(ColLabel).W()(gtx)
	})
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(ws[0]),
		layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
		layout.Rigid(func(gtx C) D {
			bs := []layout.Widget{func(gtx C) D { return a.undoButtons(gtx, &s.undo) }}
			for i, l := range labels {
				i, l := i, l
				bs = append(bs, func(gtx C) D { return Button{Label: a.T(l)}.Layout(gtx, th, &s.cmds[i]) })
			}
			return Row(6, bs...)(gtx)
		}),
	)
}

// resumoBody is the Resumo: the main attack against the target and the
// character's numbers, always on screen.
func (a *App) resumoBody(gtx C) D {
	th, s := a.Th, &a.shell
	v := a.View()
	d, r, b := v.D, v.R, v.Build
	cv, comparing := a.compareView()
	num := func(n int64) string { return view.FmtInt(n) }
	label := func(t string) layout.Widget { return T(th, a.T(t)).C(ColLabel).W() }

	skillSel := func(gtx C) D { return a.skillPicker(gtx, b, d) }
	target := func(gtx C) D {
		name := a.T("Nenhum alvo")
		var icon layout.Widget = Gap(0, 0)
		var tags layout.Widget = Gap(0, 0)
		if d.Target != nil {
			name = d.Target.NameIn(a.Lang)
			id := d.Target.ID
			icon = func(gtx C) D { return a.Icons.Icon(gtx, view.MonsterIcon(id), 28) }
			tags = T(th, d.TargetTags).Sz(SmallText).C(ColLabel).Lines(1).W() // race · element · size
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(icon),
			layout.Rigid(Gap(6, 0)),
			layout.Flexed(1, Col(1, B(th, name).Lines(2).W(), tags)),
			layout.Rigid(func(gtx C) D { return Button{Label: a.T("Trocar")}.Layout(gtx, th, &s.target) }),
		)
	}
	hero := func(gtx C) D {
		return Box{Bg: ColHover, Line: ColSelect, Radius: 3, Pad: Inset(8, 10, 8, 10)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if d.Target == nil {
				return T(th, a.T("Escolha um alvo para ver o dano.")).C(ColMuted).Wrapping().W()(gtx)
			}
			dm := d.Dmg
			expected, kills := expectedDamage(d)
			// With the equipment window's comparison: "now -> compared".
			cmp := comparing && cv.D.Target != nil
			cdm := cv.D.Dmg
			cexp, ckills := expectedDamage(cv.D)
			arrow := T(th, "→").C(ColMuted).W()
			title := T(th, a.T("Dano esperado por uso")).C(ColMuted).W()
			big := B(th, num(expected)).Sz(22).C(ColTitleText).W()
			normal := T(th, a.T("Normal")+": "+num(dm.Min)+" ~ "+num(dm.Max)).C(ColLabel).W()
			if cmp {
				title = func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, T(th, a.T("Dano esperado por uso")).C(ColMuted).W()),
						layout.Rigid(T(th, a.compareSummary()).Sz(SmallText).C(ColCompare).W()),
					)
				}
				c := betterColor(float64(expected), float64(cexp), false)
				big = Row(8, big, T(th, "→").Sz(16).C(ColMuted).W(), B(th, num(cexp)).Sz(22).C(c).W(),
					B(th, gainText(float64(expected), float64(cexp), a.Lang)).Sz(13).C(c).W())
				nc := betterColor(float64(dm.Min+dm.Max), float64(cdm.Min+cdm.Max), false)
				normal = Row(4, normal, arrow, T(th, num(cdm.Min)+" ~ "+num(cdm.Max)).C(nc).W())
			}
			ws := []layout.Widget{title, big, normal}
			var extra []layout.Widget
			if dm.Crit > 0 || cmp && cdm.Crit > 0 {
				parts := []layout.Widget{label("Crítico"), B(th, num(dm.Crit)).W()}
				if cmp && cdm.Crit != dm.Crit {
					parts = append(parts, arrow, B(th, num(cdm.Crit)).C(betterColor(float64(dm.Crit), float64(cdm.Crit), false)).W())
				}
				parts = append(parts, T(th, "("+strconv.FormatFloat(dm.CritChance, 'f', 0, 64)+"%)").C(ColMuted).W())
				extra = append(extra, Row(4, parts...))
			}
			killParts := []layout.Widget{label("Usos p/ matar"), B(th, strconv.Itoa(kills)).W()}
			if cmp && ckills != kills {
				killParts = append(killParts, arrow, B(th, strconv.Itoa(ckills)).C(betterColor(float64(kills), float64(ckills), true)).W())
			}
			hit := Row(4, label("Acerto"), B(th, strconv.FormatFloat(dm.HitChance, 'f', 0, 64)+"%").W())
			if cmp { // the arrows make the line too long for one row
				ws = append(ws, Row(14, extra...), Row(14, Row(4, killParts...), hit))
			} else {
				extra = append(extra, Row(4, killParts...), hit)
				ws = append(ws, Row(14, extra...))
			}
			if w := a.elementWarning(d); w != nil {
				ws = append(ws, w)
			}
			return Col(2, ws...)(gtx)
		})
	}
	// kv lists the stats; with the comparison, a stat it changes shows
	// "now -> compared", green when better and red when worse.
	kv := func(rows, other []statRow) layout.Widget {
		return func(gtx C) D {
			var ws []layout.Widget
			for i, rw := range rows {
				value := T(th, rw.text).W()
				if i < len(other) && other[i].text != rw.text {
					o := other[i]
					value = Row(4, value, T(th, "→").C(ColMuted).W(),
						T(th, o.text).C(betterColor(rw.val, o.val, rw.lowerBetter)).W())
				}
				name := rw.label
				ws = append(ws, func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1, T(th, a.T(name)).C(ColLabel).W()),
						layout.Rigid(value),
					)
				})
			}
			return Col(2, ws...)(gtx)
		}
	}
	offOf := func(r calc.Result) []statRow {
		ci := r.Cast()
		cast := a.T("instantânea")
		if ci.VarLeft > 0.05 {
			cast = fmt.Sprintf("−%.0f%%", 100-ci.VarLeft)
		}
		return []statRow{
			{"ATQ", fmt.Sprintf("%d + %d", r.StatusATK, r.EquipATK), float64(r.StatusATK + r.EquipATK), false},
			{"ATQM", fmt.Sprintf("%d + %d", r.MATKMin, r.EquipMATK), float64(r.MATKMin + r.EquipMATK), false},
			{"P.ATQ", fmt.Sprint(r.PATK), float64(r.PATK), false},
			{"S.ATQM", fmt.Sprint(r.SMATK), float64(r.SMATK), false},
			{"C.RATE", fmt.Sprint(r.CRATE), float64(r.CRATE), false},
			{"Crítico", fmt.Sprintf("%.1f", r.CRIT), r.CRIT, false},
			{"Precisão", fmt.Sprint(r.HIT), float64(r.HIT), false},
			{"ASPD", fmt.Sprint(r.ASPD), float64(r.ASPD), false},
			{"Conj. var.", cast, ci.VarLeft, true},
		}
	}
	defOf := func(r calc.Result) []statRow {
		return []statRow{
			{"HP", num(int64(r.HP)), float64(r.HP), false},
			{"SP", num(int64(r.SP)), float64(r.SP), false},
			{"DEF", fmt.Sprintf("%d + %d", r.HardDEF, r.SoftDEF), float64(r.HardDEF + r.SoftDEF), false},
			{"DEFM", fmt.Sprintf("%d + %d", r.HardMDEF, r.SoftMDEF), float64(r.HardMDEF + r.SoftMDEF), false},
			{"RES", fmt.Sprint(r.RES), float64(r.RES), false},
			{"MRES", fmt.Sprint(r.MRES), float64(r.MRES), false},
			{"Esquiva", fmt.Sprintf("%d + %d", r.FLEE, r.FLEE2), float64(r.FLEE + r.FLEE2), false},
			{"H.PLUS", fmt.Sprint(r.HPLUS), float64(r.HPLUS), false},
		}
	}
	var cOff, cDef []statRow
	if comparing {
		cOff, cDef = offOf(cv.R), defOf(cv.R)
	}
	off := kv(offOf(r), cOff)
	def := kv(defOf(r), cDef)
	sec := func(t string) layout.Widget { return Col(2, B(th, a.T(t)).C(ColTitleText).W(), Sep) }
	twoCols := func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, Col(3, sec("Ofensivo"), off)),
			layout.Rigid(Gap(14, 0)),
			layout.Flexed(1, Col(3, sec("Defensivo"), def)),
		)
	}
	extras := func(gtx C) D {
		ws := []layout.Widget{sec("Dano e resistências do equipamento")}
		for _, l := range v.Extras {
			l := l
			ws = append(ws, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, T(th, a.T(l.Label)).Wrapping().W()),
					layout.Rigid(Gap(6, 0)),
					layout.Rigid(B(th, l.Value).W()),
				)
			})
		}
		if len(v.Extras) == 0 {
			ws = append(ws, T(th, "—").C(ColMuted).W())
		}
		return Col(2, ws...)(gtx)
	}
	return Col(10,
		func(gtx C) D {
			return Box{Bg: ColPanel, Line: rgb(0x7d9bd1), Radius: 3, Pad: Inset(8, 10, 10, 10)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				ws := []layout.Widget{
					B(th, a.T("Resumo")).Sz(13).C(ColTitleText).W(),
					label("Habilidade principal"), skillSel,
					label("Alvo"), target,
					hero,
				}
				if fx := a.itemEffects(v); fx != nil {
					ws = append(ws, fx)
				}
				return Col(5, ws...)(gtx)
			})
		},
		func(gtx C) D { return card(gtx, twoCols) },
		func(gtx C) D { return card(gtx, extras) },
	)(gtx)
}

// elementWarning is shown when the attack's forced element (an endow, often
// imported from a replay against another monster) does no weapon damage to
// the target, with a button back to the weapon's element.
func (a *App) elementWarning(d view.DamageView) layout.Widget {
	b := a.View().Build
	if d.Target == nil || b.Atk.Element == "" || d.Dmg.EleAtkPct > 0 {
		return nil
	}
	th := a.Th
	msg := fmt.Sprintf(a.T("O ataque está com elemento %s, que não causa dano em %s (%s %d)."),
		a.T(view.ElementName(b.Atk.Element)), d.Target.NameIn(a.Lang), a.T(view.ElementName(d.Target.Element)), d.Target.ElementLv)
	return func(gtx C) D {
		if a.shell.weaponEle.Clicked(gtx) {
			a.Edit(func(b *calc.Build) { b.Atk.Element = "" })
		}
		return Col(4,
			T(th, msg).C(ColDown).Wrapping().W(),
			func(gtx C) D {
				return Button{Label: a.T("Usar o elemento da arma")}.Layout(gtx, th, &a.shell.weaponEle)
			},
		)(gtx)
	}
}

// statRow is a line of the Resumo's Ofensivo/Defensivo: its text and the
// value the comparison is judged by.
type statRow struct {
	label, text string
	val         float64
	lowerBetter bool
}

// expectedDamage is the Resumo's big number, the expected damage per use
// (crits and misses counted: a skill that always crits shows its crit, as
// a replay records it), and the uses to kill the target with it.
func expectedDamage(d view.DamageView) (int64, int) {
	dm := d.Dmg
	expected := int64(math.Round(dm.Expected))
	if expected <= 0 {
		expected = dm.Avg
	}
	kills := dm.HitsToKill
	if expected > 0 && d.Target != nil && d.Target.HP > 0 {
		kills = int((int64(d.Target.HP) + expected - 1) / expected)
	}
	return expected, kills
}

// betterColor is green when other beats now, red when it loses, the text
// colour when they are equal.
func betterColor(now, other float64, lowerBetter bool) color.NRGBA {
	switch {
	case other == now:
		return ColText
	case (other > now) != lowerBetter:
		return ColUp
	}
	return ColDown
}

// gainText is "▲ +6,9%" or "▼ −3,8%" from now to other ("=" when equal).
func gainText(now, other float64, lang string) string {
	if now <= 0 || other == now {
		return "="
	}
	g := (other/now - 1) * 100
	txt := strconv.FormatFloat(math.Abs(g), 'f', 1, 64)
	if lang != "en" {
		txt = strings.Replace(txt, ".", ",", 1)
	}
	if g > 0 {
		return "▲ +" + txt + "%"
	}
	return "▼ −" + txt + "%"
}
