package gamedata

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	if got := Normalize("  Poção   Vermelha "); got != "pocao vermelha" {
		t.Errorf("got %q", got)
	}
}

func TestSearch(t *testing.T) {
	c := &Catalog{Items: map[int]*Item{
		501:  {ID: 501, Name: Text{"pt": "Poção Vermelha", "en": "Red Potion"}},
		4001: {ID: 4001, Name: Text{"pt": "Carta Poring", "en": "Poring Card"}},
		4002: {ID: 4002, Name: Text{"pt": "Carta Fabre", "en": "Fabre Card"}},
		9001: {ID: 9001, Name: Text{"pt": "Ovo de Poring", "en": "Poring Egg"}},
		9999: {ID: 9999, Name: Text{}}, // unnamed: not indexed
	}}
	x := NewItemIndex(c)
	if x.Len() != 4 {
		t.Fatalf("indexed %d, want 4", x.Len())
	}
	ids := func(items []*Item) (out []int) {
		for _, it := range items {
			out = append(out, it.ID)
		}
		return
	}
	cases := []struct {
		q    string
		want []int
	}{
		{"pocao", []int{501}},         // accent-insensitive
		{"red potion", []int{501}},    // any language
		{"4001", []int{4001}},         // by ID
		{"poring", []int{9001, 4001}}, // both prefix-match in EN; "poring egg" is shorter
		{"carta", []int{4002, 4001}},  // tie on rank: shorter name first ("carta fabre")
		{"ovo", []int{9001}},          // prefix in PT
		{"card fabre", []int{4002}},   // all terms, any order
		{"zzz", nil},
	}
	for _, c := range cases {
		got := ids(x.Search(c.q, 10))
		if len(got) != len(c.want) {
			t.Errorf("%q: got %v, want %v", c.q, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: got %v, want %v", c.q, got, c.want)
				break
			}
		}
	}
}

func TestSearchPlural(t *testing.T) {
	for q, name := range map[string]string{
		"capas desconhecidas": "capa desconhecida for",
		"aneis":               "anel do lobo",
		"botoes":              "botao de rosa",
		"colares":             "colar do lobo",
		"bonus":               "bonus de ataque",
	} {
		if !containsAll(name, strings.Fields(q)) {
			t.Errorf("%q should match %q", q, name)
		}
	}
	if containsAll("capa velha", strings.Fields("capas desconhecidas")) {
		t.Error("plural matched a name missing the other word")
	}
}
