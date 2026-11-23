package clientimport

import (
	"path/filepath"
	"ragnarok-of-building/internal/config"
	"ragnarok-of-building/internal/gamedata"
	"slices"
	"testing"

	"ragnarok-of-building/internal/grf"
)

// TestEnchantListClient reads the real client's enchant list (skipped
// without the client): every block must parse, Gray Wolf Suits included.
func TestEnchantListClient(t *testing.T) {
	a, err := grf.Open(filepath.Join(config.ClientDir(), "data.grf"))
	if err != nil {
		t.Skip("no client")
	}
	defer a.Close()
	b, err := a.Read(`data\` + luaDir + `enchant\enchantlist.lub`)
	if err != nil {
		t.Skip("no enchant list")
	}
	infos, err := parseEnchantList(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) < 30 {
		t.Errorf("only %d enchant blocks read", len(infos))
	}
	found := false
	for _, in := range infos {
		if slices.Contains(in.targets, "Gray_W_Suits") {
			found = len(in.slots[3]) > 0 && slices.Contains(in.slots[3], "Wolf_Orb_Str_1")
		}
	}
	if !found {
		t.Error("Gray_W_Suits enchants not read")
	}
}

// TestNPCEnchants: the Memoráveis, missing from the client's enchant list,
// take stats on positions 2-4 and the special enchants on position 2.
func TestNPCEnchants(t *testing.T) {
	c := &gamedata.Catalog{Items: map[int]*gamedata.Item{18971: {ID: 18971}, 4704: {ID: 4704}, 29065: {ID: 29065}},
		Enchants: map[int]map[int][]int{}}
	addNPCEnchants(c)
	e := c.Enchants[18971]
	if !slices.Contains(e[1], 29065) || !slices.Contains(e[1], 4704) {
		t.Errorf("2nd position = %v, want STR+5 and Mettle 5", e[1])
	}
	for _, k := range []int{2, 3} {
		if !slices.Equal(e[k], []int{4704}) {
			t.Errorf("position %d = %v, want only the stat", k+1, e[k])
		}
	}
	if _, ok := c.Enchants[18972]; ok {
		t.Error("an item missing from the catalog got enchants")
	}
}

// TestNPCEnchantTables: the Illusion (Illusion Enchanter, RS26) and
// Automatron (Lisa) equipment get their NPC's enchants.
func TestNPCEnchantTables(t *testing.T) {
	items := map[int]*gamedata.Item{}
	for _, id := range []int{20847, 15377, 450127, 4700, 4742, 29535, 310082} {
		items[id] = &gamedata.Item{ID: id}
	}
	items[20847].Type, items[20847].Locations = "Armor", []string{"Garment"}
	c := &gamedata.Catalog{Items: items, Enchants: map[int]map[int][]int{}}
	addNPCEnchants(c)
	if e := c.Enchants[20847]; !slices.Contains(e[3], 4700) || !slices.Contains(e[2], 4742) {
		t.Errorf("Sobrevivente Ilusional: %v, want stats in the 3rd and 4th positions", e)
	}
	if e := c.Enchants[15377]; !slices.Contains(e[1], 29535) {
		t.Errorf("Colete Ilusión B: %v, want the armor modules", e)
	}
	if e := c.Enchants[450127]; !slices.Contains(e[1], 310082) { // B-DEF, module 1000105
		t.Errorf("Colete Automatron A: %v, want B-DEF", e)
	}
}

// TestLatamEnchants: items the client's list leaves out take latam-ro-calc's
// pools (Brasão de Schmidt, Bota Temporal); a client list is kept.
func TestLatamEnchants(t *testing.T) {
	c := &gamedata.Catalog{Items: map[int]*gamedata.Item{}, Enchants: map[int]map[int][]int{}}
	for _, id := range []int{32228, 22000, 4700, 4701, 29061, 99} {
		c.Items[id] = &gamedata.Item{ID: id}
	}
	aegis := map[string]int{"Schmidt_Insignia_Str": 32228, "Temporal_Str_Boots": 22000, "Strength3": 4702, "Expert_Archer1": 29061}
	c.Items[4702] = &gamedata.Item{ID: 4702}
	c.Enchants[22000] = map[int][]int{3: {99}} // the client lists it
	if n := addLatamEnchants(c, aegis); n != 1 {
		t.Errorf("filled %d items, want 1 (the Brasão)", n)
	}
	if got := c.Enchants[32228][3]; !slices.Contains(got, 4702) {
		t.Errorf("Brasão de Schmidt position 3: %v, want STR+3 (4702)", got)
	}
	if got := c.Enchants[22000]; len(got) != 1 || !slices.Equal(got[3], []int{99}) {
		t.Errorf("Bota Temporal kept the client list? %v", got)
	}
}
