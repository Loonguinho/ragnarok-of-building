package clientimport

import (
	"encoding/json"
	"os"
	"slices"
	"strings"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/rathena"
)

// Items of other servers the LATAM client does not have yet: the Good and
// Evil gear (crowns and boots, with the weapons of their sets and the
// enchants of their positions). Once the client lists one, its own name,
// texts and slots are used and nothing here applies to it.

var previewPrefixes = []string{"GoodnEvil_Circlet_", "FateSin_Boots_", "Justice_Vigor_", "Injustice_Vigor_", "Ch01_S_E_GAE"}

// notGear are items whose AegisName starts like the gear's: boxes and hammers.
var notGear = []string{"_Box", "_Hammer"}

// previewClasses is each suffix's class (the code of its job), and the
// weapon the Night Watch crowns are for.
var previewClasses = map[string][]string{
	"DK": {"JT_DRAGON_KNIGHT"}, "IG": {"JT_IMPERIAL_GUARD"}, "AG": {"JT_ARCH_MAGE"}, "EM": {"JT_ELEMENTAL_MASTER"},
	"CD": {"JT_CARDINAL"}, "IQ": {"JT_INQUISITOR"}, "MT": {"JT_MEISTER"}, "BO": {"JT_BIOLO"},
	"SHC": {"JT_SHADOW_CROSS"}, "ABC": {"JT_ABYSS_CHASER"}, "WH": {"JT_WINDHAWK"},
	"TR": {"JT_TROUBADOUR", "JT_TROUVERE"}, "SS": {"JT_SHINKIRO", "JT_SHIRANUI"},
	"SKE": {"JT_SKY_EMPEROR"}, "SOA": {"JT_SOUL_ASCETIC"}, "HN": {"JT_HYPER_NOVICE"}, "SH": {"JT_SPIRIT_HANDLER"},
	"NW": {"JT_NIGHT_WATCH"}, "NW2": {"JT_NIGHT_WATCH"}, "NW3": {"JT_NIGHT_WATCH"}, "NW4": {"JT_NIGHT_WATCH"}, "NW5": {"JT_NIGHT_WATCH"},
}

var previewWeapons = map[string]gamedata.Text{
	"GoodnEvil_Circlet_NW":  {"pt": "Lança-Granadas", "es": "Lanzagranadas"},
	"GoodnEvil_Circlet_NW2": {"pt": "Revólver", "es": "Revólver"},
	"GoodnEvil_Circlet_NW3": {"pt": "Metralhadora", "es": "Ametralladora"},
	"GoodnEvil_Circlet_NW4": {"pt": "Espingarda", "es": "Escopeta"},
	"GoodnEvil_Circlet_NW5": {"pt": "Rifle", "es": "Rifle"},
}

// previewNames are the families' names, as in the client's own pieces
// (Coroa Ambivalente Imperial, Essência Infernal Arcana...).
var previewNames = map[string]gamedata.Text{
	"GoodnEvil_Circlet_": {"pt": "Coroa Ambivalente", "es": "Corona Ambivalente"},
	"FateSin_Boots_":     {"pt": "Bota Ambivalente", "es": "Botas Ambivalentes"},
	"Justice_Vigor_":     {"pt": "Essência Celestial", "es": "Esencia Celestial"},
	"Injustice_Vigor_":   {"pt": "Essência Infernal", "es": "Esencia Infernal"},
}

var previewTag = gamedata.Text{"pt": " (em breve)", "es": " (próximamente)", "en": " (soon)"}

// addPreviewItems puts in the catalog the preview items it lacks, for
// MergeRathena to fill in. It returns how many.
func addPreviewItems(c *gamedata.Catalog, db *rathena.DB) int {
	byID := map[int]string{}
	want := map[int]bool{}
	seed := map[int]bool{}
	for a, id := range db.Aegis {
		byID[id] = a
		for _, p := range previewPrefixes {
			if strings.HasPrefix(a, p) && !slices.ContainsFunc(notGear, func(s string) bool { return strings.HasSuffix(a, s) }) {
				want[id] = true
				seed[id] = seed[id] || p == "GoodnEvil_Circlet_"
			}
		}
	}
	// The weapons (and enchants) of the crowns' sets.
	for _, cb := range db.Combos {
		if slices.ContainsFunc(cb.Items, func(id int) bool { return seed[id] }) {
			for _, id := range cb.Items {
				want[id] = true
			}
		}
	}
	n := 0
	for id := range want {
		ri := db.Items[id]
		if c.Items[id] != nil || ri == nil {
			continue
		}
		it := &gamedata.Item{ID: id, Preview: true, Slots: ri.Slots, Name: previewName(c, byID[id], ri.Name), Desc: gamedata.Lines{}}
		c.Items[id] = it
		n++
	}
	return n
}

func previewName(c *gamedata.Catalog, aegis, en string) gamedata.Text {
	name := gamedata.Text{"pt": en, "es": en, "en": en}
	for prefix, base := range previewNames {
		suffix, ok := strings.CutPrefix(aegis, prefix)
		codes := previewClasses[suffix]
		if !ok || len(codes) == 0 {
			continue
		}
		for _, l := range []string{"pt", "es"} {
			var of []string
			for _, code := range codes {
				for _, j := range c.Jobs {
					if j.Code == code {
						if s := j.Name[l]; s != "" {
							of = append(of, s)
						}
					}
				}
			}
			s := base[l] + " (" + strings.Join(of, "/")
			if w := previewWeapons[aegis][l]; w != "" {
				s += " - " + w
			}
			name[l] = s + ")"
		}
	}
	if n, ok := strings.CutPrefix(aegis, "Ch01_S_E_GAE0"); ok {
		if n == "6" {
			name["pt"], name["es"] = "Cristal Dimensional (Bem e Mal)", "Cristal Dimensional (Bien y Mal)"
		} else {
			name["pt"], name["es"] = "Cristal da Fronteira (Bem e Mal) "+n, "Cristal de la Frontera (Bien y Mal) "+n
		}
	}
	for l, t := range previewTag {
		name[l] += t
	}
	return name
}

// PreviewTexts gives the preview items latam-ro-calc's English description,
// and leaves their bonuses to rAthena (there is no LATAM text to check the
// LATAM script against).
func PreviewTexts(c *gamedata.Catalog, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var items map[string]struct {
		ID          int    `json:"id"`
		Res         string `json:"resName"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	notes := gamedata.Text{
		"pt": "Ainda não chegou ao LATAM: dados de outros servidores, que podem mudar aqui. Texto em inglês.",
		"es": "Aún no llegó a LATAM: datos de otros servidores, que pueden cambiar aquí. Texto en inglés.",
		"en": "Not in LATAM yet: data from other servers, which may change here.",
	}
	for _, x := range items {
		it := c.Items[x.ID]
		if it == nil || !it.Preview {
			continue
		}
		it.Resource = x.Res
		for l, n := range notes {
			it.Desc[l] = append([]string{n, "--------------------------"}, strings.Split(strings.TrimSpace(x.Description), "\n")...)
		}
		it.Latam, it.LatamProcs = nil, nil
	}
	for _, it := range c.Items {
		if it.Preview && len(it.Desc) == 0 {
			for l, n := range notes {
				it.Desc[l] = []string{n}
			}
		}
	}
	return nil
}
