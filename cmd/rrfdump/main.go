// rrfdump prints the character snapshot of .rrf replays, checked against the
// game catalog (unknown jobs/items/skills are flagged).
//
//	go run ./cmd/rrfdump C:/Gravity/Ragnarok/Replay/*.rrf
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/replay"
)

func main() {
	c, _ := gamedata.Load("data/gen/catalog.json.gz")
	for _, path := range os.Args[1:] {
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Println(path, err)
			continue
		}
		s, err := replay.Decode(b)
		if err != nil {
			fmt.Printf("%-40s ERROR %v\n", filepath.Base(path), err)
			continue
		}
		job := "?"
		if c != nil && c.Jobs[s.Job] != nil {
			job = c.Jobs[s.Job].Name.Get("pt")
		}
		unknownItems, unknownSkills := 0, 0
		if c != nil {
			for _, it := range append(s.Equipped, s.Inventory...) {
				if c.Items[it.ItemID] == nil {
					unknownItems++
				}
			}
			for id := range s.Skills {
				if c.Skills[id] == nil {
					unknownSkills++
				}
			}
		}
		fmt.Printf("%-40s %s (%d) Lv %d/%d stats %v traits %v | equipped %d, inventory %d, skills %d | unknown items %d, skills %d\n",
			filepath.Base(path), job, s.Job, s.BaseLv, s.JobLv, s.Stats, s.Traits, len(s.Equipped), len(s.Inventory), len(s.Skills), unknownItems, unknownSkills)
		if len(os.Args) == 2 && c != nil {
			for _, it := range s.Equipped {
				name := "?"
				if x := c.Items[it.ItemID]; x != nil {
					name = x.Name.Get("pt")
				}
				fmt.Printf("    eq 0x%06x  +%-2d %-40s cards %v grade %d\n", it.Equipped, it.Refine, name, it.Cards, it.Grade)
			}
		}
	}
}
