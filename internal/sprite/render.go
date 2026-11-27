package sprite

import (
	"image"
	"image/color"
	"math"
)

// Picture is one frame drawn: the image, where the frame's origin falls in
// it, and the frame's anchor (from the origin) when it has one.
type Picture struct {
	Img       *image.NRGBA
	Origin    image.Point
	Anchor    image.Point
	HasAnchor bool
}

// Render draws frame f of action a. Rotation is ignored (no character
// body, head or headgear picture at rest uses it).
func Render(s *Spr, act *Act, a, f int) (Picture, bool) {
	if a < 0 || a >= len(act.Actions) || len(act.Actions[a]) == 0 {
		return Picture{}, false
	}
	fr := act.Actions[a][min(f, len(act.Actions[a])-1)]
	type placed struct {
		src    image.Image
		at     image.Rectangle // in frame coordinates
		mirror bool
		tint   [4]uint8
	}
	var parts []placed
	var bounds image.Rectangle
	for _, l := range fr.Layers {
		var src image.Image
		switch {
		case l.Index < 0:
			continue
		case l.RGBA && l.Index < len(s.RGBA):
			src = s.RGBA[l.Index]
		case !l.RGBA && l.Index < len(s.Pal):
			src = s.Pal[l.Index]
		default:
			continue
		}
		sz := src.Bounds().Size()
		w := int(math.Round(float64(sz.X) * float64(abs32(l.ScaleX))))
		h := int(math.Round(float64(sz.Y) * float64(abs32(l.ScaleY))))
		if w <= 0 || h <= 0 {
			continue
		}
		x0 := l.X - (w+1)/2
		y0 := l.Y - (h+1)/2
		r := image.Rect(x0, y0, x0+w, y0+h)
		parts = append(parts, placed{src, r, l.Mirror, l.Color})
		bounds = bounds.Union(r)
	}
	if bounds.Empty() {
		return Picture{}, false
	}
	img := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for _, p := range parts {
		sb := p.src.Bounds()
		w, h := p.at.Dx(), p.at.Dy()
		for y := range h {
			sy := sb.Min.Y + y*sb.Dy()/h
			for x := range w {
				sx := x * sb.Dx() / w
				if p.mirror {
					sx = sb.Dx() - 1 - sx
				}
				c := color.NRGBAModel.Convert(p.src.At(sb.Min.X+sx, sy)).(color.NRGBA)
				if c.A == 0 {
					continue
				}
				c.R = uint8(int(c.R) * int(p.tint[0]) / 255)
				c.G = uint8(int(c.G) * int(p.tint[1]) / 255)
				c.B = uint8(int(c.B) * int(p.tint[2]) / 255)
				c.A = uint8(int(c.A) * int(p.tint[3]) / 255)
				over(img, p.at.Min.X-bounds.Min.X+x, p.at.Min.Y-bounds.Min.Y+y, c)
			}
		}
	}
	pic := Picture{Img: img, Origin: image.Pt(-bounds.Min.X, -bounds.Min.Y)}
	if len(fr.Anchors) > 0 {
		pic.Anchor = image.Pt(fr.Anchors[0][0], fr.Anchors[0][1])
		pic.HasAnchor = true
	}
	return pic, true
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

// over draws c over the pixel at x, y.
func over(img *image.NRGBA, x, y int, c color.NRGBA) {
	if c.A == 255 {
		img.SetNRGBA(x, y, c)
		return
	}
	d := img.NRGBAAt(x, y)
	a := int(c.A) + int(d.A)*(255-int(c.A))/255
	if a == 0 {
		return
	}
	mix := func(s, t uint8) uint8 {
		return uint8((int(s)*int(c.A) + int(t)*int(d.A)*(255-int(c.A))/255) / a)
	}
	img.SetNRGBA(x, y, color.NRGBA{mix(c.R, d.R), mix(c.G, d.G), mix(c.B, d.B), uint8(a)})
}

// Compose draws the parts in order (the first is the body) into one
// image. Each later part's anchor goes on the body's anchor; a part
// without one keeps its origin on the body's.
func Compose(parts ...Picture) *image.NRGBA {
	if len(parts) == 0 {
		return nil
	}
	body := parts[0]
	type at struct {
		pic Picture
		min image.Point // top-left, in the body's frame coordinates
	}
	var placed []at
	var bounds image.Rectangle
	for i, p := range parts {
		if p.Img == nil {
			continue
		}
		shift := image.Point{}
		if i > 0 && body.HasAnchor && p.HasAnchor {
			shift = body.Anchor.Sub(p.Anchor)
		}
		min := shift.Sub(p.Origin)
		placed = append(placed, at{p, min})
		bounds = bounds.Union(image.Rectangle{Min: min, Max: min.Add(p.Img.Rect.Size())})
	}
	out := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for _, p := range placed {
		o := p.min.Sub(bounds.Min)
		r := p.pic.Img.Rect
		for y := range r.Dy() {
			for x := range r.Dx() {
				if c := p.pic.Img.NRGBAAt(x, y); c.A > 0 {
					over(out, o.X+x, o.Y+y, c)
				}
			}
		}
	}
	return out
}
