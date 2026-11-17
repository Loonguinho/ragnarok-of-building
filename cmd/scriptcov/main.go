// scriptcov reports how much of the equipment script corpus the status
// evaluator applies, and the most common statements it skips.
//
//	go run ./cmd/scriptcov
package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

var stmt = regexp.MustCompile(`(?m)^\s*(bonus[0-9]?\s+\w+|[a-z_]+)`)

func main() {
	c, err := gamedata.Load("data/gen/catalog.json.gz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	full, partial, none, total := 0, 0, 0, 0
	skipped := map[string]int{}
	for _, it := range c.Items {
		if it.Script == "" {
			continue
		}
		switch it.Type {
		case "Armor", "Weapon", "Card", "ShadowGear":
		default:
			continue
		}
		total++
		r := script.Eval(it.Script, script.Context{Refine: 9, BaseLv: 200, JobLv: 50, Param: map[string]int{}})
		switch {
		case r.Skipped == 0:
			full++
		case len(r.Bonus) > 0:
			partial++
		default:
			none++
		}
		if r.Skipped > 0 {
			for _, m := range stmt.FindAllStringSubmatch(it.Script, -1) {
				s := m[1]
				if s == "if" || s == "else" {
					continue
				}
				if name, ok := strings.CutPrefix(s, "bonus "); ok && script.Known(strings.TrimSpace(name)) {
					continue
				}
				skipped[s]++
			}
		}
	}
	fmt.Printf("scripts %d | fully applied %d (%.0f%%) | partial %d | none %d\n",
		total, full, 100*float64(full)/float64(total), partial, none)
	type kv struct {
		k string
		v int
	}
	var top []kv
	for k, v := range skipped {
		top = append(top, kv{k, v})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].v > top[j].v })
	fmt.Println("most common unapplied statements:")
	for _, e := range top[:min(15, len(top))] {
		fmt.Printf("%6d  %s\n", e.v, e.k)
	}
}
