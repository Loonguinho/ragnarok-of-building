package clientimport

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

var reLatamSlots = regexp.MustCompile(`\s*\[\d+\]$`)

// LatamItemScripts copies latam-ro-calc's item scripts (item.json, MIT) to
// the catalog's items (Item.Latam), with the English name its EQUIP[...]
// conditions match on. Only bonus lines are kept (lists of strings): the
// structured auto-cast entries have no counterpart here. Returns how many
// items got a script.
func LatamItemScripts(path string, c *gamedata.Catalog) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var items map[string]struct {
		ID     int                        `json:"id"`
		Name   string                     `json:"name"`
		Script map[string]json.RawMessage `json:"script"`
	}
	if err := json.Unmarshal(b, &items); err != nil {
		return 0, err
	}
	n := 0
	for _, li := range items {
		it := c.Items[li.ID]
		if it == nil {
			continue
		}
		it.LatamName = strings.TrimSpace(reLatamSlots.ReplaceAllString(li.Name, ""))
		script := map[string][]string{}
		for k, raw := range li.Script {
			var lines []string
			if json.Unmarshal(raw, &lines) != nil || len(lines) == 0 {
				continue
			}
			if p, ok := strings.CutPrefix(k, "chance__"); ok {
				if it.LatamProcs == nil {
					it.LatamProcs = map[string][]string{}
				}
				it.LatamProcs[p] = lines
				continue
			}
			script[k] = lines
		}
		if len(script) > 0 {
			it.Latam = script
			n++
		}
	}
	return n, nil
}
