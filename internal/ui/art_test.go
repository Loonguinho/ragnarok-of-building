package ui

import (
	"image"
	"image/color"
	"testing"
)

// The white around the item goes; the white inside it stays.
func TestCutArt(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 75, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 75; x++ {
			c := color.NRGBA{0xff, 0xff, 0xff, 0xff}
			in := x >= 20 && x < 50 && y >= 30 && y < 70
			if in && (x < 23 || x >= 47 || y < 33 || y >= 67) {
				c = color.NRGBA{0x20, 0x10, 0x10, 0xff}
			}
			if x >= 60 && x < 62 && y == 90 { // a bit of shadow
				c = color.NRGBA{0x80, 0x80, 0xc0, 0xff}
			}
			src.SetNRGBA(x, y, c)
		}
	}
	got := cutArt(src)
	if got.Rect != image.Rect(20, 30, 50, 70) {
		t.Fatalf("cropped to %v", got.Rect)
	}
	if a := got.NRGBAAt(35, 50).A; a != 255 {
		t.Errorf("the white inside has alpha %d", a)
	}
	if s := fitArt(got, 36).Bounds().Size(); s != image.Pt(27, 36) {
		t.Errorf("fitted to %v", s)
	}
}
