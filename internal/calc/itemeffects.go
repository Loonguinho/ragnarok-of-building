package calc

import (
	"strconv"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

// ItemEffectPrefix keys an item's triggered bonus in Build.Buffs
// ("proc:470071" = 1: Bota Desconhecida FOR's STR +175 counts).
const ItemEffectPrefix = "proc:"

// ItemEffect is a bonus an equipped item gives for a while (rAthena's
// autobonus: on attack, when hit, on a skill; latam-ro-calc's "chance"
// lines). The game rolls it; the build says whether to count it.
type ItemEffect struct {
	Item         int
	Event        bool   // "[Durante o Evento]" bonus
	Key          string // Build.Buffs key
	On           bool
	Bonus, Extra map[string]int
}

// ItemEffectKey is the Build.Buffs key of an item's effect.
func ItemEffectKey(item int) string { return ItemEffectPrefix + strconv.Itoa(item) }

// EventEffectKey is the Build.Buffs key of an item's event bonus.
func EventEffectKey(item int) string { return ItemEffectPrefix + "ev" + strconv.Itoa(item) }

type itemEffects struct {
	b    *Build
	c    *gamedata.Catalog
	env  *latamEnv
	seen map[int]bool
	list []ItemEffect
}

// add records the effect of an equipped item or card: its rAthena autobonus
// scripts reached for this refine and grade, or else its latam-ro-calc
// chance lines. One effect per item, however many copies are worn.
func (x *itemEffects) add(id int, it *gamedata.Item, ctx script.Context, autobonus []string, slot string, main bool, refine int) {
	if it.EventScript != "" && !x.seen[-id] {
		x.seen[-id] = true
		r := script.Eval(it.EventScript, ctx)
		if len(r.Bonus) > 0 || len(r.Extra) > 0 {
			key := EventEffectKey(id)
			x.list = append(x.list, ItemEffect{Item: id, Event: true, Key: key, On: x.b.Buffs[key] > 0, Bonus: r.Bonus, Extra: r.Extra})
		}
	}
	if x.seen[id] || (len(autobonus) == 0 && len(it.LatamProcs) == 0) {
		return
	}
	x.seen[id] = true
	ef := ItemEffect{Item: id, Key: ItemEffectKey(id), Bonus: map[string]int{}, Extra: map[string]int{}}
	res := script.Result{Bonus: ef.Bonus, Extra: ef.Extra}
	if len(autobonus) > 0 {
		for _, src := range autobonus {
			r := script.Eval(src, ctx)
			addAll(ef.Bonus, r.Bonus)
			addAll(ef.Extra, r.Extra)
		}
	} else {
		addLatam(x.c, x.env.evalScript(it.LatamProcs, id, latamPosOf[slot], main, refine, true), &res)
	}
	if len(ef.Bonus) == 0 && len(ef.Extra) == 0 {
		return // nothing this build can get (refine too low, set missing...)
	}
	ef.On = x.b.Buffs[ef.Key] > 0
	x.list = append(x.list, ef)
}
