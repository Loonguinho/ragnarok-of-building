package ui

import (
	"image/color"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Txt is a text run: Label(...).Layout or the shorthand T(th, s).
type Txt struct {
	th       *Theme
	S        string
	Color    color.NRGBA
	Size     unit.Sp
	Font     font.Font
	MaxLines int
	Align    text.Alignment
	Wrap     bool
}

// T is text in the interface font and color.
func T(th *Theme, s string) Txt {
	return Txt{th: th, S: s, Color: ColText, Size: TextSize, Font: th.Font}
}

// B is bold text.
func B(th *Theme, s string) Txt { t := T(th, s); t.Font = th.Bold; return t }

func (t Txt) C(c color.NRGBA) Txt { t.Color = c; return t }
func (t Txt) Sz(s unit.Sp) Txt    { t.Size = s; return t }
func (t Txt) Lines(n int) Txt     { t.MaxLines = n; return t }
func (t Txt) Bold() Txt           { t.Font = t.th.Bold; return t }
func (t Txt) Center() Txt         { t.Align = text.Middle; return t }
func (t Txt) Right() Txt          { t.Align = text.End; return t }
func (t Txt) Wrapping() Txt       { t.Wrap = true; return t }
func (t Txt) F(f font.Font) Txt   { t.Font = f; return t }
func (t Txt) Layout(gtx C) D      { return t.layout(gtx) }
func (t Txt) W() layout.Widget    { return t.layout }
func colorMat(gtx C, c color.NRGBA) op.CallOp {
	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	return m.Stop()
}

func (t Txt) layout(gtx C) D {
	l := widget.Label{MaxLines: t.MaxLines, Alignment: t.Align}
	if t.MaxLines == 0 && !t.Wrap {
		l.MaxLines = 1
	}
	minX := gtx.Constraints.Min.X
	if t.Align == text.Start {
		gtx.Constraints.Min.X = 0
	}
	d := l.Layout(gtx, t.th.Shaper, t.Font, t.Size, t.S, colorMat(gtx, t.Color))
	d.Size.X = max(d.Size.X, minX)
	return d
}

// Span is a piece of colored text (client descriptions use ^RRGGBB codes).
type Span struct {
	S     string
	Color color.NRGBA
}

// ColorLine splits one description line at its ^RRGGBB codes; ^000000
// goes back to the default color.
func ColorLine(line string, def color.NRGBA) []Span {
	var out []Span
	cur := def
	for {
		i := strings.IndexByte(line, '^')
		if i < 0 || i+7 > len(line) {
			break
		}
		v, err := strconv.ParseUint(line[i+1:i+7], 16, 32)
		if err != nil {
			if i > 0 || line[:i+1] != "" {
				out = append(out, Span{line[:i+1], cur})
			}
			line = line[i+1:]
			continue
		}
		if i > 0 {
			out = append(out, Span{line[:i], cur})
		}
		cur = rgb(uint32(v))
		if v == 0 {
			cur = def
		}
		line = line[i+7:]
	}
	if line != "" {
		out = append(out, Span{line, cur})
	}
	return out
}

// RichText lays out description lines with their colors, wrapped to the
// width available.
func RichText(th *Theme, lines []string, def color.NRGBA) layout.Widget {
	return func(gtx C) D {
		var kids []layout.FlexChild
		for _, ln := range lines {
			spans := ColorLine(ln, def)
			if len(spans) == 0 {
				kids = append(kids, layout.Rigid(Gap(1, 13)))
				continue
			}
			kids = append(kids, layout.Rigid(func(gtx C) D { return flowSpans(gtx, th, spans) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	}
}

// flowSpans lays colored spans word by word, wrapping like inline text.
func flowSpans(gtx C, th *Theme, spans []Span) D {
	type word struct {
		s string
		c color.NRGBA
	}
	var words []word
	for _, sp := range spans {
		parts := strings.SplitAfter(sp.S, " ")
		for _, p := range parts {
			if p != "" {
				words = append(words, word{p, sp.Color})
			}
		}
	}
	maxW := gtx.Constraints.Max.X
	x, y, lineH, w := 0, 0, 0, 0
	for _, wd := range words {
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints.Min.X = 0
		d := T(th, wd.s).C(wd.c).Layout(g)
		call := m.Stop()
		if x > 0 && x+d.Size.X > maxW {
			x, y = 0, y+lineH
			lineH = 0
		}
		st := op.Offset(imagePt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		st.Pop()
		x += d.Size.X
		w = max(w, x)
		lineH = max(lineH, d.Size.Y)
	}
	return D{Size: imagePt(max(w, gtx.Constraints.Min.X), y+lineH)}
}
