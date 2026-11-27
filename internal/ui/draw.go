package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Drawing helpers: rectangles, gradients and borders like the CSS boxes.

type (
	C = layout.Context
	D = layout.Dimensions
)

// Fill paints r with c.
func Fill(gtx C, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
}

// FillRound paints r with rounded corners (radii in px: NW, NE, SE, SW).
func FillRound(gtx C, r image.Rectangle, c color.NRGBA, nw, ne, se, sw int) {
	paint.FillShape(gtx.Ops, c, clip.RRect{Rect: r, NW: nw, NE: ne, SE: se, SW: sw}.Op(gtx.Ops))
}

// VGradient paints r with a top-to-bottom gradient, clipped to the rounded
// rectangle.
func VGradient(gtx C, r image.Rectangle, top, bottom color.NRGBA, nw, ne, se, sw int) {
	defer clip.RRect{Rect: r, NW: nw, NE: ne, SE: se, SW: sw}.Push(gtx.Ops).Pop()
	paint.LinearGradientOp{
		Stop1: f32.Pt(0, float32(r.Min.Y)), Color1: top,
		Stop2: f32.Pt(0, float32(r.Max.Y)), Color2: bottom,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// Border strokes a w px border inside r.
func Border(gtx C, r image.Rectangle, c color.NRGBA, w, radius int) {
	h := float32(w) / 2
	rr := clip.RRect{Rect: r, NW: radius, NE: radius, SE: radius, SW: radius}
	var p clip.Path
	p.Begin(gtx.Ops)
	appendRRectF(&p, float32(r.Min.X)+h, float32(r.Min.Y)+h, float32(r.Max.X)-h, float32(r.Max.Y)-h,
		float32(rr.NW), float32(rr.NE), float32(rr.SE), float32(rr.SW))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: float32(w)}.Op())
}

// appendRRectF adds a rounded rectangle to a path.
func appendRRectF(p *clip.Path, x0, y0, x1, y1, nw, ne, se, sw float32) {
	const k = 0.55228475 // circle arc as a cubic
	p.MoveTo(f32.Pt(x0+nw, y0))
	p.LineTo(f32.Pt(x1-ne, y0))
	p.CubeTo(f32.Pt(x1-ne+ne*k, y0), f32.Pt(x1, y0+ne-ne*k), f32.Pt(x1, y0+ne))
	p.LineTo(f32.Pt(x1, y1-se))
	p.CubeTo(f32.Pt(x1, y1-se+se*k), f32.Pt(x1-se+se*k, y1), f32.Pt(x1-se, y1))
	p.LineTo(f32.Pt(x0+sw, y1))
	p.CubeTo(f32.Pt(x0+sw-sw*k, y1), f32.Pt(x0, y1-sw+sw*k), f32.Pt(x0, y1-sw))
	p.LineTo(f32.Pt(x0, y0+nw))
	p.CubeTo(f32.Pt(x0, y0+nw-nw*k), f32.Pt(x0+nw-nw*k, y0), f32.Pt(x0+nw, y0))
	p.Close()
}

// Shadow is the windows' drop shadow (box-shadow 0 2px 6px), approximated by
// a few translucent rounded layers.
func Shadow(gtx C, r image.Rectangle, radius int) {
	off := gtx.Dp(2)
	for i := 3; i >= 1; i-- {
		g := gtx.Dp(unit.Dp(i * 2))
		rr := image.Rectangle{Min: r.Min.Add(image.Pt(-g/2, off-g/2)), Max: r.Max.Add(image.Pt(g/2, off+g/2))}
		FillRound(gtx, rr, color.NRGBA{A: uint8(0x22 / i)}, radius+g, radius+g, radius+g, radius+g)
	}
}

// Box lays out w over a filled, bordered rectangle (a CSS box with padding).
type Box struct {
	Bg, Line color.NRGBA
	Radius   unit.Dp
	Pad      layout.Inset
}

func (b Box) Layout(gtx C, w layout.Widget) D {
	m := op.Record(gtx.Ops)
	dims := b.Pad.Layout(gtx, w)
	call := m.Stop()
	r := image.Rectangle{Max: dims.Size}
	rad := gtx.Dp(b.Radius)
	if b.Bg.A > 0 {
		FillRound(gtx, r, b.Bg, rad, rad, rad, rad)
	}
	call.Add(gtx.Ops)
	if b.Line.A > 0 {
		Border(gtx, r, b.Line, 1, rad)
	}
	return dims
}

// Inset shorthand in dp: top, right, bottom, left (CSS order).
func Inset(t, r, b, l unit.Dp) layout.Inset {
	return layout.Inset{Top: t, Right: r, Bottom: b, Left: l}
}

// Gap is an empty space of w×h dp.
func Gap(w, h unit.Dp) layout.Widget {
	return func(gtx C) D { return D{Size: image.Pt(gtx.Dp(w), gtx.Dp(h))} }
}

// DashedBorder draws a dashed rectangle of line width w, dashes and gaps
// dash long (the equipment window's comparison slots).
func DashedBorder(gtx C, r image.Rectangle, c color.NRGBA, w, dash int) {
	w, dash = max(w, 1), max(dash, 2)
	for x := r.Min.X; x < r.Max.X; x += 2 * dash {
		x1 := min(x+dash, r.Max.X)
		Fill(gtx, image.Rect(x, r.Min.Y, x1, r.Min.Y+w), c)
		Fill(gtx, image.Rect(x, r.Max.Y-w, x1, r.Max.Y), c)
	}
	for y := r.Min.Y; y < r.Max.Y; y += 2 * dash {
		y1 := min(y+dash, r.Max.Y)
		Fill(gtx, image.Rect(r.Min.X, y, r.Min.X+w, y1), c)
		Fill(gtx, image.Rect(r.Max.X-w, y, r.Max.X, y1), c)
	}
}
