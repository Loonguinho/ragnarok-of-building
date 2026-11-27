package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/widget"

	"ragnarok-of-building/internal/edit"
	"ragnarok-of-building/internal/view"
)

// itemEffects is the Resumo's "Efeitos de itens": the effects the
// equipped items trigger (on attacking, on being hit, on using a skill),
// one button each that turns it on or off. They count on both sides of
// the equipment window's comparison, which sits on the build. Nothing to
// show without any.
func (a *App) itemEffects(v view.BuildView) layout.Widget {
	if len(v.Buffs.Items) == 0 {
		return nil
	}
	th := a.Th
	if a.fxBtns == nil {
		a.fxBtns = map[string]*widget.Clickable{}
	}
	return func(gtx C) D {
		ws := []layout.Widget{
			Row(6, B(th, a.T("Efeitos de itens")).C(ColTitleText).W(),
				T(th, a.T("clique para ligar ou desligar")).Sz(SmallText).C(ColMuted).W()),
		}
		for _, it := range v.Buffs.Items {
			it := it
			cl := clickable(&a.fxBtns, it.Key)
			on := it.Value > 0
			if cl.Clicked(gtx) {
				val := 1
				if on {
					val = 0
				}
				a.SetBuild(edit.SetBuff(a.Build(), it.Key, val, nil))
			}
			ws = append(ws, func(gtx C) D {
				return cl.Layout(gtx, func(gtx C) D {
					bg, line, gives, tag := ColBodyAlt, ColListBorder, ColMuted, a.T("DESLIGADO")
					if on {
						bg, line, gives, tag = ColHover, ColOn, ColUp, a.T("LIGADO")
					}
					d := Box{Bg: bg, Line: line, Radius: 3, Pad: Inset(3, 6, 3, 6)}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(it.ItemID), 20) }),
							layout.Rigid(Gap(6, 0)),
							layout.Flexed(1, Row(6, T(th, it.Label).Lines(1).W(), T(th, it.Gives).C(gives).Lines(1).W())),
							layout.Rigid(B(th, tag).Sz(TinyText).C(gives).W()),
						)
					})
					pointerCursor(gtx, image.Pt(d.Size.X, d.Size.Y))
					return d
				})
			})
		}
		return Col(3, ws...)(gtx)
	}
}
