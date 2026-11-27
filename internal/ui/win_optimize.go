package ui

import (
	"context"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/edit"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/view"
)

// ---- Otimizador (win-optimize) ----

type optimizeState struct {
	goal              Select
	ref, gra          Select
	items, cards, enc Checkbox
	mvp, rolls, cos   Checkbox
	noOpts, noCos     bool
	lvl, soon         Checkbox
	anyLevel, preview bool
	tabs              Tabs
	tab               int // what the results show: an index of optTabs
	moreBtns          map[string]*widget.Clickable
	more              map[string]int // options shown besides the first optShown, by group
	tiers             map[string]*widget.Clickable
	tra               Checkbox
	traits            bool // also search trait distributions
	// Unchecked boxes (the zero value tests everything).
	noItems, noCards, noEnch bool
	noMVP                    bool
	refine, grade            int
	fixRef, fixGra           bool // a refine or grade was chosen: not the worn item's
	goalVal                  string

	run    widget.Clickable
	busy   bool
	res    *view.OptResult
	apply  map[string]*widget.Clickable
	cmpEq  map[string]*widget.Clickable // "compare in the equipment window", by optKey
	rows   map[string]*widget.Clickable
	expand map[string]*widget.Clickable
	open   map[string]bool
}

// sameRefine, sameGrade: items are suggested at the worn item's.
func (s *optimizeState) sameRefine() bool { return !s.fixRef && s.refine == 0 }
func (s *optimizeState) sameGrade() bool  { return !s.fixGra && s.grade == 0 }

