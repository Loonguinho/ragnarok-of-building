package clientimport

import (
	"path/filepath"
	"ragnarok-of-building/internal/config"
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
