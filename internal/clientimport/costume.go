package clientimport

import (
	"regexp"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// Costume enchant stones (Mestre II (Meio), ATQ +1% (Topo)...) have no
// location in rAthena: the client names their position. They get it here so
// each one fits only its costume slot.
var reCostumeStone = regexp.MustCompile(`\((Topo|Meio|Baixo|Capa)\)\s*$`)

var costumeStoneLoc = map[string]string{
	"Topo": "Costume_Head_Top", "Meio": "Costume_Head_Mid", "Baixo": "Costume_Head_Low", "Capa": "Costume_Garment",
}

// CostumeStones sets the costume slot of every stone; it returns how many.
func CostumeStones(c *gamedata.Catalog) int {
	n := 0
	for _, it := range c.Items {
		if it.Type != "Card" {
			continue
		}
		m := reCostumeStone.FindStringSubmatch(strings.TrimSpace(it.Name.Get("pt")))
		if m == nil {
			continue
		}
		it.Locations = []string{costumeStoneLoc[m[1]]}
		if it.SubType == "" {
			it.SubType = "Enchant"
		}
		n++
	}
	return n
}