func (a *App) runOptimizer() {
	s := &a.opt
	a.mu.Lock()
	if s.busy {
		a.mu.Unlock()
		return
	}
	s.busy = true
	b, lang := a.b.Clone(), a.Lang
	o := view.OptOptions{Goal: s.goalVal, Items: !s.noItems, Cards: !s.noCards, Enchants: !s.noEnch, NoMVP: s.noMVP, SameRefine: s.sameRefine(), SameGrade: s.sameGrade(), AnyLevel: s.anyLevel, Preview: s.preview, Opts: !s.noOpts, Costumes: !s.noCos, Refine: s.refine, Grade: s.grade, Traits: s.traits}
	a.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res := a.V.Optimize(ctx, b, o, lang)
		a.mu.Lock()
		s.res, s.busy = &res, false
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

// optKey names an option's widgets.
func optKey(o view.OptOption) string {
	return fmt.Sprintf("%s/%s/%d/%d/%d", o.Kind, o.Slot, o.Index, o.Item.ID, o.Opt[0])
}

func (a *App) optimizeBody(gtx C) D {
	th, s := a.Th, &a.opt
	if s.run.Clicked(gtx) {
		a.runOptimizer()
	}
	a.mu.Lock()
	res, busy := s.res, s.busy
	a.mu.Unlock()
	if s.apply == nil {
		s.apply = map[string]*widget.Clickable{}
		s.rows = map[string]*widget.Clickable{}
		s.expand = map[string]*widget.Clickable{}
		s.open = map[string]bool{}
		s.tiers = map[string]*widget.Clickable{}
		s.moreBtns = map[string]*widget.Clickable{}
		s.more = map[string]int{}
	}
	if res != nil {
		for _, g := range res.Groups {
			for _, o := range g.Options {
				for _, x := range append([]view.OptOption{o}, o.Tiers...) {
					if cl := s.apply[optKey(x)]; cl != nil && cl.Clicked(gtx) {
						a.applyOpt(x)
					}
					if cl := s.cmpEq[optKey(x)]; cl != nil && cl.Clicked(gtx) {
						nb, _ := a.optBuild(x)
						a.compareFrom(nb)
					}
				}
			}
		}
	}
	goals := a.goalOptions()
	refs := []Option{{"cur", a.T("igual ao atual")}}
	for r := 0; r <= 20; r++ {
		refs = append(refs, Option{strconv.Itoa(r), "+" + strconv.Itoa(r)})
	}
	grades := []Option{{"cur", a.T("igual ao atual")}, {"0", "—"}, {"1", "D"}, {"2", "C"}, {"3", "B"}, {"4", "A"}}
	check := func(c *Checkbox, off *bool, label string) layout.Widget {
		return func(gtx C) D {
			d, toggled := c.Layout(gtx, th, !*off, label)
			if toggled {
				*off = !*off
			}
			return d
		}
	}
	on := func(c *Checkbox, v *bool, label string) layout.Widget {
		return func(gtx C) D {
			d, toggled := c.Layout(gtx, th, *v, label)
			if toggled {
				*v = !*v
			}
			return d
		}
	}
	if s.goalVal == "" {
		s.goalVal = view.GoalDamage
	}
	kids := []layout.Widget{
		T(th, a.T("Testa uma troca por vez (item, carta ou encantamento) contra o alvo da janela de dano e mostra a melhor de cada slot.")).Sz(SmallText).C(ColMuted).Wrapping().W(),
		Row(6, T(th, a.T("Objetivo")).W(), func(gtx C) D {
			d, v, ok := s.goal.Layout(gtx, th, a.Desk, goals, s.goalVal, 130)
			if ok {
				s.goalVal = v
			}
			return d
		}),
		Row(6,
			check(&s.items, &s.noItems, a.T("Itens")),
			T(th, a.T("Refino")).W(),
			func(gtx C) D {
				cur := strconv.Itoa(s.refine)
				if s.sameRefine() {
					cur = "cur"
				}
				d, v, ok := s.ref.Layout(gtx, th, a.Desk, refs, cur, 108)
				if ok {
					s.fixRef = v != "cur"
					s.refine, _ = strconv.Atoi(v)
				}
				return d
			},
			T(th, a.T("Grau")).W(),
			func(gtx C) D {
				cur := strconv.Itoa(s.grade)
				if s.sameGrade() {
					cur = "cur"
				}
				d, v, ok := s.gra.Layout(gtx, th, a.Desk, grades, cur, 108)
				if ok {
					s.fixGra = v != "cur"
					s.grade, _ = strconv.Atoi(v)
				}
				return d
			},
		),
		Row(10, check(&s.cards, &s.noCards, a.T("Cartas")), func(gtx C) D {
			if s.noCards { // nothing to leave out
				return D{}
			}
			return check(&s.mvp, &s.noMVP, a.T("Cartas MVP"))(gtx)
		}),
		check(&s.enc, &s.noEnch, a.T("Encantamentos (os que o cliente lista para cada item)")),
		Row(10, check(&s.rolls, &s.noOpts, a.T("Bônus aleatórios (valor máximo)")), check(&s.cos, &s.noCos, a.T("Visuais e conjuntos"))),
		Row(10, on(&s.lvl, &s.anyLevel, a.T("Itens acima do meu nível")), on(&s.soon, &s.preview, a.T("Itens em breve"))),
		func(gtx C) D {
			d, toggled := s.tra.Layout(gtx, th, s.traits, a.T("Considerar mudar os talentos"))
			if toggled {
				s.traits = !s.traits
			}
			return d
		},
		func(gtx C) D {
			label := a.T("Calcular")
			if busy {
				label = a.T("Calculando…")
			}
			return Button{Label: label, Disabled: busy, MinWidth: 80}.Layout(gtx, th, &s.run)
		},
	}
	if res != nil && res.Traits != nil {
		kids = append(kids, a.optTraits(res.Traits))
	}
	if res != nil {
		kids = append(kids, a.optResult(*res))
	}
	return Col(6, kids...)(gtx)
}

// optBuild is the build with a suggestion in it, and what the chat says.
func (a *App) optBuild(o view.OptOption) (calc.Build, string) {
	b := a.Build()
	switch o.Kind {
	case "item":
		return a.V.SwapAtOpts(b, o.Slot, o.Item.ID, o.Refine, o.Grade, o.Opts), a.T("Item equipado.")
	case "opt":
		return a.V.SetOpt(b, o.Slot, o.Index, o.Opt), a.T("Bônus aplicado.")
	case "set":
		return a.V.ApplyParts(b, o.Parts), a.T("Conjunto aplicado.")
	}
	idx, id := o.Index, o.Item.ID
	return edit.EditLinked(b, o.Slot, func(e *calc.Equip) { e.Cards[idx] = id }), a.T("Carta aplicada.")
}

// applyOpt puts a suggestion in the build and optimizes again.
func (a *App) applyOpt(o view.OptOption) {
	nb, msg := a.optBuild(o)
	a.SetBuild(nb)
	a.Say(msg, false)
	// Optimize again on the new build once it is computed.
	go func() {
		time.Sleep(300 * time.Millisecond)
		a.runOptimizer()
	}()
}

// optName is a suggestion's name with the refine and grade it was measured at.
func optName(o view.OptOption) string {
	if o.Name != "" {
		return o.Name
	}
	n := itemTitle(&o.Item, o.Refine, o.Grade)
	if o.Item.MVP {
		n += " (MVP)"
	}
	return n
}

func (a *App) optResult(r view.OptResult) layout.Widget {
	th, s := a.Th, &a.opt
	note := func(t string) layout.Widget { return T(th, t).Sz(SmallText).C(ColMuted).Wrapping().W() }
	switch {
	case r.NoTarget:
		return note(a.T("Escolha um alvo na janela de dano (Alt+D) primeiro."))
	case r.NoItems:
		return note(a.T("Nada para testar: marque itens, ou equipe itens com slot de carta."))
	}
	measure := a.T("Dano esperado atual")
	switch r.Goal {
	case view.GoalPhysDef:
		measure = a.T("HP efetivo atual vs golpes físicos do alvo")
	case view.GoalMagDef:
		measure = a.T("HP efetivo atual vs magias do alvo")
	case view.GoalMaxHP:
		measure = a.T("HP máx. atual")
	case view.GoalCRate:
		measure = a.T("C.Rate atual")
	}
	head := []layout.Widget{Row(4, T(th, a.T("Alvo")+":").W(), B(th, r.Target).W())}
	if r.Goal == view.GoalDamage || r.Goal == "" {
		head[0] = Row(4, T(th, a.T("Alvo")+":").W(), B(th, r.Target).W(), T(th, "· "+r.Attack).W())
	}
	if r.Goal == view.GoalMaxHP || r.Goal == view.GoalCRate { // no target in these
		head = nil
	}
	head = append(head, Row(4, T(th, measure+":").W(), B(th, r.Base).W()))
	kids := []layout.Widget{Col(1, head...)}
	if r.Partial {
		kids = append(kids, note(a.T("Tempo esgotado: alguns slots não foram testados.")))
	}
	// The results by kind.
	count := make([]int, len(optTabs))
	for _, g := range r.Groups {
		if len(g.Options) > 0 {
			count[0]++
			count[optTabOf(g)]++
		}
	}
	labels := make([]string, len(optTabs))
	for i, t := range optTabs {
		labels[i] = fmt.Sprintf("%s (%d)", a.T(t), count[i])
	}
	kids = append(kids, func(gtx C) D {
		d, picked, ok := s.tabs.Layout(gtx, th, labels, s.tab)
		if ok {
			s.tab = picked
		}
		return d
	})
	if count[s.tab] == 0 {
		kids = append(kids, note(a.T("Nada melhor do que o atual aqui.")))
	}
	var none []string
	for _, g := range r.Groups {
		g := g
		if s.tab != 0 && optTabOf(g) != s.tab {
			continue
		}
		if len(g.Options) == 0 {
			none = append(none, g.Label)
			continue
		}
		gk := fmt.Sprintf("%s/%s/%d", g.Kind, g.Slot, g.Index)
		if s.expand[gk] == nil {
			s.expand[gk] = &widget.Clickable{}
		}
		kids = append(kids, a.optGroupRows(g, gk))
	}
	if len(none) > 0 {
		kids = append(kids, note(a.T("Sem melhora")+": "+strings.Join(none, ", ")))
	}
	in := func(tab int) bool { return s.tab == 0 || s.tab == tab }
	for _, t := range r.NoSlot {
		if !in(2) {
			break
		}
		kids = append(kids, note(a.T("Conjunto da habilidade que não cabe")+": "+t))
	}
	if len(r.NoCostume) > 0 && in(5) {
		kids = append(kids, note(a.T("Sem visual equipado (equipe um para testar as pedras)")+": "+strings.Join(r.NoCostume, ", ")))
	}
	if len(r.NoEnch) > 0 && in(3) {
		kids = append(kids, note(a.T("Sem lista de encantamentos no cliente (encantados por NPC), não testados")+": "+strings.Join(r.NoEnch, ", ")))
	}
	kids = append(kids, note(fmt.Sprintf(a.T("%d combinações testadas. Bônus condicionais entram pela média, como na janela de dano."), r.Tested)))
	return Col(4, kids...)
}

// optGroupRows is a slot's line: its best swap, gain and Aplicar; ▸ shows
// what is there now and the other options.
func (a *App) optGroupRows(g view.OptGroup, gk string) layout.Widget {
	th, s := a.Th, &a.opt
	ex := s.expand[gk]
	cur := 0
	curName := a.T("vazio")
	if g.Current != nil {
		cur, curName = g.Current.ID, g.Current.Name
	}
	const labelW = 112
	var gtx0 C
	line := func(o view.OptOption, label layout.Widget, more bool) layout.Widget {
		k := optKey(o)
		if s.tiers[k] == nil {
			s.tiers[k] = &widget.Clickable{}
		}
		if len(o.Tiers) > 0 && s.tiers[k].Clicked(gtx0) {
			s.open["t/"+k] = !s.open["t/"+k]
		}
		if s.apply[k] == nil {
			s.apply[k] = &widget.Clickable{}
			s.rows[k] = &widget.Clickable{}
		}
		ap, hv := s.apply[k], s.rows[k]
		return func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(Fixed(labelW, label)),
				layout.Flexed(1, func(gtx C) D {
					d := hv.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.Icons.Art(gtx, o.Item.ID, 28) }),
							layout.Rigid(Gap(5, 0)),
							layout.Flexed(1, func(gtx C) D {
								ws := []layout.Widget{T(th, optName(o)).Lines(2).Wrapping().W()}
								if o.Kind == "set" && (o.Slot == "set" || o.Slot == "cards") {
									n := a.T("%d pedras")
									if o.Slot == "cards" {
										n = a.T("%d cartas")
									}
									ws = []layout.Widget{T(th, fmt.Sprintf(n, len(o.Parts))).W()}
								}
								if o.Skill != "" {
									ws = append(ws, T(th, a.T("em")+" "+o.Skill).Sz(SmallText).C(ColMuted).W())
								}
								if o.Name != "" && o.Kind == "opt" {
									ws = append(ws, T(th, o.Item.Name).Sz(SmallText).C(ColMuted).W())
									if o.OptMin != "" {
										ws = append(ws, T(th, a.T("no mínimo")+" "+o.OptMin).Sz(SmallText).C(ColLink).W())
									}
								}
								if o.Tag != "" {
									ws = append(ws, T(th, o.Tag).Sz(SmallText).C(ColLink).W())
								}
								for _, p := range o.Parts {
									if o.Slot == p.Slot { // one item's cards: the name says them
										break
									}
									ws = append(ws, T(th, p.SlotLabel+": "+p.CardName).Sz(SmallText).C(ColMuted).W())
								}
								return Col(0, ws...)(gtx)
							}),
							layout.Rigid(Gap(6, 0)),
							layout.Rigid(func(gtx C) D {
								c := ColUp
								switch {
								case o.Gain == "=" || o.Later || o.Gain == "+0,0%" || o.Gain == "+0.0%" || o.Gain == "+0":
									c = ColMuted
								case strings.HasPrefix(o.Gain, "-"):
									c = ColDown
								}
								return B(th, o.Gain).C(c).Layout(gtx)
							}),
						)
					})
					if hv.Hovered() {
						id := o.Item.ID
						if o.Kind == "item" {
							a.Desk.ShowTip(func(gtx C) D { return a.compareTip(gtx, cur, id, nil) })
						} else if o.Kind != "opt" {
							a.Desk.ShowTip(func(gtx C) D { return a.itemTip(gtx, id, nil) })
						}
					}
					return d
				}),
				layout.Rigid(Gap(6, 0)),
				layout.Rigid(func(gtx C) D {
					if len(o.Tiers) == 0 {
						return D{}
					}
					return Inset(0, 3, 0, 0).Layout(gtx, func(gtx C) D {
						return Button{Label: fmt.Sprintf(a.T("%d níveis"), len(o.Tiers)), Arrow: foldArrow(s.open["t/"+k])}.Layout(gtx, th, s.tiers[k])
					})
				}),
				layout.Rigid(func(gtx C) D {
					cl := clickable(&s.cmpEq, k)
					d := Button{Label: "↔", MinWidth: 22, Disabled: o.Gain == "="}.Layout(gtx, th, cl)
					if cl.Hovered() {
						a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(th, a.T("Comparar no equipamento")).W()) })
					}
					return d
				}),
				layout.Rigid(Gap(3, 0)),
				layout.Rigid(func(gtx C) D { return Button{Label: a.T("Aplicar"), Disabled: o.Gain == "="}.Layout(gtx, th, ap) }),
				layout.Rigid(func(gtx C) D {
					if !more {
						return Gap(22, 0)(gtx)
					}
					return Inset(0, 0, 0, 3).Layout(gtx, func(gtx C) D {
						return Button{MinWidth: 24, Arrow: foldArrow(s.open[optOpenKey(s.tab, gk)] != (s.tab != 0))}.Layout(gtx, th, ex)
					})
				}),
			)
		}
	}
	// withTiers is an option's line and, open, its levels.
	withTiers := func(o view.OptOption, label layout.Widget, more bool) []layout.Widget {
		rows := []layout.Widget{line(o, label, more)}
		under := ""
		if o.OptsText != "" {
			under = a.T("BA:") + " " + o.OptsText
		}
		byRefine, byGrade := a.stepsText(o)
		for _, t := range []string{under, byRefine, byGrade} {
			if t != "" { // a line of its own: it takes the whole width
				rows = append(rows, Pad(Inset(0, 0, 0, labelW), T(th, t).Sz(SmallText).C(ColLink).Wrapping().W()))
			}
		}
		if len(o.Tiers) > 0 && s.open["t/"+optKey(o)] {
			for _, t := range o.Tiers {
				rows = append(rows, Pad(Inset(0, 0, 0, 14), line(t, func(gtx C) D { return D{} }, false)))
			}
		}
		return rows
	}
	// In its own tab a group starts open; with everything, closed.
	ok := optOpenKey(s.tab, gk)
	isOpen := func() bool { return s.open[ok] != (s.tab != 0) }
	return func(gtx C) D {
		gtx0 = gtx
		if ex.Clicked(gtx) {
			s.open[ok] = !s.open[ok]
		}
		mb := s.moreBtns[gk]
		if mb == nil {
			mb = &widget.Clickable{}
			s.moreBtns[gk] = mb
		}
		if mb.Clicked(gtx) {
			s.more[gk] += optMore
		}
		rows := withTiers(g.Options[0], B(th, g.Label).Sz(SmallText).C(ColTitleText).Wrapping().W(), true)
		if isOpen() {
			if g.Kind != "set" {
				rows = append(rows, Pad(Inset(0, labelW, 0, 0), T(th, a.T("atual")+": "+curName).Sz(SmallText).C(ColMuted).W()))
			}
			left := 0
			for i, o := range g.Options[1:] {
				if i+1 >= optShown+s.more[gk] && o.Tag == "" && !o.Later {
					left++
					continue
				}
				rows = append(rows, withTiers(o, func(gtx C) D { return D{} }, false)...)
			}
			if left > 0 {
				rows = append(rows, Pad(Inset(2, 0, 2, labelW), func(gtx C) D {
					return Button{Label: fmt.Sprintf(a.T("Mostrar mais (%d)"), left), Arrow: ArrowDown}.Layout(gtx, th, mb)
				}))
			}
		} else { // the attack's own cards show without opening
			for _, o := range g.Options[1:] {
				if o.Tag != "" {
					rows = append(rows, withTiers(o, func(gtx C) D { return D{} }, false)...)
				}
			}
		}
		m := op.Record(gtx.Ops)
		d := Inset(3, 0, 3, 0).Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return Col(2, rows...)(gtx)
		})
		call := m.Stop()
		call.Add(gtx.Ops)
		Fill(gtx, image.Rect(0, d.Size.Y-1, d.Size.X, d.Size.Y), rgb(0xe3e6ec))
		return d
	}
}

