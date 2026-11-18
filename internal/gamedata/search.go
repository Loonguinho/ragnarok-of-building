package gamedata

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// ItemIndex is an in-memory, accent-insensitive search over item names in
// every language. 18k items scan in well under a millisecond per query.
type ItemIndex struct {
	entries []indexEntry
}

type indexEntry struct {
	item  *Item
	names []string // normalized names, one per language
}

// NewItemIndex builds the index. Items without any name are skipped.
func NewItemIndex(c *Catalog) *ItemIndex {
	idx := &ItemIndex{}
	for _, it := range c.Items {
		e := indexEntry{item: it}
		for _, l := range Langs {
			if n := Normalize(it.Name[l]); n != "" {
				e.names = append(e.names, n)
			}
		}
		if len(e.names) > 0 {
			idx.entries = append(idx.entries, e)
		}
	}
	sort.Slice(idx.entries, func(i, j int) bool { return idx.entries[i].item.ID < idx.entries[j].item.ID })
	return idx
}

// Len returns the number of searchable items.
func (x *ItemIndex) Len() int { return len(x.entries) }

// Search returns up to limit items matching q by ID or by name in any language.
// Ranking: exact ID, then names starting with q, then names containing q;
// ties by shorter name, then ID.
func (x *ItemIndex) Search(q string, limit int) []*Item {
	q = Normalize(q)
	if q == "" || limit <= 0 {
		return nil
	}
	id, idErr := strconv.Atoi(q)
	terms := strings.Fields(q)

	type hit struct {
		item *Item
		rank int
		size int
	}
	var hits []hit
	for _, e := range x.entries {
		if idErr == nil && e.item.ID == id {
			hits = append(hits, hit{e.item, 0, 0})
			continue
		}
		best := -1
		size := 0
		for _, n := range e.names {
			if !containsAll(n, terms) {
				continue
			}
			r := 2
			if strings.HasPrefix(n, q) {
				r = 1
			}
			if best == -1 || r < best || (r == best && len(n) < size) {
				best, size = r, len(n)
			}
		}
		if best >= 0 {
			hits = append(hits, hit{e.item, best, size})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.size != b.size {
			return a.size < b.size
		}
		return a.item.ID < b.item.ID
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]*Item, len(hits))
	for i, h := range hits {
		out[i] = h.item
	}
	return out
}

func containsAll(s string, terms []string) bool {
	for _, t := range terms {
		if !strings.Contains(s, t) && !containsSingular(s, t) {
			return false
		}
	}
	return true
}

// containsSingular lets a plural search word match a singular name
// ("capas desconhecidas" finds "Capa Desconhecida", "aneis" finds "anel",
// "botoes" finds "botao"); words are already lowercased without accents.
func containsSingular(s, t string) bool {
	if len(t) < 4 || !strings.HasSuffix(t, "s") {
		return false
	}
	forms := []string{t[:len(t)-1]}
	switch {
	case strings.HasSuffix(t, "oes"), strings.HasSuffix(t, "aes"):
		forms = append(forms, t[:len(t)-3]+"ao")
	case strings.HasSuffix(t, "eis"):
		forms = append(forms, t[:len(t)-3]+"el")
	case strings.HasSuffix(t, "ais"):
		forms = append(forms, t[:len(t)-3]+"al")
	case strings.HasSuffix(t, "res"), strings.HasSuffix(t, "zes"), strings.HasSuffix(t, "les"):
		forms = append(forms, t[:len(t)-2])
	}
	for _, f := range forms {
		if strings.Contains(s, f) {
			return true
		}
	}
	return false
}

// Normalize lowercases, strips accents and collapses spaces: "Poção  Vermelha" -> "pocao vermelha".
func Normalize(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.Join(strings.Fields(strings.ToLower(out)), " ")
}
