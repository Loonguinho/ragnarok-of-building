// icon draws the app icon (a sword on an RO-window-blue tile) as a PNG.
//
//	go run ./tools/icon cmd/app/icon.png
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const size = 256
const ss = 4 // supersampling for smooth edges

type shape func(x, y float64) (color.RGBA, bool)

func lerp(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }

func roundRect(x0, y0, x1, y1, r float64) func(x, y float64) bool {
	return func(x, y float64) bool {
		if x < x0 || x > x1 || y < y0 || y > y1 {
			return false
		}
		cx := math.Max(x0+r, math.Min(x, x1-r))
		cy := math.Max(y0+r, math.Min(y, y1-r))
		return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
	}
}

func poly(pts [][2]float64) func(x, y float64) bool {
	return func(x, y float64) bool {
		in := false
		for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
			xi, yi, xj, yj := pts[i][0], pts[i][1], pts[j][0], pts[j][1]
			if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
				in = !in
			}
		}
		return in
	}
}

func main() {
	tile := roundRect(8, 8, 248, 248, 44)
	border := roundRect(8, 8, 248, 248, 44)
	inner := roundRect(16, 16, 240, 240, 38)
	// Sword pointing up, slightly tilted feel kept simple: vertical.
	blade := poly([][2]float64{{128, 30}, {146, 58}, {146, 168}, {110, 168}, {110, 58}})
	ridge := poly([][2]float64{{128, 38}, {131, 60}, {131, 164}, {125, 164}, {125, 60}})
	guard := roundRect(72, 166, 184, 186, 9)
	grip := roundRect(118, 186, 138, 226, 5)
	pommel := func(x, y float64) bool { return (x-128)*(x-128)+(y-232)*(y-232) <= 13*13 }

	layers := []shape{
		func(x, y float64) (color.RGBA, bool) { // border
			return color.RGBA{0x12, 0x2f, 0x6b, 255}, border(x, y)
		},
		func(x, y float64) (color.RGBA, bool) { // tile gradient
			t := y / size
			return color.RGBA{lerp(0x5b, 0x1d, t), lerp(0x8f, 0x4f, t), lerp(0xe0, 0xa8, t), 255}, inner(x, y)
		},
		func(x, y float64) (color.RGBA, bool) { return color.RGBA{0xf4, 0xf7, 0xfc, 255}, blade(x, y) },
		func(x, y float64) (color.RGBA, bool) { return color.RGBA{0xb9, 0xc6, 0xdc, 255}, ridge(x, y) },
		func(x, y float64) (color.RGBA, bool) { return color.RGBA{0xf2, 0xc2, 0x3a, 255}, guard(x, y) },
		func(x, y float64) (color.RGBA, bool) { return color.RGBA{0x7a, 0x4a, 0x24, 255}, grip(x, y) },
		func(x, y float64) (color.RGBA, bool) { return color.RGBA{0xf2, 0xc2, 0x3a, 255}, pommel(x, y) },
	}
	_ = tile

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := float64(px) + (float64(sx)+0.5)/ss
					y := float64(py) + (float64(sy)+0.5)/ss
					var c color.RGBA
					hit := false
					for _, l := range layers {
						if lc, ok := l(x, y); ok {
							c, hit = lc, true
						}
					}
					if hit {
						r += float64(c.R)
						g += float64(c.G)
						b += float64(c.B)
						a += 255
					}
				}
			}
			n := float64(ss * ss)
			if a > 0 {
				img.SetNRGBA(px, py, color.NRGBA{uint8(r / (a / 255)), uint8(g / (a / 255)), uint8(b / (a / 255)), uint8(a / n)})
			}
		}
	}
	f, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