// ---- Banco de Itens (win-items), after the Kafra storage ----

type itemsState struct {
	q       Input
	tabs    SideTabs
	filt    IconFilter
	sort    Select
	sortVal string
	scroll  ScrollList
	close   widget.Clickable
	allKey  string
	viewKey string
	all     []view.ItemView // the tab's results
	results []view.ItemView // after the icon filter and the order
	rows    map[int]*widget.Clickable
	shown   int // item whose description is shown below
}

// itemTab is one storage tab: an item type, its icon and its sub-filters
// (equip locations).
type itemTab struct {
	typ, label string
	icon       int
	subs       []itemSub
}

type itemSub struct {
	label string
	icon  int
	locs  []string
}

var itemTabs = []itemTab{
	{"", "Todos", 644, nil},
	{"weapon", "Armas", 1101, nil},
	{"armor", "Armaduras e acessórios", 2301, []itemSub{
		{"Armadura", 2301, []string{"Armor"}}, {"Escudo", 2101, []string{"Left_Hand"}},
		{"Capa", 2501, []string{"Garment"}}, {"Sapatos", 2401, []string{"Shoes"}},
		{"Acessório", 2601, []string{"Both_Accessory", "Right_Accessory", "Left_Accessory"}},
	}},
	{"head", "Chapéus", 2220, []itemSub{
		{"Topo", 2220, []string{"Head_Top"}}, {"Meio", 2201, []string{"Head_Mid"}}, {"Baixo", 2265, []string{"Head_Low"}},
	}},
	{"costume", "Visuais", 19500, []itemSub{
		{"Topo", 19500, []string{"Costume_Head_Top"}}, {"Meio", 19501, []string{"Costume_Head_Mid"}},
		{"Baixo", 19502, []string{"Costume_Head_Low"}}, {"Capa", 20500, []string{"Costume_Garment"}},
	}},
	{"card", "Cartas", 4001, nil},
	{"shadow", "Equip. sombrios", 24012, nil},
	{"other", "Outros", 501, nil},
}

