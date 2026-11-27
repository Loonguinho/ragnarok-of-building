// websplit writes the web page's game data (make web): the catalog without
// the item descriptions, which open the window sooner, and the
// descriptions apart, one file per language, which the page loads once
// the window is up (see gamedata.TakeItemDescs).
//
//	go run ./tools/websplit -in data/gen/catalog.json.gz -out dist/web
//
// It writes catalog.json.gz and desc-<lang>.json.gz.
package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ragnarok-of-building/internal/gamedata"
)

func main() {
	in := flag.String("in", "data/gen/catalog.json.gz", "the full catalog")
	out := flag.String("out", "dist/web", "folder to write")
	flag.Parse()
	c, err := gamedata.Load(*in)
	if err != nil {
		fail(err)
	}
	descs := gamedata.TakeItemDescs(c)
	if err := write(filepath.Join(*out, "catalog.json.gz"), c); err != nil {
		fail(err)
	}
	for _, l := range gamedata.Langs {
		if err := write(filepath.Join(*out, "desc-"+l+".json.gz"), descs[l]); err != nil {
			fail(err)
		}
	}
}

// write saves v as gzipped JSON and says how big it came out.
func write(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw, _ := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err := json.NewEncoder(zw).Encode(v); err != nil {
		f.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	st, _ := os.Stat(path)
	fmt.Printf("%s: %.2f MB\n", path, float64(st.Size())/(1<<20))
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
