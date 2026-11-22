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
