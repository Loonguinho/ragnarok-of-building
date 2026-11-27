package view

import (
	"os"
	"slices"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestItemDescsArriveLater: the web page opens with the catalog's item
// descriptions taken out and puts them back per language later; every
// item must then read as with the whole catalog, set texts included.
func TestItemDescsArriveLater(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	whole, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	testViews(t, whole)
	split, _ := gamedata.Load(path)
	descs := gamedata.TakeItemDescs(split)
	v := testViews(t, split)
	for _, l := range gamedata.Langs {
		v.PutItemDescs(l, descs[l])
	}
	bad := 0
	for id, it := range whole.Items {
		for _, l := range gamedata.Langs {
			want, got := it.Desc.Get(l), split.Items[id].Desc.Get(l)
			if slices.Equal(got, want) || blank(got) && blank(want) {
				continue // a blank text comes as [] or [""], by map order
			}
			// The whole catalog loses the fallback text where the client
			// has none in l and the sets are added: the split one keeps it.
			if len(want) > 0 && want[0] == setSep && len(got) > len(want) && slices.Equal(got[len(got)-len(want):], want) {
				continue
			}
			if bad++; bad <= 5 {
				t.Errorf("item %d %s:\n got %q\nwant %q", id, l, got, want)
			}
		}
	}
}

func blank(lines []string) bool {
	return !slices.ContainsFunc(lines, func(s string) bool { return s != "" })
}
