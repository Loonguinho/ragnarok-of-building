package clientimport

import (
	"os"
	"path/filepath"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestMergeLatamCalc(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monster.json")
	const body = `{"20994":{"id":20994,"dbname":"MD_BETELGEUSE","name":"Betelgeuse","stats":{"level":250,"health":2000000000,
		"str":211,"agi":211,"vit":132,"int":152,"dex":279,"luk":108,"defense":346,"magicDefense":102,"res":500,"mres":500,
		"elementName":"Neutral 2","scaleName":"Large","raceName":"Dragon","mvp":1,"class":1}},
	"1002":{"id":1002,"dbname":"PORING","name":"Poring","stats":{"level":1,"health":60,"defense":2,"elementName":"Water 1",
		"scaleName":"Medium","raceName":"DemiHuman","mvp":0,"class":0}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &gamedata.Catalog{
		Monsters:     map[int]*gamedata.Monster{1002: {ID: 1002, Name: "Poring", HP: 55, Class: "Normal", DamageTaken: 100}},
		MonsterStubs: map[int]string{20994: "MD_BETELGEUSE"},
	}
	added, updated, err := MergeLatamCalc(c, path)
	if err != nil || added != 1 || updated != 1 {
		t.Fatalf("added %d updated %d err %v", added, updated, err)
	}
	b := c.Monsters[20994]
	if b == nil || b.HP != 2000000000 || !b.MVP || b.Class != "Boss" || b.DamageTaken != 1 || b.ElementLv != 2 || b.RES != 500 {
		t.Fatalf("Betelgeuse: %+v", b)
	}
	if _, stub := c.MonsterStubs[20994]; stub {
		t.Error("still a stub")
	}
	if p := c.Monsters[1002]; p.HP != 60 || p.Race != "Demihuman" || p.Name != "Poring" {
		t.Errorf("Poring: %+v", p)
	}
}
