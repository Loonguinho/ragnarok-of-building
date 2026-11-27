package calc

import "maps"

// The cast and delay reductions keep where each part comes from, so the
// interface can list them (the attributes window's Conj. var. tip), as
// latam-tools does.

// sourceKeys are the bonuses whose sources are kept.
var sourceKeys = []string{"bVariableCastrate", "bCastrate", "bFixedCastrate", "bFixedCast", "bDelayrate"}

// Source is one part of a kept bonus: Kind says what gave it ("item",
// "card", "opt" (random option), "combo", "effect" (an item's triggered
// effect), "pet", or a group: "buffs", "classbuffs", "consumables",
// "passives"); Items are the items behind it (several for a combo).
type Source struct {
	Key   string
	Kind  string
	Items []int
	Value int
}

// noteSources keeps the kept bonuses of one script's result.
func (r *Result) noteSources(kind string, b map[string]int, items ...int) {
	for _, k := range sourceKeys {
		if v := b[k]; v != 0 {
			r.Sources = append(r.Sources, Source{Key: k, Kind: kind, Items: items, Value: v})
		}
	}
}

// keptBefore copies the kept bonuses, to tell what a step added.
func keptBefore(b map[string]int) map[string]int {
	out := map[string]int{}
	for _, k := range sourceKeys {
		out[k] = b[k]
	}
	return out
}

// noteAdded keeps what a step (the buffs, the passives) added to the kept
// bonuses since before.
func (r *Result) noteAdded(kind string, before, b map[string]int) {
	diff := maps.Clone(before)
	for _, k := range sourceKeys {
		diff[k] = b[k] - before[k]
	}
	r.noteSources(kind, diff)
}
