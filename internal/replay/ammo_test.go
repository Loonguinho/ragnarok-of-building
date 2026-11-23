package replay

import "testing"

// TestWornAmmo: of the stacks flagged as worn, the one the recording spends
// was loaded; then the one it equips; a lone stack is the one.
func TestWornAmmo(t *testing.T) {
	inv := []Item{{ItemID: 1752, Equipped: equipAmmo, Index: 5}, {ItemID: 1773, Equipped: equipAmmo, Index: 9}, {ItemID: 501, Index: 7}}
	spend := []byte{0xfa, 0x07, 0, 0, 9, 0, 3, 0}
	equip := []byte{0x3c, 0x01, 5, 0}
	if got := wornAmmo(inv, [][]byte{equip, spend}); got != 1773 {
		t.Errorf("spent: %d", got)
	}
	if got := wornAmmo(inv, [][]byte{equip}); got != 1752 {
		t.Errorf("equipped: %d", got)
	}
	if got := wornAmmo(inv, nil); got != 0 {
		t.Errorf("undecided: %d", got)
	}
	if got := wornAmmo(inv[:1], nil); got != 1752 {
		t.Errorf("lone: %d", got)
	}
}
