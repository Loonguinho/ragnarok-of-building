// spriteimport draws the character pictures the equipment window puts
// together (internal/sprite) from the RO LATAM client's data.grf: every
// job's body and every hair style, male and female, and every headgear
// and costume headgear of the catalog, each standing and facing the
// viewer. It writes them to web/sprites as PNGs named by their contents,
// with index.json. Run it after cmd/import.
//
//	go run ./cmd/spriteimport                  # RO_CLIENT from .env
//	go run ./cmd/spriteimport -client D:\RO -out web/sprites
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/text/encoding/korean"

	"ragnarok-of-building/internal/config"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/grf"
	"ragnarok-of-building/internal/icons"
	"ragnarok-of-building/internal/lua51"
	"ragnarok-of-building/internal/sprite"
)

const (
	spriteDir = `data\sprite\`
	luaDir    = `data\luafiles514\lua files\datainfo\`
	maxHair   = 60
)

func main() {
	client := flag.String("client", config.ClientDir(), "RO LATAM client folder (default: RO_CLIENT in .env)")
	catalog := flag.String("catalog", "data/gen/catalog.json.gz", "catalog whose jobs and headgears are drawn")
	out := flag.String("out", "web/sprites", "folder to write")
	flag.Parse()
	if err := run(filepath.Join(*client, "data.grf"), *catalog, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type importer struct {
	a     *grf.Archive
	files map[string][]byte // hash -> PNG
	miss  []string
}

func run(grfPath, catalogPath, out string) error {
	a, err := grf.Open(grfPath)
	if err != nil {
		return err
	}
	defer a.Close()
	c, err := gamedata.Load(catalogPath)
	if err != nil {
		return err
	}
	im := &importer{a: a, files: map[string][]byte{}}
	idx := sprite.Index{Body: map[string]sprite.Entry{}, Head: map[string]sprite.Entry{}, Acc: map[string]sprite.Entry{}}
	sexes := []bool{false, true}

	for id, j := range c.Jobs {
		name, ok := jobSprites[j.Code]
		if !ok {
			im.miss = append(im.miss, "job "+j.Code)
			continue
		}
		race := `인간족\`
		if slices.Contains(doramJobs, j.Code) {
			race = `도람족\`
			idx.Doram = append(idx.Doram, id)
		}
		for _, f := range sexes {
			if e, ok := im.draw(spriteDir + race + `몸통\` + sex(f) + `\` + name + "_" + sex(f)); ok {
				idx.Body[sprite.Key(id, f)] = e
			}
		}
	}
	slices.Sort(idx.Doram)

	for _, race := range []struct{ dir, prefix string }{{`인간족\`, ""}, {`도람족\`, "d"}} {
		for _, f := range sexes {
			for n := 1; n <= maxHair; n++ {
				base := spriteDir + race.dir + `머리통\` + sex(f) + `\` + fmt.Sprint(n) + "_" + sex(f)
				if _, ok := a.Lookup(base + ".spr"); !ok {
					continue
				}
				if e, ok := im.draw(base); ok {
					idx.Head[race.prefix+sprite.Key(n, f)] = e
				}
			}
		}
	}

	accNames, err := im.accessoryNames()
	if err != nil {
		return err
	}
	views := map[int]bool{}
	for _, it := range c.Items {
		if it.ClassNum > 0 && slices.ContainsFunc(it.Locations, func(l string) bool { return strings.Contains(l, "Head_") }) {
			views[it.ClassNum] = true
		}
	}
	for v := range views {
		name, ok := accNames[v]
		if !ok || name == "" {
			continue
		}
		for _, f := range sexes {
			if e, ok := im.draw(spriteDir + `악세사리\` + sex(f) + `\` + sex(f) + name); ok {
				idx.Acc[sprite.Key(v, f)] = e
			}
		}
	}

	if err := im.write(out, idx); err != nil {
		return err
	}
	fmt.Printf("bodies %d, heads %d, headgears %d (of %d views) -> %d pictures in %s\n",
		len(idx.Body), len(idx.Head), len(idx.Acc), len(views), len(im.files), out)
	if len(im.miss) > 0 {
		slices.Sort(im.miss)
		fmt.Printf("not drawn (%d): %s\n", len(im.miss), strings.Join(im.miss, ", "))
	}
	return nil
}

func sex(female bool) string {
	if female {
		return "여"
	}
	return "남"
}

// draw renders the standing, front-facing frame of a sprite.
func (im *importer) draw(base string) (sprite.Entry, bool) {
	sb, err1 := im.a.Read(base + ".spr")
	ab, err2 := im.a.Read(base + ".act")
	if err := errors.Join(err1, err2); err != nil {
		im.miss = append(im.miss, strings.TrimPrefix(base, spriteDir))
		return sprite.Entry{}, false
	}
	s, err := sprite.DecodeSpr(sb)
	if err != nil {
		im.miss = append(im.miss, base+": "+err.Error())
		return sprite.Entry{}, false
	}
	act, err := sprite.DecodeAct(ab)
	if err != nil {
		im.miss = append(im.miss, base+": "+err.Error())
		return sprite.Entry{}, false
	}
	p, ok := sprite.Render(s, act, 0, 0)
	if !ok {
		return sprite.Entry{}, false
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, p.Img); err != nil {
		return sprite.Entry{}, false
	}
	h := icons.Hash(buf.Bytes())
	im.files[h] = buf.Bytes()
	return sprite.Entry{File: h, OX: p.Origin.X, OY: p.Origin.Y, AX: p.Anchor.X, AY: p.Anchor.Y, HasAnchor: p.HasAnchor}, true
}

// accessoryNames reads AccNameTable: view ID -> sprite name ("_고글").
func (im *importer) accessoryNames() (map[int]string, error) {
	env := map[string]lua51.Value{}
	for _, f := range []string{"accessoryid", "accname"} {
		b, err := im.a.Read(luaDir + f + ".lub")
		if err != nil {
			return nil, err
		}
		if _, err := lua51.LoadEnv(b, env); err != nil {
			var pe *lua51.PartialError
			if !errors.As(err, &pe) {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
		}
	}
	t, ok := env["AccNameTable"].(*lua51.Table)
	if !ok {
		return nil, errors.New("accname.lub: no AccNameTable")
	}
	dec := korean.EUCKR.NewDecoder()
	out := map[int]string{}
	for _, k := range t.IntKeys() {
		raw, _ := t.Get(float64(k)).(string)
		if s, err := dec.String(raw); err == nil {
			out[k] = s
		}
	}
	return out, nil
}

// write replaces the folder's pictures with these and writes the index.
func (im *importer) write(out string, idx sprite.Index) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	old, _ := filepath.Glob(filepath.Join(out, "*.png"))
	for _, f := range old {
		if _, keep := im.files[strings.TrimSuffix(filepath.Base(f), ".png")]; !keep {
			if err := os.Remove(f); err != nil {
				return err
			}
		}
	}
	total := 0
	for h, b := range im.files {
		total += len(b)
		f := filepath.Join(out, h+".png")
		if cur, err := os.ReadFile(f); err == nil && bytes.Equal(cur, b) {
			continue
		}
		if err := os.WriteFile(f, b, 0o644); err != nil {
			return err
		}
	}
	b, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	fmt.Printf("%.1f MB of pictures, index %d KB\n", float64(total)/1e6, len(b)/1024)
	return os.WriteFile(filepath.Join(out, sprite.IndexFile), b, 0o644)
}
