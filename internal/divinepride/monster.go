package divinepride

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// monsterJSON is the part of a Monster response the damage calc needs.
type monsterJSON struct {
	ID           int      `json:"id"`
	Level        int      `json:"level"`
	Health       int      `json:"health"`
	Race         string   `json:"race"`
	Element      string   `json:"element"` // "Neutral 2"
	ElementLevel int      `json:"elementLevel"`
	Size         string   `json:"size"`
	Type         string   `json:"type"` // Normal, Boss, MVP
	Def          int      `json:"def"`
	MDef         int      `json:"mDef"`
	Res          int      `json:"res"`
	MRes         int      `json:"mRes"`
	Str          int      `json:"str"`
	Agi          int      `json:"agi"`
	Vit          int      `json:"vit"`
	Int          int      `json:"int"`
	Dex          int      `json:"dex"`
	Luk          int      `json:"luk"`
	Attributes   []string `json:"attributes"`
}

// "Taking only 1% of the damage dealt to it".
var damageTaken = regexp.MustCompile(`(?i)taking only (\d+)% of the damage`)

// Divine Pride names that differ from rAthena's.
var raceNames = map[string]string{"Demi-Human": "Demihuman", "Demi Human": "Demihuman", "Human": "Demihuman"}
var elementNames = map[string]string{"Shadow": "Dark"}

// ParseMonster maps a Monster response to a damage target named name.
func ParseMonster(b []byte, name string) (*gamedata.Monster, error) {
	var d monsterJSON
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if d.ID == 0 || d.Health <= 0 {
		return nil, errors.New("divinepride: monster without stats")
	}
	ele, _, _ := strings.Cut(d.Element, " ")
	if n, ok := elementNames[ele]; ok {
		ele = n
	}
	race := d.Race
	if n, ok := raceNames[race]; ok {
		race = n
	}
	m := &gamedata.Monster{
		ID: d.ID, Name: name, Level: d.Level, HP: d.Health, DEF: d.Def, MDEF: d.MDef,
		RES: d.Res, MRES: d.MRes, Stats: [6]int{d.Str, d.Agi, d.Vit, d.Int, d.Dex, d.Luk},
		Size: d.Size, Race: race, Element: ele, ElementLv: min(max(d.ElementLevel, 1), 4),
		Class: "Normal", DamageTaken: 100, Source: "Divine Pride",
	}
	if m.Size == "" {
		m.Size = "Small"
	}
	if m.Race == "" {
		m.Race = "Formless"
	}
	if m.Element == "" {
		m.Element = "Neutral"
	}
	switch strings.ToLower(d.Type) {
	case "mvp":
		m.Class, m.MVP = "Boss", true
	case "boss":
		m.Class = "Boss"
	}
	for _, a := range d.Attributes {
		if g := damageTaken.FindStringSubmatch(a); g != nil {
			m.DamageTaken, _ = strconv.Atoi(g[1])
		}
	}
	return m, nil
}