// itemBankMax is how many items a tab lists at most.
const itemBankMax = 5000

func (a *App) itemsBody(gtx C) D {
	th, s := a.Th, &a.items
	if s.close.Clicked(gtx) {
		a.wItems.Open = false
		a.Desk.changed = true
	}
	tab := itemTabs[min(max(s.tabs.Cur, 0), len(itemTabs)-1)]
	key := s.q.Text() + "|" + tab.typ + "|" + a.Lang
	if key != s.allKey {
		s.allKey = key
		s.all = a.V.BrowseItems(strings.TrimSpace(s.q.Text()), tab.typ, a.Lang, itemBankMax)
	}
	// Icon filter and order, applied to the tab's results.
	vkey := key + "|" + fmt.Sprint(s.filt.On, len(s.all)) + "|" + s.sortVal
	if vkey != s.viewKey {
		s.viewKey = vkey
		s.results = nil
		var locs []string
		if s.filt.On >= 0 && s.filt.On < len(tab.subs) {
			locs = tab.subs[s.filt.On].locs
		}
		for _, it := range s.all {
			if len(locs) > 0 && !locIn(it.Loc, locs) {
				continue
			}
			s.results = append(s.results, it)
		}
		sortItems(s.results, s.sortVal)
	}
	if s.rows == nil {
		s.rows = map[int]*widget.Clickable{}
	}
	for _, it := range s.results {
		if cl := s.rows[it.ID]; cl != nil && cl.Clicked(gtx) {
			s.shown = it.ID
		}
	}

	var tabs []SideTab
	for _, t := range itemTabs {
		tabs = append(tabs, SideTab{Icon: view.ItemIcon(t.icon), Label: a.T(t.label)})
	}
	list := func(gtx C) D {
		h := gtx.Dp(360)
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
		if len(s.results) == 0 {
			msg := a.T("Digite o nome de um item (PT, ES ou EN) ou o ID, ou escolha uma aba.")
			if s.q.Text() != "" || tab.typ != "" {
				msg = a.T("Nenhum item encontrado.")
			}
			d := Inset(8, 8, 8, 8).Layout(gtx, T(th, msg).C(rgb(0x777777)).Wrapping().W())
			d.Size.Y = h
			return d
		}
		return s.scroll.Layout(gtx, len(s.results), func(gtx C, i int) D { return a.itemBankRow(gtx, s.results[i]) })
	}
	bottom := func(gtx C) D {
		var subs []SideTab
		for _, sb := range tab.subs {
			subs = append(subs, SideTab{Icon: view.ItemIcon(sb.icon), Label: a.T(sb.label)})
		}
		orders := []Option{{"", a.T("Padrão")}, {"az", "A-Z"}, {"za", "Z-A"}, {"id", "ID"}}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if len(subs) == 0 {
					return D{}
				}
				d, _ := s.filt.Layout(gtx, a, subs)
				return d
			}),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				d, val, ok := s.sort.Layout(gtx, th, a.Desk, orders, s.sortVal, 64)
				if ok {
					s.sortVal = val
				}
				return d
			}),
		)
	}
	kids := []layout.Widget{
		func(gtx C) D {
			d, _ := SearchBox(gtx, th, &s.q, a.T("Nome ou ID…"), 0)
			return d
		},
		func(gtx C) D {
			d, changed := s.tabs.Layout(gtx, a, tabs, Col(4, list, bottom))
			if changed {
				s.filt.On = -1
				s.scroll.List.Position = layout.Position{}
			}
			return d
		},
	}
	if it := a.V.Item(s.shown, a.Lang); it != nil {
		kids = append(kids, func(gtx C) D {
			return RowTop(8,
				func(gtx C) D {
					return Box{Bg: ColPanel, Line: ColListBorder, Pad: Inset(4, 4, 4, 4)}.Layout(gtx, func(gtx C) D {
						return a.Icons.Icon(gtx, view.ItemIcon(it.ID), 48)
					})
				},
				func(gtx C) D {
					gtx.Constraints.Max.X -= gtx.Dp(70)
					return Box{Bg: ColPanel, Line: ColListBorder, Pad: Inset(5, 6, 5, 6)}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						name := it.Name
						if it.Slots > 0 {
							name += fmt.Sprintf(" [%d]", it.Slots)
						}
						return Col(4, B(th, name).W(), RichText(th, it.Desc, rgb(0x000000)), T(th, fmt.Sprintf("ID %d", it.ID)).Sz(SmallText).C(ColMuted).W())(gtx)
					})
				},
			)(gtx)
		})
	}
	return Col(6, kids...)(gtx)
}

