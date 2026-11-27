package ui

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"ragnarok-of-building/internal/view"
)

// equippedTip is the tip of a worn item: the item as it is worn (refine,
// grade), its cards and enchants (the Pingente do Lobo Cinzento's orbs...),
// its random options, then its description.
func (a *App) equippedTip(gtx C, sv view.SlotView) D {
	th := a.Th
	it := sv.Item
	if it == nil {
		return D{}
	}
	gtx.Constraints.Max.X = gtx.Dp(300)
	gtx.Constraints.Min.X = gtx.Dp(300)
	section := func(title string, rows []layout.Widget) layout.Widget {
		return func(gtx C) D {
			return Inset(4, 6, 0, 6).Layout(gtx, Col(2, append([]layout.Widget{B(th, title).Sz(SmallText).C(ColLabel).W()}, rows...)...))
		}
	}
	var cards, enchants []layout.Widget
	for i, c := range sv.Cards {
		if c == nil {
			continue
		}
		id, name := c.ID, c.Name
		row := func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(id), 20) }),
				layout.Rigid(Gap(5, 0)),
				layout.Flexed(1, T(th, name).Wrapping().W()),
			)
		}
		if i < sv.CardSlots {
			cards = append(cards, row)
		} else {
			enchants = append(enchants, row)
		}
	}
	var opts []layout.Widget
	for _, o := range sv.Opts {
		opts = append(opts, T(th, o.Label).C(ColUp).Wrapping().W())
	}

	m := op.Record(gtx.Ops)
	kids := []layout.Widget{
		func(gtx C) D {
			mm := op.Record(gtx.Ops)
			hd := Inset(3, 6, 3, 6).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				name := itemTitle(it, sv.Refine, sv.Grade)
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.Icons.Art(gtx, it.ID, 48) }),
					layout.Rigid(Gap(6, 0)),
					layout.Flexed(1, B(th, name).Wrapping().W()),
					layout.Rigid(T(th, fmt.Sprintf("#%d", it.ID)).Sz(SmallText).C(rgb(0x666677)).W()),
				)
			})
			call := mm.Stop()
			r := image.Rectangle{Max: hd.Size}
			VGradient(gtx, r, rgb(0xe9eef8), rgb(0xcfd8ea), 3, 3, 0, 0)
			call.Add(gtx.Ops)
			Fill(gtx, image.Rect(0, r.Max.Y-1, r.Max.X, r.Max.Y), ColListBorder)
			return hd
		},
	}
	if len(cards) > 0 {
		kids = append(kids, section(a.T("Cartas"), cards))
	}
	if len(enchants) > 0 {
		kids = append(kids, section(a.T("Encantamentos"), enchants))
	}
	if len(opts) > 0 {
		kids = append(kids, section(a.T("Bônus aleatórios"), opts))
	}
	if len(cards)+len(enchants)+len(opts) > 0 {
		kids = append(kids, func(gtx C) D {
			return Inset(5, 6, 0, 6).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return Sep(gtx)
			})
		})
	}
	// The slot's item comes without its description (the site loads them
	// apart): it is read as the other item tips read it.
	desc := it.Desc
	if full := a.V.Item(it.ID, a.Lang); full != nil {
		desc = full.Desc
	}
	kids = append(kids, func(gtx C) D {
		return Inset(5, 6, 5, 6).Layout(gtx, RichText(th, desc, rgb(0x000000)))
	})
	d := Col(0, kids...)(gtx)
	call := m.Stop()
	r := image.Rectangle{Max: d.Size}
	FillRound(gtx, r, ColPanel, 3, 3, 3, 3)
	call.Add(gtx.Ops)
	Border(gtx, r, rgb(0x8a94a8), 1, 3)
	return d
}
