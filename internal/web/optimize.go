package web

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// Optimizer, part 1: for every card slot (and every enchant position in use)
// of the equipped items, try each card that fits, one at a time, and rank them
// by the expected damage against the damage window's target with its attack.

const (
	optTop     = 5               // suggestions per position
	optTimeout = 8 * time.Second // the rest of the positions are skipped after this
)

type optOption struct {
	Item     itemView
	Value    string // expected damage
	Gain     string // "+18,4%"
	Slot     string
	Index    int
	Positive bool
}

type optGroup struct {
	Slot, Label string
	Index       int
	Enchant     bool
	Current     *itemView
	Options     []optOption
}

type optResult struct {
	NoTarget bool
	NoItems  bool
	Base     string
	Target   string
	Attack   string
	Groups   []optGroup
	Same     []string // positions where the current card is already the best
	NoEnch   []string // enchanted items the client's enchant list doesn't cover
	Tested   int
	Partial  bool // stopped by the time limit
}

// optPosition is one card position to optimize.
type optPosition struct {
	slots   []string // linked slots (one headgear covering several)
	label   string
	index   int
	enchant bool
	current int
	pool    []*gamedata.Item
}

// optCards lists the cards once (the catalog never changes).
func (s *Server) optCards() []*gamedata.Item {
	s.optOnce.Do(func() {
		for _, it := range s.opt.Catalog.Items {
			if it.Type == "Card" && it.SubType != "Enchant" && it.Name.Get("pt") != "" {
				s.optCardList = append(s.optCardList, it)
			}
		}
		sort.Slice(s.optCardList, func(i, j int) bool { return s.optCardList[i].ID < s.optCardList[j].ID })
	})
	return s.optCardList
}

// optExpected is the objective: expected damage per use of the attack. The
// build must already be normalized (Normalize is not safe to run in parallel
// on builds sharing maps).
func (s *Server) optExpected(b calc.Build) float64 {
	r := calc.Compute(b, s.opt.Catalog)
	return s.damageView(b, r, "pt").Dmg.Expected
}

func (s *Server) optimize(ctx context.Context, b calc.Build, enchants bool, l string) optResult {
	c := s.opt.Catalog
	calc.Normalize(&b, c)
	var res optResult
	m := c.Monsters[b.Target]
	if m == nil {
		res.NoTarget = true
		return res
	}
	res.Target = m.NameIn(l)
	res.Attack = tr(l, "Ataque básico")
	if sk := c.Skills[b.Atk.Skill]; sk != nil {
		res.Attack = sk.Name.Get(l) + " " + tr(l, "Nv.") + " " + strconv.Itoa(b.Atk.Level)
	}
	base := s.optExpected(b)
	res.Base = fmtInt(int64(base + 0.5))

	cards := s.optCards()
	var pos []optPosition
	done := map[string]bool{}
	for _, info := range append(append([]slotInfo{}, equipSlots...), shadowSlots...) {
		e, ok := b.Equip[info.Key]
		it := c.Items[e.Item]
		if !ok || it == nil || done[info.Key] {
			continue
		}
		linked := []string{info.Key}
		if strings.HasPrefix(info.Key, "Head_") {
			for _, h := range []string{calc.HeadTop, calc.HeadMid, calc.HeadLow} {
				if h != info.Key && b.Equip[h].Item == e.Item {
					linked = append(linked, h)
				}
			}
		}
		for _, k := range linked {
			done[k] = true
		}
		var pool []*gamedata.Item
		for _, cd := range cards {
			if calc.CardFits(cd, info.Key) {
				pool = append(pool, cd)
			}
		}
		// Enchants: only what the client's enchant list gives this item.
		ench := c.Enchants[it.ID]
		for i := 0; i < 4; i++ {
			p := optPosition{slots: linked, label: info.Label, index: i, current: e.Cards[i]}
			switch {
			case i < it.Slots:
				p.pool = pool
			case enchants && len(ench[i]) > 0:
				p.enchant = true
				for _, id := range ench[i] {
					if ei := c.Items[id]; ei != nil {
						p.pool = append(p.pool, ei)
					}
				}
			default:
				if enchants && ench == nil && e.Cards[i] != 0 {
					if name := it.Name.Get(l); !slices.Contains(res.NoEnch, name) {
						res.NoEnch = append(res.NoEnch, name)
					}
				}
				continue
			}
			pos = append(pos, p)
		}
	}
	if len(pos) == 0 {
		res.NoItems = true
		return res
	}

	// Evaluate every (position, card) in parallel.
	type job struct{ p, k int }
	vals := make([][]float64, len(pos))
	var jobs []job
	for pi, p := range pos {
		vals[pi] = make([]float64, len(p.pool))
		for k := range p.pool {
			vals[pi][k] = -1
			jobs = append(jobs, job{pi, k})
		}
	}
	ctx, cancel := context.WithTimeout(ctx, optTimeout)
	defer cancel()
	ch := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < max(1, runtime.NumCPU()-1); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				p := pos[j.p]
				nb := b
				nb.Equip = maps.Clone(b.Equip)
				for _, k := range p.slots {
					e := nb.Equip[k]
					e.Cards[p.index] = p.pool[j.k].ID
					nb.Equip[k] = e
				}
				vals[j.p][j.k] = s.optExpected(nb)
			}
		}()
	}
