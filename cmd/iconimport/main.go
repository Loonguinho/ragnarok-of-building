// iconimport takes the item and skill icons from the RO LATAM client's
// data.grf and writes them where the web page serves them (web/icons):
// one PNG per distinct picture, named by its contents, and index.json
// from each item and skill ID to its file. Run it after cmd/import, so
// the catalog's items are the ones looked up.
//
//	go run ./cmd/iconimport                    # RO_CLIENT from .env, else C:\Gravity\Ragnarok
//	go run ./cmd/iconimport -client D:\RO -out web/icons
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ragnarok-of-building/internal/config"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/grf"
	"ragnarok-of-building/internal/icons"
)

// uiDir is the client's interface folder, where both kinds of icon live.
const uiDir = `data\texture\유저인터페이스\item\`

func main() {
	client := flag.String("client", config.ClientDir(), "RO LATAM client folder (default: RO_CLIENT in .env)")
	catalog := flag.String("catalog", "data/gen/catalog.json.gz", "catalog whose items and skills get icons")
	out := flag.String("out", "web/icons", "folder to write")
	flag.Parse()
	if err := run(filepath.Join(*client, "data.grf"), *catalog, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(grfPath, catalogPath, out string) error {
	a, err := grf.Open(grfPath)
	if err != nil {
		return err
	}
	defer a.Close()
	cat, err := gamedata.Load(catalogPath)
	if err != nil {
		return err
	}
	idx := icons.Index{Item: map[int]string{}, Skill: map[int]string{}}
	files := map[string][]byte{}
	byRes := map[string]string{} // resource -> hash ("" = none), read once
	take := func(res string) string {
		if h, ok := byRes[res]; ok {
			return h
		}
		h := ""
		if b, err := a.Read(uiDir + res + ".bmp"); err == nil {
			if p, err := icons.ToPNG(b); err == nil {
				h = icons.Hash(p)
				files[h] = p
			} else {
				fmt.Fprintf(os.Stderr, "%s: %v\n", res, err)
			}
		}
		byRes[res] = h
		return h
	}
	for id, it := range cat.Items {
		if it.Resource == "" {
			continue
		}
		if h := take(it.Resource); h != "" {
			idx.Item[id] = h
		}
	}
	for id, s := range cat.Skills {
		if s.Code == "" {
			continue
		}
		if h := take(strings.ToLower(s.Code)); h != "" {
			idx.Skill[id] = h
		}
	}
	return write(out, idx, files)
}

// write replaces the folder's icons with these: pictures no longer used go.
func write(out string, idx icons.Index, files map[string][]byte) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	old, _ := filepath.Glob(filepath.Join(out, "*.png"))
	for _, f := range old {
		if _, keep := files[strings.TrimSuffix(filepath.Base(f), ".png")]; !keep {
			if err := os.Remove(f); err != nil {
				return err
			}
		}
	}
	total := 0
	for h, b := range files {
		total += len(b)
		f := filepath.Join(out, icons.File(h))
		if cur, err := os.ReadFile(f); err == nil && string(cur) == string(b) {
			continue
		}
		if err := os.WriteFile(f, b, 0o644); err != nil {
			return err
		}
	}
	b, err := json.Marshal(idx) // map keys come out sorted: no churn between runs
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, icons.IndexFile), b, 0o644); err != nil {
		return err
	}
	fmt.Printf("items %d, skills %d -> %d pictures, %.1f MB, index %d KB in %s\n",
		len(idx.Item), len(idx.Skill), len(files), float64(total)/1e6, len(b)/1024, out)
	return nil
}
