package ui

import (
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/golang/freetype/truetype"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Hinted draws text the way Windows and the RO client do: each glyph
// rasterized with the font's own TrueType hinting (golang/freetype), on
// whole pixels. Gio's text is unhinted, which at 11px on a 96 DPI screen
// looks blurry and washed out. Rendered lines are cached as images.
type Hinted struct {
	mu       sync.Mutex
	fonts    map[font.Font]fontPair
	fallback fontPair
	faces    map[faceKey]*hface
	cur, old map[lineKey]*lineImg
}

// fontPair is a font and the one used for the runes it lacks.
type fontPair struct{ main, alt *truetype.Font }

type faceKey struct {
	f  font.Font
	px int // size in 1/4 px
}

type hface struct {
	main, alt fontFace
	asc, desc int // whole pixels above and below the baseline
	lineH     int // baseline to baseline
	// Measuring runs the font's hinting program for every glyph, which is
	// slow: advances and whole-line widths are kept (used under Hinted.mu).
	adv    map[rune]fixed.Int26_6
	widths map[string]int
}

// widthCacheMax is when a face's line widths are dropped and measured again.
const widthCacheMax = 20000

type fontFace struct {
	f    *truetype.Font
	face xfont.Face
}

type lineKey struct {
	face faceKey
	s    string
	c    color.NRGBA
}

type lineImg struct {
	op paint.ImageOp
	w  int
}

// cacheMax is when the line cache starts a new generation: lines unused
// for a whole generation are dropped.
const cacheMax = 3000

func newHinted(fallback fontPair) *Hinted {
	return &Hinted{
		fonts:    map[font.Font]fontPair{},
		fallback: fallback,
		faces:    map[faceKey]*hface{},
		cur:      map[lineKey]*lineImg{},
		old:      map[lineKey]*lineImg{},
	}
}

// Add registers a font for a font.Font (typeface and weight).
func (h *Hinted) Add(f font.Font, p fontPair) { h.fonts[f] = p }

func (h *Hinted) face(k faceKey) *hface {
	if hf := h.faces[k]; hf != nil {
		return hf
	}
	// The font itself, its typeface in that weight, in normal weight, or
	// the fallback.
	p, ok := h.fonts[k.f]
	if !ok {
		p, ok = h.fonts[font.Font{Typeface: k.f.Typeface, Weight: k.f.Weight}]
	}
	if !ok {
		p, ok = h.fonts[font.Font{Typeface: k.f.Typeface}]
	}
	if !ok {
		p = h.fallback
	}
	opts := &truetype.Options{Size: float64(k.px) / 4, DPI: 72, Hinting: xfont.HintingFull}
	hf := &hface{
		main:   fontFace{p.main, truetype.NewFace(p.main, opts)},
		alt:    fontFace{p.alt, truetype.NewFace(p.alt, opts)},
		adv:    map[rune]fixed.Int26_6{},
		widths: map[string]int{},
	}
	m := hf.main.face.Metrics()
	hf.asc, hf.desc = m.Ascent.Ceil(), m.Descent.Ceil()
	hf.lineH = int(math.Round(float64(m.Ascent+m.Descent) / 64 * 1.2))
	h.faces[k] = hf
	return hf
}

// pick is the face that has r.
func (hf *hface) pick(r rune) fontFace {
	if hf.main.f.Index(r) == 0 && hf.alt.f.Index(r) != 0 {
		return hf.alt
	}
	return hf.main
}

// width is s's advance in whole pixels.
func (hf *hface) width(s string) int {
	if w, ok := hf.widths[s]; ok {
		return w
	}
	if len(hf.widths) >= widthCacheMax {
		hf.widths = map[string]int{}
	}
	w := hf.measure(s)
	hf.widths[s] = w
	return w
}

// advance is r's advance in ff, cached per face (the main and fallback
// fonts never share a rune: pick sends each rune to one of them).
func (hf *hface) advance(ff fontFace, r rune) fixed.Int26_6 {
	if a, ok := hf.adv[r]; ok {
		return a
	}
	a, _ := ff.face.GlyphAdvance(r)
	hf.adv[r] = a
	return a
}

func (hf *hface) measure(s string) int {
	var w fixed.Int26_6
	prev, prevFace := rune(-1), (*truetype.Font)(nil)
	for _, r := range s {
		ff := hf.pick(r)
		if prev >= 0 && prevFace == ff.f {
			w += ff.face.Kern(prev, r)
		}
		w += hf.advance(ff, r)
		prev, prevFace = r, ff.f
	}
	return w.Ceil()
}

// render draws one line in c.
func (hf *hface) render(s string, c color.NRGBA) *lineImg {
	w := hf.width(s)
	li := &lineImg{w: w}
	if w == 0 {
		return li
	}
	hgt := hf.asc + hf.desc
	img := image.NewNRGBA(image.Rect(0, 0, w, hgt))
	dot := fixed.P(0, hf.asc)
	prev, prevFace := rune(-1), (*truetype.Font)(nil)
	for _, r := range s {
		ff := hf.pick(r)
		if prev >= 0 && prevFace == ff.f {
			dot.X += ff.face.Kern(prev, r)
		}
		dr, mask, mp, adv, ok := ff.face.Glyph(dot, r)
		if ok {
			a := mask.(*image.Alpha)
			for y := dr.Min.Y; y < dr.Max.Y; y++ {
				for x := dr.Min.X; x < dr.Max.X; x++ {
					if x < 0 || y < 0 || x >= w || y >= hgt {
						continue
					}
					v := a.AlphaAt(mp.X+x-dr.Min.X, mp.Y+y-dr.Min.Y).A
					if v == 0 {
						continue
					}
					i := img.PixOffset(x, y)
					old := img.Pix[i+3]
					na := max(old, gammaA[v])
					img.Pix[i], img.Pix[i+1], img.Pix[i+2] = c.R, c.G, c.B
					img.Pix[i+3] = uint8(uint32(na) * uint32(c.A) / 255)
				}
			}
		}
		dot.X += adv
		prev, prevFace = r, ff.f
	}
	li.op = paint.NewImageOp(img)
	li.op.Filter = paint.FilterNearest
	return li
}

// gammaA lifts the glyph coverage: Gio blends in linear light, which
// makes thin dark text look lighter than the same coverage blended the
// way GDI does (in sRGB). 1-(1-a)^γ gives dark-on-light text its weight
// back.
var gammaA = func() (t [256]uint8) {
	const g = 1.8
	for i := range t {
		a := float64(i) / 255
		t[i] = uint8(math.Round((1 - math.Pow(1-a, g)) * 255))
	}
	return
}()

func (h *Hinted) line(fk faceKey, hf *hface, s string, c color.NRGBA) *lineImg {
	k := lineKey{fk, s, c}
	if li := h.cur[k]; li != nil {
		return li
	}
	li := h.old[k]
	if li == nil {
		li = hf.render(s, c)
	}
	if len(h.cur) >= cacheMax {
		h.old, h.cur = h.cur, map[lineKey]*lineImg{}
	}
	h.cur[k] = li
	return li
}

// Layout draws s like a widget.Label: maxLines 0 wraps without limit, 1
// is a single line; lines past the limit or the width end in "…".
func (h *Hinted) Layout(gtx C, f font.Font, size unit.Sp, s string, c color.NRGBA, maxLines int, wrap bool, align text.Alignment) D {
	h.mu.Lock()
	defer h.mu.Unlock()
	pps := gtx.Metric.PxPerSp
	if pps == 0 {
		pps = 1
	}
	fk := faceKey{f, int(math.Round(float64(float32(size)*pps) * 4))}
	hf := h.face(fk)
	maxW := gtx.Constraints.Max.X
	lines := h.lines(hf, s, maxW, maxLines, wrap)
	lw := 0
	imgs := make([]*lineImg, len(lines))
	for i, l := range lines {
		imgs[i] = h.line(fk, hf, l, c)
		lw = max(lw, imgs[i].w)
	}
	boxW := min(max(lw, gtx.Constraints.Min.X), maxW)
	for i, li := range imgs {
		if li.w == 0 {
			continue
		}
		x := 0
		switch align {
		case text.Middle:
			x = (boxW - li.w) / 2
		case text.End:
			x = boxW - li.w
		}
		st := op.Offset(image.Pt(x, i*hf.lineH)).Push(gtx.Ops)
		li.op.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		st.Pop()
	}
	hgt := hf.asc + (len(lines)-1)*hf.lineH + hf.desc
	hgt = max(hgt, gtx.Constraints.Min.Y)
	return D{Size: image.Pt(boxW, hgt), Baseline: hgt - hf.asc}
}

// lines breaks s into the lines to draw.
func (h *Hinted) lines(hf *hface, s string, maxW, maxLines int, wrap bool) []string {
	s = strings.TrimSuffix(s, "\n")
	s = strings.ReplaceAll(s, "\t", " ")
	single := maxLines == 0 && !wrap
	if single {
		maxLines = 1
	}
	var out []string
	cut := false
	for _, para := range strings.Split(s, "\n") {
		if single {
			out = append(out, para)
			continue
		}
		out = append(out, wrapLine(hf, para, maxW)...)
	}
	if maxLines > 0 && len(out) > maxLines {
		out, cut = out[:maxLines], true
	}
	last := len(out) - 1
	if cut || hf.width(out[last]) > maxW {
		out[last] = ellipsize(hf, out[last], maxW, cut)
	}
	return out
}

// wrapLine breaks a paragraph at spaces to fit maxW (a word too long
// alone is broken anywhere).
func wrapLine(hf *hface, para string, maxW int) []string {
	if hf.width(para) <= maxW {
		return []string{para}
	}
	var out []string
	cur := ""
	for _, word := range strings.SplitAfter(para, " ") {
		if word == "" {
			continue
		}
		try := cur + word
		if hf.width(strings.TrimRight(try, " ")) <= maxW {
			cur = try
			continue
		}
		if cur != "" {
			out = append(out, strings.TrimRight(cur, " "))
			cur = ""
		}
		// The word alone.
		for hf.width(strings.TrimRight(word, " ")) > maxW {
			rs := []rune(word)
			n := 1
			for n < len(rs) && hf.width(string(rs[:n+1])) <= maxW {
				n++
			}
			out = append(out, string(rs[:n]))
			word = string(rs[n:])
		}
		cur = word
	}
	if cur != "" || len(out) == 0 {
		out = append(out, strings.TrimRight(cur, " "))
	}
	return out
}

// ellipsize shortens s to fit maxW with "…" at the end (always, when
// lines were cut after it).
func ellipsize(hf *hface, s string, maxW int, force bool) string {
	if !force && hf.width(s) <= maxW {
		return s
	}
	rs := []rune(s)
	for n := len(rs); n > 0; n-- {
		t := strings.TrimRight(string(rs[:n]), " ") + "…"
		if hf.width(t) <= maxW {
			return t
		}
	}
	return "…"
}
