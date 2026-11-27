package ui

import (
	"context"
	"fmt"
	"image"
	"sync"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"ragnarok-of-building/internal/sprite"
)

// charState is the character drawn in the middle of the equipment window
// (App.Sprites): how it looks, which the build does not keep, and the
// picture of the last look drawn.
type charState struct {
	Female     bool
	Hair       int // style, from 1 (0: the first)
	NoCostumes bool

	sex, prev, next widget.Clickable
	costumes        Checkbox

	mu    sync.Mutex
	key   string // the look drawn, or being drawn
	img   paint.ImageOp
	size  image.Point
	hairs int // hair styles for the look's job and sex
}

// look is what the character looks like now.
func (a *App) look() sprite.Look {
	s := &a.char
	b := a.Build()
	return sprite.Look{Job: b.Job, Female: s.Female, Hair: max(s.Hair, 1), Heads: a.V.HeadViews(b, !s.NoCostumes)}
}

// characterPicture lays the character out in a box of sz pixels, drawing
// it again in the background when its look changed. Without sprites (the
// desktop) it shows fallback instead.
func (a *App) characterPicture(gtx C, sz image.Point, fallback layout.Widget) D {
	s := &a.char
	if a.Sprites == nil {
		g := gtx
		g.Constraints = layout.Exact(sz)
		return layout.Center.Layout(g, fallback)
	}
	l := a.look()
	key := fmt.Sprint(l)
	s.mu.Lock()
	img, isz, drawn := s.img, s.size, s.key
	s.mu.Unlock()
	if drawn != key {
		s.mu.Lock()
		s.key = key
		s.mu.Unlock()
		go func() {
			ctx := context.Background()
			pic, err := a.Sprites.Character(ctx, l)
			hairs := a.Sprites.Hairs(ctx, l.Job, l.Female)
			s.mu.Lock()
			if s.key == key { // not changed again meanwhile
				s.hairs = hairs
				if err == nil && pic != nil {
					s.img = paint.NewImageOp(pic)
					s.img.Filter = paint.FilterNearest // pixel art
					s.size = pic.Rect.Size()
				} else {
					s.img, s.size = paint.ImageOp{}, image.Point{}
				}
			}
			s.mu.Unlock()
			if a.Invalidate != nil {
				a.Invalidate()
			}
		}()
	}
	if isz.X == 0 || isz.Y == 0 {
		g := gtx
		g.Constraints = layout.Exact(sz)
		return layout.Center.Layout(g, fallback)
	}
	// The largest whole scale that fits, so the pixels stay sharp; the
	// feet near the bottom, as the client stands its characters.
	k := max(1, min(sz.X/isz.X, (sz.Y-gtx.Dp(6))/isz.Y))
	w, h := isz.X*k, isz.Y*k
	at := image.Pt((sz.X-w)/2, sz.Y-h-gtx.Dp(4))
	st := op.Offset(at).Push(gtx.Ops)
	sc := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(float32(k), float32(k)))).Push(gtx.Ops)
	img.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	sc.Pop()
	st.Pop()
	return D{Size: sz}
}

// characterControls are the look's choices: sex, hair style, and whether
// the costumes show over the headgears.
func (a *App) characterControls(gtx C) D {
	th, s := a.Th, &a.char
	if a.Sprites == nil {
		return D{}
	}
	changed := false
	if s.sex.Clicked(gtx) {
		s.Female, s.Hair, changed = !s.Female, 1, true
	}
	s.mu.Lock()
	hairs := s.hairs
	s.mu.Unlock()
	cur := max(s.Hair, 1)
	if s.prev.Clicked(gtx) && hairs > 0 {
		s.Hair, changed = (cur+hairs-2)%hairs+1, true
	}
	if s.next.Clicked(gtx) && hairs > 0 {
		s.Hair, changed = cur%hairs+1, true
	}
	sexLabel := a.T("Masculino")
	if s.Female {
		sexLabel = a.T("Feminino")
	}
	d := Col(4,
		Row(4,
			func(gtx C) D { return Button{Label: "‹", MinWidth: 18}.Layout(gtx, th, &s.prev) },
			T(th, fmt.Sprintf("%s %d", a.T("Cabelo"), max(s.Hair, 1))).W(),
			func(gtx C) D { return Button{Label: "›", MinWidth: 18}.Layout(gtx, th, &s.next) },
		),
		func(gtx C) D { return Button{Label: sexLabel}.Layout(gtx, th, &s.sex) },
		func(gtx C) D {
			d, toggled := s.costumes.Layout(gtx, th, !s.NoCostumes, a.T("Mostrar visuais"))
			if toggled {
				s.NoCostumes, changed = !s.NoCostumes, true
			}
			return d
		},
	)(gtx)
	if changed {
		a.mu.Lock()
		a.needSave = true
		a.mu.Unlock()
	}
	return d
}
