package view

import (
	"context"
	"maps"
	"runtime"
	"sort"
	"sync"

	"ragnarok-of-building/internal/calc"
)

// Best card for a card or enchant position: every card that fits (the
// client's enchant list for enchant positions), ranked by the goal —
// expected damage against the target (GoalDamage) or effective HP against
// it (GoalPhysDef, GoalMagDef) — like the optimizer.

// BestCards ranks the cards for position idx of the item in slot.
func (v *Views) BestCards(ctx context.Context, b calc.Build, slot string, idx int, goal string, l string) BestResult {
	s := v.s
	c := s.opt.Catalog
	var res BestResult
	if c.Monster(b.Target) == nil {
		res.NoTarget = true
		return res
	}
	b.Equip, b.Skills, b.Buffs = maps.Clone(b.Equip), maps.Clone(b.Skills), maps.Clone(b.Buffs)
	b.Atk.States = maps.Clone(b.Atk.States)
	if b.Equip == nil {
		b.Equip = map[string]calc.Equip{}
	}
	calc.Normalize(&b, c)
	host := c.Items[b.Equip[slot].Item]
	if host == nil || idx < 0 || idx > 3 {
		return res
	}
	// Against the build as it is, like Melhor item (the current card reads "=").
	base := s.optScore(b, goal)
	if base <= 0 {
		res.NoTarget = true
		return res
	}
	var ids []int
	if ench, ok := c.Enchants[host.ID]; ok && idx >= host.Slots {
		ids = append(ids, ench[idx]...)
	} else {
		for _, it := range c.Items {
			if it.Type == "Card" && calc.CardFits(it, slot) && it.Name.Get(l) != "" &&
				(idx < host.Slots) == (it.SubType != "Enchant") {
				ids = append(ids, it.ID)
			}
		}
	}
	sort.Ints(ids)
	ctx, cancel := context.WithTimeout(ctx, bestTimeout)
	defer cancel()
	type scored struct {
		id    int
		ratio float64
	}
	var (
		mu  sync.Mutex
		out []scored
		wg  sync.WaitGroup
	)
	ch := make(chan int)
	for range max(1, runtime.NumCPU()-1) {
		wg.Go(func() {
			for id := range ch {
				nb := s.swapItem(b, slot, id, idx)
				if nb.Equip[slot].Cards[idx] != id { // Normalize dropped it
					continue
				}
				e := s.optScore(nb, goal)
				mu.Lock()
				out = append(out, scored{id, e / base})
				mu.Unlock()
			}
		})
	}
feed:
	for _, id := range ids {
		select {
		case ch <- id:
		case <-ctx.Done():
			res.Partial = true
			break feed
		}
	}
	close(ch)
	wg.Wait()
	res.Tested = len(out)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ratio != out[j].ratio {
			return out[i].ratio > out[j].ratio
		}
		return out[i].id < out[j].id
	})
	for _, sc := range out[:min(len(out), bestTop)] {
		ln := StatLine{Diff: pctChange(sc.ratio, l), Better: sc.ratio > 1.0005, Same: sc.ratio > 0.9995 && sc.ratio < 1.0005}
		if ln.Same {
			ln.Diff = "="
		}
		res.Items = append(res.Items, BestItem{Item: viewItem(c.Items[sc.id], l), Gain: ln, Ratio: sc.ratio})
	}
	return res
}