// itemsFooter is the storage's footer: the count and Fechar.
func (a *App) itemsFooter(gtx C) D {
	th, s := a.Th, &a.items
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(T(th, fmt.Sprintf("%d %s", len(s.results), a.T("itens"))).C(ColLabel).W()),
		layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
		layout.Rigid(func(gtx C) D { return Button{Label: a.T("Fechar")}.Layout(gtx, th, &s.close) }),
	)
}

// itemBankRow is one storage line: the icon, with the slots in its corner,
// and the name.
func (a *App) itemBankRow(gtx C, it view.ItemView) D {
	th, s := a.Th, &a.items
	cl := s.rows[it.ID]
	if cl == nil {
		cl = &widget.Clickable{}
		s.rows[it.ID] = cl
	}
	d := cl.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		m := op.Record(gtx.Ops)
		d := Inset(2, 4, 2, 4).Layout(gtx, func(gtx C) D {
			name := it.Name
			if it.MVP {
				name += " (MVP)"
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Stack{Alignment: layout.SW}.Layout(gtx,
						layout.Stacked(func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(it.ID), 24) }),
						layout.Stacked(func(gtx C) D {
							if it.Slots == 0 {
								return D{}
							}
							return T(th, fmt.Sprint(it.Slots)).Sz(SmallText).C(ColText).W()(gtx)
						}),
					)
				}),
				layout.Rigid(Gap(6, 0)),
				layout.Flexed(1, T(th, name).Lines(1).W()),
			)
		})
		call := m.Stop()
		switch {
		case it.ID == s.shown:
			Fill(gtx, image.Rectangle{Max: d.Size}, ColSelect)
		case cl.Hovered():
			Fill(gtx, image.Rectangle{Max: d.Size}, ColHover)
		}
		call.Add(gtx.Ops)
		pointerCursor(gtx, d.Size)
		return d
	})
	if cl.Hovered() && it.ID != s.shown {
		id := it.ID
		a.Desk.ShowTip(func(gtx C) D { return a.itemTip(gtx, id, nil) })
	}
	return d
}

