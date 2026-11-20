// Package ui is the native interface of Ragnarok of Building, drawn with
// Gio: RO-client-style windows (the look of the retired web version's
// ro-ui.css), without a browser engine. It only uses Gio's portable
// packages; the window itself (gioui.org/app) is opened by cmd/app.
package ui

import (
	"image/color"
	"os"
	"path/filepath"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/unit"
)

// Colors of ro-ui.css (:root).
var (
	ColText       = rgb(0x2b2b2b)
	ColTitleA     = rgb(0xf4f8ff)
	ColTitleB     = rgb(0xa9bfe3)
	ColTitleText  = rgb(0x1d2b4a)
	ColBorder     = rgb(0x6f7f9e)
	ColBody       = rgb(0xf2f2ef)
	ColBodyAlt    = rgb(0xe4e6ea)
	ColBtnA       = rgb(0xfdfdfd)
	ColBtnB       = rgb(0xd3d9e4)
	ColSelect     = rgb(0xb9d2ff)
	ColHover      = rgb(0xe8f0ff)
	ColHP         = rgb(0x3fbf3f)
	ColSP         = rgb(0x3f6fdf)
	ColEXP        = rgb(0xe0b030)
	ColAP         = rgb(0xe0702f)
	ColSystem     = rgb(0xffe36b)
	ColUp         = rgb(0x1d7a2a)
	ColDown       = rgb(0xb42318)
	ColGrade      = rgb(0xb5179e)
	ColOn         = rgb(0x6aa84f)
	ColLink       = rgb(0x1d4fa8)
	ColMuted      = rgb(0x6b6b6b)
	ColLabel      = rgb(0x4a4a6a)
	ColListBorder = rgb(0xb7bcc6)
	ColSep        = rgb(0xc4c8d0)
	ColWhite      = rgb(0xffffff)
	ColTipBG      = rgb(0xfffff4)
	ColShadow     = color.NRGBA{A: 0x50}
	ColChatBG     = color.NRGBA{A: 0x8c}
	ColSkyA       = rgb(0x8fb7d9)
	ColSkyB       = rgb(0xa7c98a)
	ColSkyC       = rgb(0x6f9a55)
)

func rgb(c uint32) color.NRGBA {
	return color.NRGBA{R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c), A: 0xff}
}

// Sizes (CSS px = Gio dp).
const (
	TextSize  = unit.Sp(11)
	SmallText = unit.Sp(10)
	TinyText  = unit.Sp(9)
	TitleH    = unit.Dp(18)
	BtnH      = unit.Dp(18)
	InputH    = unit.Dp(20)
)

// Theme holds the text shaper and the fonts.
type Theme struct {
	Shaper *text.Shaper
	Font   font.Font
	Bold   font.Font
	Mono   font.Font
}

// NewTheme uses Tahoma, the RO client's interface font, when Windows has it
// (as the web pages do), else Gio's Go fonts.
func NewTheme() *Theme {
	var faces []font.FontFace
	dir := filepath.Join(os.Getenv("WINDIR"), "Fonts")
	for _, f := range []struct {
		file   string
		weight font.Weight
	}{{"tahoma.ttf", font.Normal}, {"tahomabd.ttf", font.Bold}} {
		b, err := os.ReadFile(filepath.Join(dir, f.file))
		if err != nil {
			continue
		}
		face, err := opentype.Parse(b)
		if err != nil {
			continue
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: "Tahoma", Weight: f.weight}, Face: face})
	}
	faces = append(faces, gofont.Collection()...)
	th := &Theme{Shaper: text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces))}
	th.Font = font.Font{Typeface: "Tahoma"}
	if len(faces) == len(gofont.Collection()) {
		th.Font = font.Font{Typeface: "Go"}
	}
	th.Bold = th.Font
	th.Bold.Weight = font.Bold
	th.Mono = font.Font{Typeface: "Go Mono"}
	return th
}
