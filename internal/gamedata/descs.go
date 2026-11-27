package gamedata

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// The web page ships the item descriptions apart from the catalog, one
// file per language: they are most of its size and only the tooltips show
// them, so the window can open before they arrive. (Skill descriptions
// stay: Skill.RatioAt reads damage from them.)

// ItemDescs is one language's item descriptions, by item ID.
type ItemDescs map[int][]string

// TakeItemDescs removes the items' descriptions from c and returns them
// per language, each as Lines.Get gives it (so a language the client has
// no text in shows the same fallback as before).
func TakeItemDescs(c *Catalog) map[string]ItemDescs {
	out := map[string]ItemDescs{}
	for _, l := range Langs {
		out[l] = ItemDescs{}
	}
	for id, it := range c.Items {
		if len(it.Desc) == 0 {
			continue
		}
		for _, l := range Langs {
			if lines := it.Desc.Get(l); len(lines) > 0 {
				out[l][id] = lines
			}
		}
		it.Desc = nil
	}
	return out
}

// PutItemDescs gives the items their descriptions in lang (from
// TakeItemDescs), replacing what they had in it.
func PutItemDescs(c *Catalog, lang string, d ItemDescs) {
	for id, lines := range d {
		it := c.Items[id]
		if it == nil {
			continue
		}
		if it.Desc == nil {
			it.Desc = Lines{}
		}
		it.Desc[lang] = lines
	}
}

// ReadItemDescs reads one language's descriptions as websplit writes them
// (gzipped JSON, or plain if a server unzipped it), calling pause between
// items so a single-threaded page can draw meanwhile.
func ReadItemDescs(r io.Reader, pause func()) (ItemDescs, error) {
	br := bufio.NewReader(r)
	var src io.Reader = br
	if magic, _ := br.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("gamedata: descriptions: %w", err)
		}
		src = zr
	}
	dec := json.NewDecoder(src)
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, fmt.Errorf("gamedata: descriptions: not an object (%v)", err)
	}
	out := ItemDescs{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("gamedata: descriptions: %w", err)
		}
		key, _ := t.(string)
		id, err := strconv.Atoi(key)
		if err != nil {
			return nil, fmt.Errorf("gamedata: descriptions: item %q", key)
		}
		var lines []string
		if err := dec.Decode(&lines); err != nil {
			return nil, fmt.Errorf("gamedata: descriptions: item %d: %w", id, err)
		}
		out[id] = lines
		if pause != nil {
			pause()
		}
	}
	return out, nil
}