// locIn reports whether a comma-separated location list has one of locs.
func locIn(loc string, locs []string) bool {
	for _, l := range strings.Split(loc, ",") {
		for _, w := range locs {
			if l == w {
				return true
			}
		}
	}
	return false
}

// sortItems orders the item bank: "az", "za" (accents folded), "id", or
// "" for the search's own order.
func sortItems(items []view.ItemView, by string) {
	if by == "" {
		return
	}
	keys := make(map[int]string, len(items))
	for _, it := range items {
		keys[it.ID] = gamedata.Normalize(it.Name)
	}
	slices.SortStableFunc(items, func(x, y view.ItemView) int {
		switch by {
		case "az":
			return strings.Compare(keys[x.ID], keys[y.ID])
		case "za":
			return strings.Compare(keys[y.ID], keys[x.ID])
		}
		return x.ID - y.ID
	})
}

// stepsText says what the option gives at higher refines, and at higher
// grades, of its item ("" when there is none).
func (a *App) stepsText(o view.OptOption) (refine, grade string) {
	if len(o.Steps) == 0 {
		return "", ""
	}
	names := [5]string{"", "D", "C", "B", "A"}
	pct := func(t string) string { return strings.TrimPrefix(t, "+") } // +9 is the refine's plus
	var r, g []string
	worn := a.Build().Equip[o.Slot]
	for _, st := range o.Steps {
		if st.Grade == worn.Grade {
			r = append(r, fmt.Sprintf("[+%d] %s", st.Refine, pct(st.Gain)))
			continue
		}
		at := names[st.Grade]
		if st.Refine > worn.Refine {
			at += fmt.Sprintf(" +%d", st.Refine)
		}
		g = append(g, fmt.Sprintf("[%s] %s", at, pct(st.Gain)))
	}
	if len(r) > 0 {
		refine = a.T("Ganho com refino maior:") + " " + strings.Join(r, " · ")
	}
	if len(g) > 0 {
		grade = a.T("Ganho com grau maior:") + " " + strings.Join(g, " · ")
	}
	return refine, grade
}

// The result tabs, and how many options a group shows before "Mostrar mais".
var optTabs = []string{"Tudo", "Equip.", "Cartas", "Encant.", "BAs", "Visuais"}

const (
	optShown = 6
	optMore  = 10
)

// optTabOf is the tab a group of results is in.
func optTabOf(g view.OptGroup) int {
	costume := strings.HasPrefix(g.Slot, "Costume_")
	switch {
	case costume || g.Kind == "set" && g.Slot == "set":
		return 5
	case g.Kind == "item":
		return 1
	case g.Kind == "enchant":
		return 3
	case g.Kind == "opt":
		return 4
	}
	return 2 // cards, their sets, and one item's cards together
}

// optOpenKey is where a group's folding is kept: "everything" and the tabs
// have their own.
func optOpenKey(tab int, gk string) string {
	if tab != 0 {
		return "closed/" + gk
	}
	return gk
}
