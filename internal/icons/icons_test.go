package icons

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"golang.org/x/image/bmp"
)

func bmpOf(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestToPNGKeysMagentaAndKeepsColours(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	src.Set(0, 0, color.RGBA{0xFF, 0x00, 0xFF, 0xFF}) // the key
	src.Set(1, 0, color.RGBA{0xFC, 0x04, 0xFA, 0xFF}) // near the key
	src.Set(2, 0, color.RGBA{0x10, 0x80, 0x20, 0xFF})
	p, err := ToPNG(bmpOf(t, src))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(p))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := img.(*image.Paletted); !ok {
		t.Errorf("3 colours: got %T, want a paletted PNG", img)
	}
	for x, want := range []color.NRGBA{{}, {}, {0x10, 0x80, 0x20, 0xFF}} {
		if got := color.NRGBAModel.Convert(img.At(x, 0)); got != want {
			t.Errorf("pixel %d = %v, want %v", x, got, want)
		}
	}
}

func TestToPNGManyColours(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 20, 20)) // 400 colours
	for i := range 400 {
		src.Set(i%20, i/20, color.RGBA{uint8(i), uint8(i >> 8), 7, 0xFF})
	}
	p, err := ToPNG(bmpOf(t, src))
	if err != nil {
		t.Fatal(err)
	}
	img, _ := png.Decode(bytes.NewReader(p))
	if got := color.NRGBAModel.Convert(img.At(19, 19)); got != (color.NRGBA{143, 1, 7, 0xFF}) {
		t.Errorf("last pixel = %v: colours were lost", got)
	}
}

func TestServed(t *testing.T) {
	files := map[string]string{
		IndexFile:     `{"item":{"501":"aa"},"skill":{"5":"bb"}}`,
		File("aa"):    "potion",
		File("bb"):    "bash",
		"unused.json": "",
	}
	reads := map[string]int{}
	s := NewServed(func(_ context.Context, name string) ([]byte, error) {
		reads[name]++
		if b, ok := files[name]; ok {
			return []byte(b), nil
		}
		return nil, os.ErrNotExist
	})
	ctx := context.Background()
	for rel, want := range map[string]string{"images/items/item/501.png": "potion", "images/skill/5.png": "bash"} {
		if b, err := s.Get(ctx, rel); err != nil || string(b) != want {
			t.Errorf("%s = %q, %v; want %q", rel, b, err, want)
		}
	}
	for _, rel := range []string{"images/items/item/502.png", "images/skill/501.png", "images/items/collection/501.png", "images/mobs/png/1002.png", "../index.json"} {
		if _, err := s.Get(ctx, rel); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: err %v, want not found", rel, err)
		}
	}
	if reads[IndexFile] != 1 {
		t.Errorf("index read %d times, want once", reads[IndexFile])
	}
}
