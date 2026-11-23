package clientimport

import (
	"regexp"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

var (
	reAmmoType = regexp.MustCompile(`(?i)tipo:\s*munição`)
	reAmmoEle  = regexp.MustCompile(`(?i)propriedade:\s*(\S+)`)
	reBonusEle = regexp.MustCompile(`bonus bAtkEle,\s*Ele_\w+;\s*`)
)

// Ammo takes the ammunition's ATK and element from the client description
// (LATAM values over rAthena's). Items the client does not call "Munição"
// (surplus capsules, bombs) are left out of the equipment.
func Ammo(c *gamedata.Catalog) int {
	n := 0
	for _, it := range c.Items {
		if it.Type != "Ammo" {
			continue
		}
		text := ""
		for _, l := range it.Desc["pt"] {
			text += strings.TrimSpace(descColor.ReplaceAllString(l, "")) + "\n"
		}
		if !reAmmoType.MatchString(text) {
			it.Type, it.SubType, it.Locations, it.Script = "", "", nil, ""
			it.Attack = 0
			continue
		}
		n++
		if m := reDescATK.FindStringSubmatch(text); m != nil {
			it.Attack, _ = strconv.Atoi(m[1])
		}
		if m := reAmmoEle.FindStringSubmatch(text); m != nil {
			if ele := descEle[strings.ToLower(strings.TrimRight(m[1], "."))]; ele != "" {
				it.Script = strings.TrimSpace(reBonusEle.ReplaceAllString(it.Script, ""))
				if ele != "Ele_Neutral" {
					it.Script = strings.TrimSpace("bonus bAtkEle," + ele + ";\n" + it.Script)
				}
			}
		}
	}
	return n
}