feed:
	for _, j := range jobs {
		select {
		case ch <- j:
		case <-ctx.Done():
			res.Partial = true
			break feed
		}
	}
	close(ch)
	wg.Wait()

	pct := func(v float64) string {
		if base <= 0 {
			return ""
		}
		g := (v/base - 1) * 100
		txt := strconv.FormatFloat(g, 'f', 1, 64)
		if l != "en" {
			txt = strings.Replace(txt, ".", ",", 1)
		}
		if g >= 0 {
			txt = "+" + txt
		}
		return txt + "%"
	}
	for pi, p := range pos {
		label := tr(l, p.label) + " · " + tr(l, "Carta") + " " + strconv.Itoa(p.index+1)
		if p.enchant {
			label = tr(l, p.label) + " · " + tr(l, "Encantamento") + " (slot " + strconv.Itoa(p.index+1) + ")"
		}
		g := optGroup{Slot: p.slots[0], Label: label, Index: p.index, Enchant: p.enchant}
		if ci := c.Items[p.current]; p.current != 0 && ci != nil {
			v := viewItem(ci, l)
			g.Current = &v
		}
		idx := make([]int, 0, len(p.pool))
		for k, v := range vals[pi] {
			if v >= 0 {
				res.Tested++
				if v > base*1.0005 && p.pool[k].ID != p.current {
					idx = append(idx, k)
				}
			}
		}
		sort.SliceStable(idx, func(a, z int) bool { return vals[pi][idx[a]] > vals[pi][idx[z]] })
		seen := map[string]bool{}
		for _, k := range idx {
			if len(g.Options) >= optTop {
				break
			}
			// Tiers of one card/enchant ("… 10", "… 9") show only the best.
			fam := optFamily(p.pool[k].Name.Get(l))
			if seen[fam] {
				continue
			}
			seen[fam] = true
			g.Options = append(g.Options, optOption{Item: viewItem(p.pool[k], l), Value: fmtInt(int64(vals[pi][k] + 0.5)),
				Gain: pct(vals[pi][k]), Slot: g.Slot, Index: p.index, Positive: true})
		}
		if len(g.Options) == 0 {
			res.Same = append(res.Same, label)
			continue
		}
		res.Groups = append(res.Groups, g)
	}
	// Biggest gains first.
	sort.SliceStable(res.Groups, func(a, z int) bool {
		return gainOf(res.Groups[a]) > gainOf(res.Groups[z])
	})
	return res
}

var optTier = regexp.MustCompile(`\s+(\+?\d+|[IVX]+|(?i:nv|lv)\.?\s*\d+)$`)

// optFamily strips a trailing tier ("Insígnia do Talento 10" -> "Insígnia do Talento").
func optFamily(name string) string {
	return strings.ToLower(optTier.ReplaceAllString(strings.TrimSpace(name), ""))
}

func gainOf(g optGroup) float64 {
	if len(g.Options) == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(strings.NewReplacer("+", "", "%", "", ",", ".").Replace(g.Options[0].Gain), 64)
	return f
}

// POST /build/optimize: JSON {b: build, enc: bool} -> the Optimizer window body.
func (s *Server) handleOptimize(w http.ResponseWriter, r *http.Request) {
	if s.opt.Catalog == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	var in struct {
		B   calc.Build `json:"b"`
		Enc bool       `json:"enc"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBuildBody+256)).Decode(&in); err != nil {
		http.Error(w, "invalid build", http.StatusBadRequest)
		return
	}
	l := lang(r)
	res := s.optimize(r.Context(), in.B, in.Enc, l)
	s.fragment(w, r, "build", "build_optimize", res)
}
