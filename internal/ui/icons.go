package ui

import (
	"context"
	"image"
	"image/png"
	"os"
	"sync"

	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Icons loads item/skill/monster/class icons in the background (from the
// icon cache, which downloads each one once) and keeps them in memory.
type Icons struct {
	Get        func(ctx context.Context, rel string) (string, error) // the cache (web.IconCache.Get)
	Invalidate func()                                                // redraw when one arrives

	mu      sync.Mutex
	imgs    map[string]paint.ImageOp
	pending map[string]bool
	sem     chan struct{}
}

// NewIcons uses get to fetch and invalidate to ask for a redraw.
func NewIcons(get func(ctx context.Context, rel string) (string, error), invalidate func()) *Icons {
	return &Icons{Get: get, Invalidate: invalidate, imgs: map[string]paint.ImageOp{}, pending: map[string]bool{}, sem: make(chan struct{}, 6)}
}

// Img returns the icon, loading it when needed (ok false until it arrives
// or when there is none).
func (ic *Icons) Img(rel string) (paint.ImageOp, bool) {
	if ic == nil || rel == "" {
		return paint.ImageOp{}, false
	}
	ic.mu.Lock()
	defer ic.mu.Unlock()
	if op, ok := ic.imgs[rel]; ok {
		return op, op.Size().X > 0
	}
	if !ic.pending[rel] {
		ic.pending[rel] = true
		go ic.load(rel)
	}
	return paint.ImageOp{}, false
}

func (ic *Icons) load(rel string) {
	ic.sem <- struct{}{}
	defer func() { <-ic.sem }()
	var op paint.ImageOp
	if file, err := ic.Get(context.Background(), rel); err == nil {
		if f, err := os.Open(file); err == nil {
			if img, err := png.Decode(f); err == nil {
				op = paint.NewImageOp(img)
				op.Filter = paint.FilterNearest // pixel art
			}
			f.Close()
		}
	}
	ic.mu.Lock()
	ic.imgs[rel] = op
	delete(ic.pending, rel)
	ic.mu.Unlock()
	if ic.Invalidate != nil {
		ic.Invalidate()
	}
}

// Icon lays an icon in a size×size dp box (empty while loading).
func (ic *Icons) Icon(gtx C, rel string, size unit.Dp) D {
	px := gtx.Dp(size)
	sz := image.Pt(px, px)
	img, ok := ic.Img(rel)
	if !ok {
		return D{Size: sz}
	}
	g := gtx
	g.Constraints.Min, g.Constraints.Max = sz, sz
	widget.Image{Src: img, Fit: widget.ScaleDown, Scale: 1}.Layout(g)
	return D{Size: sz}
}
