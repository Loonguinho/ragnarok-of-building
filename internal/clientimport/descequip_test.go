package clientimport

import (
	"slices"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestDescEquip(t *testing.T) {
	c := &gamedata.Catalog{Items: map[int]*gamedata.Item{
		480926: {ID: 480926, Name: gamedata.Text{"pt": "Capa Desconhecida FOR"}, Slots: 1, Desc: gamedata.Lines{"pt": {
			"Capa feita de couro.", "Tipo: ^777777Capa^000000", "DEF: ^77777740^000000 DEFM: ^7777770^000000",
			"Peso: ^77777750^000000", "Nível necessário: ^777777100^000000", "Classes: ^777777Todas^000000"}}},
		400001: {ID: 400001, Name: gamedata.Text{"pt": "Chapéu"}, Desc: gamedata.Lines{"pt": {
			"Tipo: Equip. para Cabeça Defense: 0", "Equipa em: Topo, Meio"}}},
		16741: {ID: 16741, Name: gamedata.Text{"pt": "Espadão"}, Desc: gamedata.Lines{"pt": {
			"Tipo: Espada de Duas Mãos", "ATQ: 220 ATQM: 0", "Nível da arma: 4"}}},
		14244: {ID: 14244, Name: gamedata.Text{"pt": "Cx Anel", "en": "Clamorous Ring Box"}, Desc: gamedata.Lines{"pt": {"Tipo: Acessório"}}},
		1201:  {ID: 1201, Type: "Weapon", SubType: "Dagger", Locations: []string{"Right_Hand"}, Attack: 17},
	}}
	if n := DescEquip(c); n != 3 {
		t.Errorf("typed %d items, want 3", n)
	}
	if it := c.Items[480926]; it.Type != "Armor" || !slices.Equal(it.Locations, []string{"Garment"}) || it.Defense != 40 || it.EquipLvMin != 100 || it.Weight != 50 || !it.Refineable {
		t.Errorf("cape: %+v", it)
	}
	if it := c.Items[400001]; !slices.Contains(it.Locations, "Head_Top") || !slices.Contains(it.Locations, "Head_Mid") {
		t.Errorf("headgear: %+v", it.Locations)
	}
	if it := c.Items[16741]; it.SubType != "2hSword" || it.Attack != 220 || it.WeaponLv != 4 || !slices.Equal(it.Locations, []string{"Both_Hand"}) {
		t.Errorf("weapon: %+v", it)
	}
	if c.Items[14244].Type != "" || c.Items[1201].Attack != 17 {
		t.Error("boxes and rAthena items must stay as they are")
	}
}
