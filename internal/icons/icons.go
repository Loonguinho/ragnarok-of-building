// Package icons holds the item and skill icons taken from the game
// client's data.grf (cmd/iconimport) and serves them to the window.
//
// The icons ship with the web page as PNG files named by their contents
// (many items share a picture: every card has the same one), plus an
// index from each item and skill ID to its file.
package icons

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"regexp"
	"strconv"
	"sync"

	"golang.org/x/image/bmp"
)

// IndexFile is the index's name in the icons folder.
const IndexFile = "index.json"

// Index maps item and skill IDs to icon files (File names them).
type Index struct {
	Item  map[int]string `json:"item"`
	Skill map[int]string `json:"skill"`
}

// File is the name of the icon whose contents hash to h.
func File(h string) string { return h + ".png" }

// Hash names an icon by its contents (short: a few thousand files).
func Hash(png []byte) string {
	s := sha256.Sum256(png)
	return hex.EncodeToString(s[:6])
}

// ToPNG turns a client BMP into a PNG: the client's magenta key becomes
// transparent. Icons have at most 256 colours, so they come out paletted
// (smaller); a picture with more keeps every colour.
func ToPNG(b []byte) ([]byte, error) {
	img, err := bmp.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	r := img.Bounds()
	full := image.NewNRGBA(r)
	idx := map[color.NRGBA]uint8{}
	var pal color.Palette
	paletted := image.NewPaletted(r, nil)
	fits := true
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if isKey(c) {
				c = color.NRGBA{}
			}
			full.SetNRGBA(x, y, c)
			if !fits {
				continue
			}
			i, ok := idx[c]
			if !ok {
				if len(pal) == 256 {
					fits = false
					continue
				}
				i = uint8(len(pal))
				idx[c] = i
				pal = append(pal, c)
			}
			paletted.SetColorIndex(x, y, i)
		}
	}
	var out image.Image = full
	if fits {
		paletted.Palette = pal
		out = paletted
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// isKey is the client's transparent colour: magenta, give or take what
// the artists' tools left near it.
func isKey(c color.NRGBA) bool { return c.R >= 0xF8 && c.G <= 0x08 && c.B >= 0xF8 }

// iconPath: the paths the window asks for (view.ItemIcon, view.SkillIcon).
var iconPath = regexp.MustCompile(`^images/(items/item|skill)/([0-9]{1,9})\.png$`)

// Served gives the window the client icons: get reads a file of the icons
// folder (the index, then the icons). The index is read once, the first
// time an icon is asked for.
type Served struct {
	get func(ctx context.Context, name string) ([]byte, error)

	once  sync.Once
	index Index
	err   error
}

// NewServed reads the icons folder through get.
func NewServed(get func(ctx context.Context, name string) ([]byte, error)) *Served {
	return &Served{get: get}
}

// Get returns the PNG for an icon path (view.ItemIcon, view.SkillIcon);
// os.ErrNotExist for pictures the client has none of, and for the kinds
// not taken from it yet (item art, monsters, classes).
func (s *Served) Get(ctx context.Context, rel string) ([]byte, error) {
	m := iconPath.FindStringSubmatch(rel)
	if m == nil {
		return nil, os.ErrNotExist
	}
	s.once.Do(func() { s.err = s.load(ctx) })
	if s.err != nil {
		return nil, s.err
	}
	id, _ := strconv.Atoi(m[2])
	kind := s.index.Item
	if m[1] == "skill" {
		kind = s.index.Skill
	}
	h, ok := kind[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return s.get(ctx, File(h))
}

func (s *Served) load(ctx context.Context) error {
	b, err := s.get(ctx, IndexFile)
	if err != nil {
		return fmt.Errorf("icons: %w", err)
	}
	if err := json.Unmarshal(b, &s.index); err != nil {
		return fmt.Errorf("icons: %s: %w", IndexFile, err)
	}
	return nil
}
