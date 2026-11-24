package ui

import (
	"image"
	"image/color"

	xdraw "golang.org/x/image/draw"
)

// The client's collection art (75x100, the item window's picture) is drawn
// on white. cutArt takes the white around the item out and crops to it.

// artWhite is how light a background pixel is (its darkest channel): white,
// the soft glow and the pale shadow under the item.
const artWhite = 200

// cutArt returns the art without its background.
func cutArt(src image.Image) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.Draw(img, img.Rect, src, b.Min, xdraw.Src)
	low := func(x, y int) int {
		c := img.NRGBAAt(x, y)
		return int(min(c.R, c.G, c.B))
	}
	// The light area reached from the border: light parts inside the
	// item's outline stay.
	back := make([]bool, w*h)
	var todo []image.Point
	visit := func(x, y int) {
		if x < 0 || y < 0 || x >= w || y >= h || back[y*w+x] {
			return
		}
		if c := img.NRGBAAt(x, y); c.A > 0 && low(x, y) < artWhite {
			return
		}
		back[y*w+x] = true
		todo = append(todo, image.Pt(x, y))
	}
	for x := 0; x < w; x++ {
		visit(x, 0)
		visit(x, h-1)
	}
	for y := 0; y < h; y++ {
		visit(0, y)
		visit(w-1, y)
	}
	for len(todo) > 0 {
		p := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		visit(p.X-1, p.Y)
		visit(p.X+1, p.Y)
		visit(p.X, p.Y-1)
		visit(p.X, p.Y+1)
	}
	dropBits(back, w, h)
	isBack := func(x, y int) bool { return x < 0 || y < 0 || x >= w || y >= h || back[y*w+x] }
	out := image.NewNRGBA(img.Rect)
	var used image.Rectangle
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(x, y)
			switch {
			case c.A == 0:
				continue
			case back[y*w+x]:
				continue
			case isBack(x-1, y) || isBack(x+1, y) || isBack(x, y-1) || isBack(x, y+1):
				c = offWhite(c) // the outline is blended with the white
			}
			out.SetNRGBA(x, y, c)
			if c.A > 60 {
				used = used.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if used.Empty() {
		return out
	}
	return out.SubImage(used).(*image.NRGBA)
}

// dropBits adds to the background what is left of the shadow: pieces much
// smaller than the biggest one, and the flat one on the floor under it.
func dropBits(back []bool, w, h int) {
	seen := make([]bool, w*h)
	type part struct {
		px []int
		r  image.Rectangle
	}
	var pieces []part
	var big part
	for i := range back {
		if back[i] || seen[i] {
			continue
		}
		seen[i] = true
		piece := []int{i}
		var r image.Rectangle
		for k := 0; k < len(piece); k++ {
			x, y := piece[k]%w, piece[k]/w
			r = r.Union(image.Rect(x, y, x+1, y+1))
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					if j := ny*w + nx; !back[j] && !seen[j] {
						seen[j] = true
						piece = append(piece, j)
					}
				}
			}
		}
		p := part{piece, r}
		pieces = append(pieces, p)
		if len(piece) > len(big.px) {
			big = p
		}
	}
	for _, p := range pieces {
		bit := len(p.px)*25 < len(big.px)
		floor := len(p.px)*4 < len(big.px) && p.r.Dx() >= 2*p.r.Dy() && p.r.Min.Y >= big.r.Max.Y-4
		if bit || floor {
			for _, i := range p.px {
				back[i] = true
			}
		}
	}
}

// offWhite takes the white blended into an outline pixel back out, leaving
// it translucent.
func offWhite(c color.NRGBA) color.NRGBA {
	a := 255 - int(min(c.R, c.G, c.B))
	if a >= 160 { // a dark outline is all item
		return c
	}
	a = min(255, a*255/160)
	if a == 0 {
		return color.NRGBA{}
	}
	f := func(v uint8) uint8 { return uint8(max(0, min(255, 255-(255-int(v))*255/a))) }
	return color.NRGBA{R: f(c.R), G: f(c.G), B: f(c.B), A: uint8(a)}
}

// fitArt scales the art down to fit a px square (never up).
func fitArt(src *image.NRGBA, px int) image.Image {
	b := src.Bounds()
	if b.Dx() <= px && b.Dy() <= px {
		return src
	}
	w, h := px, px
	if b.Dx() > b.Dy() {
		h = max(1, b.Dy()*px/b.Dx())
	} else {
		w = max(1, b.Dx()*px/b.Dy())
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Rect, src, b, xdraw.Src, nil)
	return dst
}
