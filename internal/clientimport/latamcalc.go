package clientimport

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// latamMonster is one entry of latam-ro-calc's monster.json (MIT,
// github.com/adsonpleal/latam-ro-calc): the monsters LATAM players fight,
// with stats checked against LATAM recordings.
type latamMonster struct {
	ID    int    `json:"id"`
	Aegis string `json:"dbname"`
	Name  string `json:"name"`
	Stats struct {
		Level   int    `json:"level"`
		Health  int    `json:"health"`
		Str     int    `json:"str"`
		Agi     int    `json:"agi"`
		Vit     int    `json:"vit"`
		Int     int    `json:"int"`
		Dex     int    `json:"dex"`
		Luk     int    `json:"luk"`
		Def     int    `json:"defense"`
		MDef    int    `json:"magicDefense"`
		Res     int    `json:"res"`
		MRes    int    `json:"mres"`
		Element string `json:"elementName"` // "Neutral 2"
		Size    string `json:"scaleName"`
		Race    string `json:"raceName"`
		MVP     int    `json:"mvp"`
		Class   int    `json:"class"` // 1 = boss
	} `json:"stats"`
}

// latamDamageTaken is the % of damage a few LATAM targets take, from
// latam-ro-calc's monster-damage-reduction.ts (measured on recordings):
// Varmundt's Biosphere (90% off, its four MVPs 99%), Betelgeuse 99%,
// Schulang and Freyja 90%.
func latamDamageTaken(id int) int {
	switch {
	case id == 21555 || id == 21563 || id == 21571 || id == 21579:
		return 1
	case id >= 21548 && id <= 21579:
		return 10
	case id == 20994:
		return 1
	case id == 21360 || id == 21361:
		return 10
	}
	return 0
}

// MergeLatamCalc reads latam-ro-calc's monster.json: monsters rAthena knows
// only by ID (or not at all) are added, and the stats of the others follow
// LATAM's. It returns how many were added and updated.
func MergeLatamCalc(c *gamedata.Catalog, path string) (added, updated int, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var list map[string]latamMonster
	if err := json.Unmarshal(b, &list); err != nil {
		return 0, 0, err
	}
	if c.Monsters == nil {
		c.Monsters = map[int]*gamedata.Monster{}
	}
	for _, e := range list {
		s := e.Stats
		if e.ID <= 0 || s.Health <= 0 {
			continue
		}
		ele, lvS, _ := strings.Cut(s.Element, " ")
		lv, _ := strconv.Atoi(lvS)
		if ele == "Shadow" {
			ele = "Dark"
		}
		race := s.Race
		if strings.EqualFold(strings.ReplaceAll(race, "-", ""), "demihuman") {
			race = "Demihuman"
		}
		m := c.Monsters[e.ID]
		if m == nil {
			m = &gamedata.Monster{ID: e.ID, Name: e.Name, DamageTaken: 100}
			c.Monsters[e.ID] = m
			delete(c.MonsterStubs, e.ID)
			added++
		} else {
			updated++
		}
		m.Level, m.HP, m.DEF, m.MDEF, m.RES, m.MRES = s.Level, s.Health, s.Def, s.MDef, s.Res, s.MRes
		m.Stats = [6]int{s.Str, s.Agi, s.Vit, s.Int, s.Dex, s.Luk}
		m.Element, m.ElementLv = def(ele, "Neutral"), min(max(lv, 1), 4)
		m.Size, m.Race = def(s.Size, "Small"), def(race, "Formless")
		m.MVP = s.MVP == 1
		if s.Class == 1 {
			m.Class = "Boss"
		} else if m.Class == "" {
			m.Class = "Normal"
		}
		if d := latamDamageTaken(e.ID); d > 0 {
			m.DamageTaken = d
		}
		m.Source = "latam-ro-calc"
	}
	for id, card := range mobCards {
		if m := c.Monsters[id]; m != nil && len(m.Names) == 0 {
			m.Names = monsterNames(c, []int{card})
		}
	}
	return added, updated, nil
}

func def(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
