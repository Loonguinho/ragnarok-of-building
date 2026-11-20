package view

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestTrFormats(t *testing.T) {
	cases := []struct{ l, in, want string }{
		{"en", "Salvar", "Save"},
		{"es", "Salvar", "Guardar"},
		{"pt", "Salvar", "Salvar"},
		{"en", "texto que não está no dicionário", "texto que não está no dicionário"},
		{"en", "100 × tamanho 75% × elemento 150% = 112", "100 × size 75% × element 150% = 112"},
		{"en", "50 × elemento 150% = 75", "50 × element 150% = 75"},
		{"es", "Nv. 10", "Nv. 10"},
		{"en", "Nv. 10", "Lv. 10"},
		{"en", "O elemento Ghost não causa dano em Ghost 4.", "Element Ghost does no damage to Ghost 4."},
		{"en", "Build — Magus", "Build — Magus"},
	}
	for _, c := range cases {
		if got := tr(c.l, c.in); got != c.want {
			t.Errorf("tr(%s, %q) = %q, want %q", c.l, c.in, got, c.want)
		}
	}
}

func TestMonsterSearchTranslated(t *testing.T) {
	c := testCatalog()
	c.Monsters = map[int]*gamedata.Monster{1002: {ID: 1002, Name: "Poring", Level: 1, Race: "Plant", Element: "Water", ElementLv: 1}}
	s, err := newTestServer(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	for l, want := range map[string]string{"pt": "Planta · Água", "en": "Plant · Water", "es": "Planta · Agua"} {
		rows := (&Views{s: s}).SearchMonsters("poring", l)
		if len(rows) != 1 || rows[0].Race+" · "+rows[0].Element != want {
			t.Errorf("%s: %+v, want %q", l, rows, want)
		}
	}
}
