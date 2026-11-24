package clientimport

import (
	"regexp"
	"slices"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

var reDescSide = regexp.MustCompile(`(?i)^(?:Tipo|Equipa em)\s*:\s*(Aces\. Direito|Aces\. Esquerdo|Acessório)$`)

// DescSides takes the side of accessories and of accessory cards from the
// LATAM client's description, which recordings confirm: rAthena lists a few
// for either side. Returns how many items changed.
func DescSides(c *gamedata.Catalog) int {
	n := 0
	for _, it := range c.Items {
		if it.Type != "Armor" && (it.Type != "Card" || it.SubType == "Enchant") {
			continue
		}
		for _, l := range it.Desc["pt"] {
			m := reDescSide.FindStringSubmatch(strings.TrimSpace(descColor.ReplaceAllString(l, "")))
			if m == nil {
				continue
			}
			want := []string{"Both_Accessory"}
			switch strings.ToLower(m[1]) {
			case "aces. direito":
				want = []string{"Right_Accessory"}
			case "aces. esquerdo":
				want = []string{"Left_Accessory"}
			}
			if !slices.Equal(it.Locations, want) {
				it.Locations = want
				n++
			}
			break
		}
	}
	return n
}

// scriptFixes are rAthena scripts whose numbers differ from the LATAM
// description: {item: {rAthena's text, LATAM's}}.
var scriptFixes = map[int][2]string{
	27307: {`"RL_R_TRIP",15`, `"RL_R_TRIP",10`}, // Carta Bellare Mutante: Expurgar +10%
}

// FixScripts applies scriptFixes; returns how many items changed.
func FixScripts(c *gamedata.Catalog) int {
	n := 0
	for id, f := range scriptFixes {
		if it := c.Items[id]; it != nil && strings.Contains(it.Script, f[0]) {
			it.Script = strings.ReplaceAll(it.Script, f[0], f[1])
			n++
		}
	}
	return n
}
