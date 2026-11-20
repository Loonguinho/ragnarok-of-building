package view

import (
	"slices"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

const searchLimit = 60

// itemView is what the item templates render.
type itemView struct {
	ID    int
	Name  string
	Slots int
	Desc  []string
	Lang  string
	Loc   string // comma-separated equip locations (for client-side slot rules)
	Type  string
	MVP   bool // a card only MVPs drop
}

func viewItem(it *gamedata.Item, l string) itemView {
	return itemView{ID: it.ID, Name: it.Name.Get(l), Slots: it.Slots, Desc: it.Desc.Get(l), Lang: l,
		Loc: strings.Join(it.Locations, ","), Type: it.Type, MVP: it.MVPCard}
}

// itemTypes are the item database filters (value, label).
var itemTypes = [][2]string{
	{"", "Todos"}, {"weapon", "Armas"}, {"armor", "Armaduras e acessórios"}, {"head", "Chapéus"},
	{"costume", "Visuais"}, {"card", "Cartas"}, {"shadow", "Equip. sombrios"}, {"other", "Outros"},
}

func matchType(it *gamedata.Item, typ string) bool {
	isHead := it.Type == "Armor" && (it.HasLocation("Head_Top") || it.HasLocation("Head_Mid") || it.HasLocation("Head_Low"))
	isCostume := it.Type == "Armor" && !isHead && (it.HasLocation("Costume_Head_Top") || it.HasLocation("Costume_Head_Mid") ||
		it.HasLocation("Costume_Head_Low") || it.HasLocation("Costume_Garment"))
	switch typ {
	case "":
		return true
	case "weapon":
		return it.Type == "Weapon"
	case "armor":
		return it.Type == "Armor" && !isHead && !isCostume
	case "costume":
		return isCostume
	case "head":
		return isHead
	case "card":
		return it.Type == "Card"
	case "shadow":
		return it.Type == "ShadowGear"
	case "other":
		return it.Type == ""
	}
	return false
}

// searchItems searches by name/ID within a type. With no query, a type
// filter lists that type by ID; with neither, nothing is listed.
func (s *Server) searchItems(q, typ, l string) []itemView { return s.searchItemsN(q, typ, l, searchLimit) }

// searchItemsN is searchItems with its own result limit (the item bank
// lists a whole type to scroll through).
func (s *Server) searchItemsN(q, typ, l string, limit int) []itemView {
	var pool []*gamedata.Item
	switch {
	case q != "":
		pool = s.items.Search(q, len(s.opt.Catalog.Items))
	case typ != "":
		ids := make([]int, 0, len(s.opt.Catalog.Items))
		for id, it := range s.opt.Catalog.Items {
			if matchType(it, typ) && it.Name.Get(l) != "" {
				ids = append(ids, id)
			}
		}
		slices.Sort(ids)
		for _, id := range ids[:min(limit, len(ids))] {
			pool = append(pool, s.opt.Catalog.Items[id])
		}
	}
	var out []itemView
	for _, it := range pool {
		if matchType(it, typ) {
			out = append(out, viewItem(it, l))
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}
