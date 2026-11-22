package rathena

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadPets: active and commented-out entries (the evolved pets rAthena
// has no monster stats for) are read; an active entry wins.
func TestLoadPets(t *testing.T) {
	src := `Body:
  - Mob: PORING
    EggItem: Poring_Egg
    Script: >
      bonus bLuk,2;
#  - Mob: PORING
#    EggItem: Poring_Egg
#    Script: >
#      bonus bLuk,99;
#
#  - Mob: MISTRESS3
#    EggItem: aegis_9193
#    Script: >
#      .@i = getpetinfo(PETINFO_INTIMATE);
#
#      if (.@i >= PET_INTIMATE_LOYAL) {
#         bonus bNoSizeFix;
#      }
  - Mob: DROPS
    EggItem: Drops_Egg
`
	path := filepath.Join(t.TempDir(), "pet_db.yml")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	db := &DB{Mobs: map[int]*Mob{1002: {ID: 1002, Aegis: "PORING"}, 1113: {ID: 1113, Aegis: "DROPS"}},
		MobStubs: map[int]string{22405: "MISTRESS3"}}
	if err := db.loadPets(path, map[string]int{"Poring_Egg": 9001, "Drops_Egg": 9002}); err != nil {
		t.Fatal(err)
	}
	if p := db.Pets[9001]; p == nil || p.Mob != 1002 || p.Script != "bonus bLuk,2;\n" {
		t.Errorf("Poring = %+v, want the active entry", p)
	}
	if p := db.Pets[9193]; p == nil || p.Mob != 22405 {
		t.Errorf("commented Queen Bee = %+v", p)
	}
	if db.Pets[9002] != nil {
		t.Error("a pet without a script was kept")
	}
}
