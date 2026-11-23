package clientimport

import (
	"regexp"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// descKind maps the description's "Tipo:" (PT client) to rAthena's type,
// subtype and equip locations.
var descKind = map[string]struct {
	typ, sub string
	locs     []string
}{
	"capa":                 {"Armor", "", []string{"Garment"}},
	"calçado":              {"Armor", "", []string{"Shoes"}},
	"armadura":             {"Armor", "", []string{"Armor"}},
	"roupa":                {"Armor", "", []string{"Armor"}},
	"escudo":               {"Armor", "", []string{"Left_Hand"}},
	"acessório":            {"Armor", "", []string{"Both_Accessory"}},
	"aces. esquerdo":       {"Armor", "", []string{"Left_Accessory"}},
	"aces. direito":        {"Armor", "", []string{"Right_Accessory"}},
	"equip. para cabeça":   {"Armor", "", nil}, // position from "Equipa em:"
	"adaga":                {"Weapon", "Dagger", []string{"Right_Hand"}},
	"espada":               {"Weapon", "1hSword", []string{"Right_Hand"}},
	"espada de duas mãos":  {"Weapon", "2hSword", []string{"Both_Hand"}},
	"lança":                {"Weapon", "1hSpear", []string{"Right_Hand"}},
	"lança de duas mãos":   {"Weapon", "2hSpear", []string{"Both_Hand"}},
	"machado":              {"Weapon", "1hAxe", []string{"Right_Hand"}},
	"machado de duas mãos": {"Weapon", "2hAxe", []string{"Both_Hand"}},
	"maça":                 {"Weapon", "Mace", []string{"Right_Hand"}},
	"cajado":               {"Weapon", "Staff", []string{"Right_Hand"}},
	"cajado de duas mãos":  {"Weapon", "2hStaff", []string{"Both_Hand"}},
	"arco":                 {"Weapon", "Bow", []string{"Both_Hand"}},
	"soqueira":             {"Weapon", "Knuckle", []string{"Right_Hand"}},
	"instrumento musical":  {"Weapon", "Musical", []string{"Right_Hand"}},
	"chicote":              {"Weapon", "Whip", []string{"Right_Hand"}},
	"livro":                {"Weapon", "Book", []string{"Right_Hand"}},
	"katar":                {"Weapon", "Katar", []string{"Both_Hand"}},
	"shuriken huuma":       {"Weapon", "Huuma", []string{"Both_Hand"}},
	"pistola":              {"Weapon", "Revolver", []string{"Both_Hand"}},
	"rifle":                {"Weapon", "Rifle", []string{"Both_Hand"}},
	"espingarda":           {"Weapon", "Shotgun", []string{"Both_Hand"}},
	"metralhadora":         {"Weapon", "Gatling", []string{"Both_Hand"}},
	"lança-granadas":       {"Weapon", "Grenade", []string{"Both_Hand"}},
}

var headPos = map[string]string{"topo": "Head_Top", "top": "Head_Top", "meio": "Head_Mid", "mid": "Head_Mid",
	"middle": "Head_Mid", "baixo": "Head_Low", "lower": "Head_Low", "low": "Head_Low"}

var (
	descColor = regexp.MustCompile(`\^[0-9a-fA-F]{6}`)
	descNum   = func(label string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)(?:^|\s)` + label + `:\s*(\d+)`)
	}
	reDescATK    = descNum(`ATQ`)
	reDescMATK   = descNum(`ATQM`)
	reDescDEF    = descNum(`DEF`)
	reDescWLv    = descNum(`Nível da arma`)
	reDescEquip  = descNum(`Nível necessário`)
	reDescWeight = descNum(`Peso`)
)

// DescEquip gives the LATAM-only equipment rAthena doesn't have (no type, so
// no slot rules) its type, slot, ATK/DEF, weapon and required level from the
// client description ("Tipo: Capa", "Equipa em: Topo", "DEF: 40", ...), so
// the equipment picker can offer it. Boxes are skipped: their text describes
// what they hold.
func DescEquip(c *gamedata.Catalog) int {
	n := 0
	for _, it := range c.Items {
		if it.Type != "" || isBox(it) {
			continue
		}
		var lines []string
		for _, l := range it.Desc["pt"] {
			lines = append(lines, strings.TrimSpace(descColor.ReplaceAllString(l, "")))
		}
		kind, pos := "", ""
		for _, l := range lines {
			if v, ok := strings.CutPrefix(l, "Tipo:"); ok && kind == "" {
				v = strings.TrimSpace(v)
				if i := strings.Index(v, " Defense"); i >= 0 { // "Equip. para Cabeça Defense: 0"
					v = v[:i]
				}
				kind = strings.ToLower(v)
			}
			if v, ok := strings.CutPrefix(l, "Equipa em:"); ok && pos == "" {
				pos = strings.ToLower(strings.TrimSpace(v))
			}
		}
		k, ok := descKind[kind]
		if !ok {
			continue
		}
		locs := k.locs
		if locs == nil { // headgear
			for word, loc := range headPos {
				if strings.Contains(pos, word) {
					locs = appendUnique(locs, loc)
				}
			}
			if len(locs) == 0 {
				locs = []string{"Head_Top"}
			}
		}
		it.Type, it.SubType, it.Locations = k.typ, k.sub, locs
		it.Refineable = true
		for _, l := range lines {
			num := func(re *regexp.Regexp) int {
				if m := re.FindStringSubmatch(l); m != nil {
					v, _ := strconv.Atoi(m[1])
					return v
				}
				return -1
			}
			if v := num(reDescATK); v >= 0 && it.Attack == 0 {
				it.Attack = v
			}
			if v := num(reDescMATK); v >= 0 && it.MagicAttack == 0 {
				it.MagicAttack = v
			}
			if v := num(reDescDEF); v >= 0 && it.Defense == 0 {
				it.Defense = v
			}
			if v := num(reDescWLv); v > 0 {
				it.WeaponLv = v
			}
			if v := num(reDescEquip); v > 0 {
				it.EquipLvMin = v
			}
			if v := num(reDescWeight); v >= 0 && it.Weight == 0 {
				it.Weight = v
			}
		}
		if it.Type == "Armor" && it.ArmorLv == 0 {
			it.ArmorLv = 1
		}
		it.DescEquip = true
		n++
	}
	return n
}

func isBox(it *gamedata.Item) bool {
	name := strings.ToLower(it.Name.Get("pt"))
	en := strings.ToLower(it.Name.Get("en"))
	return strings.HasPrefix(name, "caixa") || strings.HasPrefix(name, "pacote") || strings.Contains(en, " box") || strings.HasSuffix(en, "box")
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

var (
	reDescReqLv    = regexp.MustCompile(`(?i)^N[ií]vel (?:necess[aá]rio|m[ií]nimo)\s*:\s*(\d+)`)
	reDescWeaponLv = regexp.MustCompile(`(?i)^N[ií]vel da arma\s*:\s*(\d+)`)
	reDescArmorLv  = regexp.MustCompile(`(?i)^N[ií]vel do equip(?:amento|\.)?\s*:\s*(\d+)`)
)

// DescLevels takes the required base level, the weapon level and the armor
// level from the LATAM client's description where it gives them: rAthena's
// are kRO's and differ on hundreds of items (Jaqueta de Motociclista needs
// level 55 in LATAM, not 1). Returns how many items changed.
func DescLevels(c *gamedata.Catalog) int {
	n := 0
	for _, it := range c.Items {
		if it.Type != "Weapon" && it.Type != "Armor" && it.Type != "ShadowGear" {
			continue
		}
		changed := false
		for _, l := range it.Desc["pt"] {
			l = strings.TrimSpace(descColor.ReplaceAllString(l, ""))
			num := func(re *regexp.Regexp) int {
				if m := re.FindStringSubmatch(l); m != nil {
					v, _ := strconv.Atoi(m[1])
					return v
				}
				return 0
			}
			if v := num(reDescReqLv); v > 0 && v != it.EquipLvMin {
				it.EquipLvMin, changed = v, true
			}
			if v := num(reDescWeaponLv); v > 0 && it.Type == "Weapon" && v != it.WeaponLv {
				it.WeaponLv, changed = v, true
			}
			if v := num(reDescArmorLv); v > 0 && it.Type == "Armor" && v != it.ArmorLv {
				it.ArmorLv, changed = v, true
			}
		}
		if changed {
			n++
		}
	}
	return n
}
