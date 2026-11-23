package view

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"testing"
	"time"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// needStress skips the slow tests unless asked for: STRESS=20 go test
// ./internal/view -run Stress (minutes; the bigger, the more builds).
func needStress(t *testing.T) {
	if os.Getenv("STRESS") == "" {
		t.Skip("set STRESS=n to run")
	}
}

// stressCatalog loads the generated catalog for the comparison tests.
func stressCatalog(t *testing.T) (*gamedata.Catalog, *Views) {
	t.Helper()
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := New(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	return c, v
}

// traitHosts are the items with positions that take trait enchants, by slot.
func traitHosts(c *gamedata.Catalog) map[string][]int {
	out := map[string][]int{}
	for id, pos := range c.Enchants {
		it := c.Items[id]
		if it == nil {
			continue
		}
		takes := false
		for _, list := range pos {
			for _, e := range list {
				takes = takes || traitEnchant(c.Items[e])
			}
		}
		if !takes {
			continue
		}
		for _, sl := range calc.Slots {
			if calc.Fits(it, sl) {
				out[sl] = append(out[sl], id)
			}
		}
	}
	for _, l := range out {
		sort.Ints(l)
	}
	return out
}

// randomBuild is a 4th class with gear it can wear, cards and enchants.
func randomBuild(c *gamedata.Catalog, rng *rand.Rand, job int, hosts map[string][]int, weapons map[string][]int) calc.Build {
	j := c.Jobs[job]
	b := calc.Build{Job: job, BaseLv: 200 + rng.Intn(51), JobLv: 1 + rng.Intn(50), Equip: map[string]calc.Equip{}, Skills: map[int]int{}}
	for i := range b.Stats {
		b.Stats[i] = 1 + rng.Intn(130)
	}
	budget := c.TraitPoints[b.BaseLv-1] + 7
	if rng.Intn(4) > 0 {
		for left := budget; left > 0; {
			i, n := rng.Intn(6), 1+rng.Intn(60)
			n = min(n, left, 100-b.Traits[i])
			if n <= 0 {
				if b.Traits == [6]int{100, 100, 100, 100, 100, 100} {
					break
				}
				continue
			}
			b.Traits[i] += n
			left -= n
			if rng.Intn(8) == 0 {
				break
			}
		}
	}
	for _, id := range c.Chain(job) {
		for _, sk := range c.Jobs[id].Tree {
			if s := c.Skills[sk]; s != nil && rng.Intn(3) > 0 {
				b.Skills[sk] = 1 + rng.Intn(s.MaxLv)
			}
		}
	}
	var ws []int
	for _, l := range weapons {
		for _, id := range l {
			if j.CanEquip(c.Items[id]) {
				ws = append(ws, id)
			}
		}
	}
	sort.Ints(ws)
	if len(ws) > 0 {
		b.Equip[calc.Weapon] = calc.Equip{Item: ws[rng.Intn(len(ws))], Refine: rng.Intn(16), Grade: rng.Intn(5)}
	}
	for _, sl := range calc.Slots {
		if l := hosts[sl]; len(l) > 0 && rng.Intn(3) > 0 {
			id := l[rng.Intn(len(l))]
			if !j.CanEquip(c.Items[id]) {
				continue
			}
			e := calc.Equip{Item: id, Refine: rng.Intn(16), Grade: rng.Intn(5)}
			for idx := c.Items[id].Slots; idx < 4; idx++ {
				if opts := c.Enchants[id][idx]; len(opts) > 0 && rng.Intn(4) > 0 {
					e.Cards[idx] = opts[rng.Intn(len(opts))]
				}
			}
			b.Equip[sl] = e
		}
	}
	mobs := []int{21067, 21542, 21539, 1039, 2408, 21573}
	b.Target = mobs[rng.Intn(len(mobs))]
	calc.Normalize(&b, c)
	var act []int
	for id := range b.Skills {
		if sk := c.Skills[id]; !sk.Passive && !sk.Free() {
			act = append(act, id)
		}
	}
	sort.Ints(act)
	if len(act) > 0 && rng.Intn(5) > 0 {
		b.Atk.Skill = act[rng.Intn(len(act))]
		b.Atk.Level = b.Skills[b.Atk.Skill]
	}
	calc.Normalize(&b, c)
	return b
}

func jsonOf(x any) string { b, _ := json.Marshal(x); return string(b) }

// checkTraits is what every trait comparison must satisfy.
func checkTraits(v *Views, b calc.Build, o TraitOptions) (TraitView, []string) {
	var errs []string
	bad := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	c := v.s.opt.Catalog
	before := jsonOf(b)
	r := v.CompareTraits(context.Background(), b, o, "pt")
	if jsonOf(b) != before {
		bad("the caller's build changed")
	}
	if r.NotFourth || r.NoTarget || r.NoDamage {
		if len(r.Cols) > 0 {
			bad("columns with a reason not to compare")
		}
		return r, errs
	}
	nb := cloneBuild(b)
	calc.Normalize(&nb, c)
	base := v.s.optScore(nb, o.Goal)
	if len(r.Cols) < 2 {
		bad("only %d columns", len(r.Cols))
		return r, errs
	}
	if r.Cols[0].Traits != nb.Traits || math.Abs(r.Cols[0].Ratio-1) > 1e-12 {
		bad("current column %v ratio %v", r.Cols[0].Traits, r.Cols[0].Ratio)
	}
	best := r.Cols[len(r.Cols)-1]
	for i, cl := range r.Cols {
		sum := 0
		for _, x := range cl.Traits {
			sum += x
			if x < 0 || x > 100 {
				bad("%s: trait %d", cl.Title, x)
			}
		}
		if i > 0 && sum != min(r.Budget, 600) {
			bad("%s spends %d of %d", cl.Title, sum, r.Budget)
		}
		if math.IsNaN(cl.Ratio) || math.IsInf(cl.Ratio, 0) {
			bad("%s ratio %v", cl.Title, cl.Ratio)
		}
		if i > 0 && cl.Ratio > best.Ratio*(1+1e-9) && !r.Partial {
			bad("%s (%v) beats the best found (%v)", cl.Title, cl.Ratio, best.Ratio)
		}
		// Applying the column gives what it says.
		ap := v.ApplyTraits(b, cl.Traits, cl.Enchants)
		if i > 0 || r.Used <= r.Budget {
			if got := v.s.optScore(ap, o.Goal) / base; math.Abs(got-cl.Ratio) > 1e-9*math.Max(1, cl.Ratio) {
				bad("%s: says %v, applied %v", cl.Title, cl.Ratio, got)
			}
		}
		if ap.Traits != cl.Traits {
			bad("%s: applied traits %v, want %v", cl.Title, ap.Traits, cl.Traits)
		}
		for _, e := range cl.Enchants {
			if e.Card != 0 && ap.Equip[e.Slot].Cards[e.Idx] != e.Card {
				bad("%s: %s/%d applied %d, want %d", cl.Title, e.Slot, e.Idx, ap.Equip[e.Slot].Cards[e.Idx], e.Card)
			}
			cur := nb.Equip[e.Slot].Cards[e.Idx]
			if e.Changed != (e.Card != cur) {
				bad("%s: %s/%d changed=%v but %d -> %d", cl.Title, e.Slot, e.Idx, e.Changed, cur, e.Card)
			}
			if e.Changed && !slices.Contains(c.Enchants[nb.Equip[e.Slot].Item][e.Idx], e.Card) {
				bad("%s: %s/%d does not take %s", cl.Title, e.Slot, e.Idx, e.Name)
			}
			if e.Changed && !o.AnyLevel && enchLevel(c.Items[e.Card]) != enchLevel(c.Items[cur]) {
				bad("%s: %s is another level", cl.Title, e.Name)
			}
		}
		// Nothing else moved.
		for sl, e := range ap.Equip {
			if e.Item != nb.Equip[sl].Item || e.Refine != nb.Equip[sl].Refine || e.Grade != nb.Equip[sl].Grade {
				bad("%s: %s became %+v", cl.Title, sl, e)
			}
		}
		if ap.Stats != nb.Stats || jsonOf(ap.Skills) != jsonOf(nb.Skills) || jsonOf(ap.Atk) != jsonOf(nb.Atk) {
			bad("%s: applying changed stats, skills or the attack", cl.Title)
		}
	}
	// The table: every value is the column's own, the diffs against column 0.
	for _, row := range r.Rows {
		if len(row.Values) != len(r.Cols) || len(row.Diffs) != len(r.Cols) || len(row.Better) != len(r.Cols) {
			bad("row %s has %d/%d/%d cells for %d columns", row.Label, len(row.Values), len(row.Diffs), len(row.Better), len(r.Cols))
			continue
		}
		for i := range r.Cols {
			if (row.Diffs[i] == "") != (row.Better[i] == 0) {
				bad("row %s col %d: diff %q but better %d", row.Label, i, row.Diffs[i], row.Better[i])
			}
		}
		if row.Diffs[0] != "" {
			bad("row %s: the current column has a diff", row.Label)
		}
	}
	if len(r.Rows) > 0 {
		for i, cl := range r.Cols {
			if want := fmtInt(int64(v.s.optScore(cl.Build, o.Goal) + 0.5)); r.Rows[0].Values[i] != want {
				bad("%s: the table says %s, its build gives %s", cl.Title, r.Rows[0].Values[i], want)
			}
		}
	}
	return r, errs
}

func stressN(def int) int {
	if n, err := strconv.Atoi(os.Getenv("STRESS")); err == nil {
		return n
	}
	return def
}

func stressBuilds(t *testing.T, c *gamedata.Catalog, v *Views, n int, seed int64) map[string]calc.Build {
	out := map[string]calc.Build{}
	rng := rand.New(rand.NewSource(seed))
	hosts := traitHosts(c)
	weapons := map[string][]int{}
	for id, it := range c.Items {
		if it.Type == "Weapon" && it.WeaponLv >= 4 && it.Attack > 0 && it.Script != "" && calc.Fits(it, calc.Weapon) {
			weapons[it.SubType] = append(weapons[it.SubType], id)
		}
	}
	var jobs []int
	for id, j := range c.Jobs {
		if j.Fourth && len(j.Tree) > 0 {
			jobs = append(jobs, id)
		}
	}
	sort.Ints(jobs)
	for i := 0; i < n; i++ {
		job := jobs[i%len(jobs)]
		out[fmt.Sprintf("%s#%d", c.Jobs[job].Code, i)] = randomBuild(c, rng, job, hosts, weapons)
	}
	if os.Getenv("STRESS") != "" {
		for _, g := range []string{"../../tmp/replays/mine/*.rrf", "../../tmp/replays/*.rrf", "../../tmp/latam-ro-calc/src/app/replay/__tests__/fixtures/*.rrf"} {
			fs, _ := filepath.Glob(g)
			for _, f := range fs {
				buf, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				b, _, _, err := v.ImportReplay(buf, "pt")
				if err != nil || c.Jobs[b.Job] == nil || !c.Jobs[b.Job].Fourth {
					continue
				}
				out["replay "+filepath.Base(f)] = b
			}
		}
	}
	return out
}

// TestStressTraits: the comparison's properties on random builds of every
// 4th class (STRESS=n for more, with the recordings of tmp).
func TestStressTraits(t *testing.T) {
	needStress(t)
	c, v := stressCatalog(t)
	builds := stressBuilds(t, c, v, stressN(19), 1)
	names := make([]string, 0, len(builds))
	for k := range builds {
		names = append(names, k)
	}
	sort.Strings(names)
	var slowest time.Duration
	compared, none := 0, map[string]int{}
	for i, k := range names {
		b := builds[k]
		goals := []string{GoalDamage, GoalPhysDef, GoalMagDef}
		for gi, g := range goals {
			if os.Getenv("STRESS") == "" && gi != i%3 {
				continue
			}
			o := TraitOptions{Goal: g, AnyLevel: (i+gi)%2 == 0}
			t0 := time.Now()
			r, errs := checkTraits(v, b, o)
			slowest = max(slowest, time.Since(t0))
			switch {
			case r.NotFourth:
				none["not fourth"]++
			case r.NoTarget:
				none["no target"]++
			case r.NoDamage:
				none["no damage"]++
			default:
				compared++
				// Asking again gives the same answer.
				r2 := v.CompareTraits(context.Background(), b, o, "pt")
				if !r.Partial && !r2.Partial {
					for ci := range r.Cols {
						if ci >= len(r2.Cols) || math.Abs(r.Cols[ci].Ratio-r2.Cols[ci].Ratio) > 1e-9 || r.Cols[ci].Traits != r2.Cols[ci].Traits || jsonOf(r.Cols[ci].Enchants) != jsonOf(r2.Cols[ci].Enchants) {
							errs = append(errs, fmt.Sprintf("column %d is another the second time: %v %v", ci, r.Cols[ci].Traits, r2.Cols[min(ci, len(r2.Cols)-1)].Traits))
							break
						}
					}
				}
				// On the best build found, nothing much better is found.
				best := r.Cols[len(r.Cols)-1]
				ap := v.ApplyTraits(b, best.Traits, best.Enchants)
				r3 := v.CompareTraits(context.Background(), ap, o, "pt")
				if n := len(r3.Cols); n > 0 && !r.Partial && !r3.Partial && r3.Cols[n-1].Ratio > 1.002 {
					errs = append(errs, fmt.Sprintf("after applying the best, %.2f%% more is found: %v -> %v", (r3.Cols[n-1].Ratio-1)*100, best.Traits, r3.Cols[n-1].Traits))
				}
			}
			for _, e := range errs {
				t.Errorf("%s [%s any=%v skill %d]: %s", k, g, o.AnyLevel, b.Atk.Skill, e)
			}
		}
	}
	t.Logf("%d comparisons, not compared %v, slowest %v", compared, none, slowest.Round(time.Millisecond))
}

// TestStressTraitsOptimum: the best found is not worse than a grid of
// distributions (each with the search's own choice of enchants).
func TestStressTraitsOptimum(t *testing.T) {
	needStress(t)
	c, v := stressCatalog(t)
	n := 3
	if os.Getenv("STRESS") != "" {
		n = 38
	}
	builds := stressBuilds(t, c, v, n, 7)
	worst, checked := 0.0, 0
	for k, b := range builds {
		o := TraitOptions{Goal: GoalDamage}
		r := v.CompareTraits(context.Background(), b, o, "pt")
		if len(r.Cols) == 0 || r.Partial {
			continue
		}
		nb := cloneBuild(b)
		calc.Normalize(&nb, c)
		base := v.s.optScore(nb, GoalDamage)
		ts := &traitSearch{s: v.s, ctx: context.Background(), b: nb, goal: GoalDamage, pos: v.s.traitPositions(nb, false), max: 100, seen: map[[6]int]traitScore{}}
		var list [][6]int
		for a := 0; a <= 100; a += 10 {
			for co := 0; co <= 100; co += 10 {
				for cr := 0; cr <= 100; cr += 10 {
					for sp := 0; sp <= 100; sp += 20 {
						left := r.Budget - a - co - cr - sp
						if left < 0 || left > 200 {
							continue
						}
						list = append(list, [6]int{a, min(left, 100), max(0, left-100), sp, co, cr})
					}
				}
			}
		}
		ranked := ts.evalAll(list)
		if len(ranked) == 0 {
			continue
		}
		checked++
		grid := ts.seen[ranked[0]].score / base
		best := r.Cols[len(r.Cols)-1].Ratio
		gap := grid/best - 1
		worst = max(worst, gap)
		if gap > 0.002 {
			t.Errorf("%s skill %d: the grid finds %v at %v, the search %v at %v", k, b.Atk.Skill, grid, ranked[0], best, r.Cols[len(r.Cols)-1].Traits)
		}
	}
	t.Logf("%d builds, worst gap %.3f%%", checked, worst*100)
}

// TestStressBestCards: every Melhor carta row (alone, with its set, with the
// traits changed) gives, applied, the gain it shows.
func TestStressBestCards(t *testing.T) {
	needStress(t)
	c, v := stressCatalog(t)
	builds := stressBuilds(t, c, v, stressN(6), 3)
	names := make([]string, 0, len(builds))
	for k := range builds {
		names = append(names, k)
	}
	sort.Strings(names)
	rows, traits, sets := 0, 0, 0
	for i, k := range names {
		if i%3 != 0 && os.Getenv("STRESS") != "" {
			continue
		}
		b := builds[k]
		base := v.s.optScore(b, GoalDamage)
		if base <= 0 {
			continue
		}
		for _, sl := range calc.Slots {
			it := c.Items[b.Equip[sl].Item]
			if it == nil || sl == calc.Ammo {
				continue
			}
			for idx := 0; idx < 4; idx++ {
				if idx >= it.Slots && len(c.Enchants[it.ID][idx]) == 0 {
					continue
				}
				if os.Getenv("STRESS") == "" && (idx+len(sl))%3 != 0 {
					continue
				}
				before := jsonOf(b)
				res := v.BestCardsTraits(context.Background(), b, sl, idx, GoalDamage, true, "pt")
				if jsonOf(b) != before {
					t.Fatalf("%s: Melhor carta changed the build", k)
				}
				for ri, bi := range res.Items {
					rows++
					var ap calc.Build
					switch {
					case bi.Traits != nil:
						traits++
						ap = v.Swap(b, sl, bi.Item.ID, idx)
						ap.Traits = *bi.Traits
						sum := 0
						for _, x := range ap.Traits {
							sum += x
						}
						if want := traitBudget(b, calc.Compute(b, c)); sum != want {
							t.Errorf("%s %s/%d: traits row spends %d of %d", k, sl, idx, sum, want)
						}
					case len(bi.Combo) > 0:
						sets++
						ap = v.ApplyCombo(b, sl, idx, bi.Item.ID, bi.Combo)
					default:
						ap = v.Swap(b, sl, bi.Item.ID, idx)
					}
					calc.Normalize(&ap, c)
					if got := v.s.optScore(ap, GoalDamage) / base; math.Abs(got-bi.Ratio) > 1e-9*math.Max(1, got) {
						t.Errorf("%s %s/%d %s (traits %v, set %v): says %v, applied %v", k, sl, idx, bi.Item.Name, bi.Traits != nil, len(bi.Combo), bi.Ratio, got)
					}
					if ri > 0 && bi.Ratio > res.Items[ri-1].Ratio+1e-12 {
						t.Errorf("%s %s/%d: row %d out of order", k, sl, idx, ri)
					}
				}
			}
		}
	}
	t.Logf("%d rows, %d with traits, %d sets", rows, traits, sets)
}

// TestStressCompareItems: in the item comparison, column 0 is the build as
// it is and equipping a column gives the damage it shows.
func TestStressCompareItems(t *testing.T) {
	needStress(t)
	c, v := stressCatalog(t)
	builds := stressBuilds(t, c, v, stressN(19), 5)
	rng := rand.New(rand.NewSource(11))
	var ids []int
	for id, it := range c.Items {
		if it.Type == "Armor" || it.Type == "Weapon" {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	n := 0
	for k, b := range builds {
		for _, sl := range []string{calc.Weapon, calc.Armor, calc.Garment, calc.Shoes, calc.AccLeft, calc.HeadTop} {
			cands := []CompareCand{{}}
			for try := 0; try < 400 && len(cands) < 4; try++ {
				id := ids[rng.Intn(len(ids))]
				if calc.Fits(c.Items[id], sl) && c.Jobs[b.Job].CanEquip(c.Items[id]) {
					cands = append(cands, CompareCand{Item: id, Refine: rng.Intn(13), Grade: rng.Intn(5)})
				}
			}
			before := jsonOf(b)
			r := v.Compare(b, sl, cands, "pt")
			if jsonOf(b) != before {
				t.Fatalf("%s: the comparison changed the build", k)
			}
			if len(r.Cols) != len(cands) || len(r.Rows) == 0 || !r.Rows[0].Main {
				t.Errorf("%s %s: %d columns, rows %d", k, sl, len(r.Cols), len(r.Rows))
				continue
			}
			for i, cand := range cands {
				n++
				ap := v.CompareBuild(b, sl, cand)
				calc.Normalize(&ap, c)
				if want := fmtInt(int64(v.s.optExpected(ap) + 0.5)); r.Rows[0].Values[i] != want {
					t.Errorf("%s %s col %d (%d): shows %s, equipped gives %s", k, sl, i, cand.Item, r.Rows[0].Values[i], want)
				}
			}
		}
	}
	t.Logf("%d columns", n)
}

// TestStressEnchantsExhaustive: on the best traits found, every combination
// of enchants is tried: the choice made position by position is as good.
func TestStressEnchantsExhaustive(t *testing.T) {
	needStress(t)
	c, v := stressCatalog(t)
	builds := stressBuilds(t, c, v, stressN(10), 21)
	checked, tried, worst := 0, 0, 0.0
	limit := 400
	if os.Getenv("STRESS") != "" {
		limit = 6000
	}
	for k, b := range builds {
		for _, any := range []bool{false, true} {
			r := v.CompareTraits(context.Background(), b, TraitOptions{Goal: GoalDamage, AnyLevel: any}, "pt")
			if len(r.Cols) == 0 || r.Partial {
				continue
			}
			nb := cloneBuild(b)
			calc.Normalize(&nb, c)
			pos := v.s.traitPositions(nb, any)
			n := 1
			for _, p := range pos {
				if n *= len(p.opts); n > limit {
					break
				}
			}
			if n > limit || n < 2 {
				continue
			}
			checked++
			best := r.Cols[len(r.Cols)-1]
			top, topBuild := 0.0, calc.Build{}
			for code := 0; code < n; code++ {
				x, rest := cloneBuild(nb), code
				x.Traits = best.Traits
				for _, p := range pos {
					id := p.opts[rest%len(p.opts)]
					rest /= len(p.opts)
					for _, sl := range p.slots {
						e := x.Equip[sl]
						e.Cards[p.idx] = id
						x.Equip[sl] = e
					}
				}
				calc.Normalize(&x, c)
				tried++
				if sc := v.s.optScore(x, GoalDamage); sc > top {
					top, topBuild = sc, x
				}
			}
			gap := top/v.s.optScore(best.Build, GoalDamage) - 1
			worst = max(worst, gap)
			if gap > 0.0005 {
				t.Errorf("%s any=%v: every combination finds %.3f%% more (%v)", k, any, gap*100, topBuild.Traits)
			}
		}
	}
	t.Logf("%d builds, %d combinations, worst gap %.4f%%", checked, tried, worst*100)
}

// TestStressOptimizer: the optimizer's trait suggestion and its other rows
// give, applied, what they show; the build does not change.
func TestStressOptimizer(t *testing.T) {
	needStress(t)
	c, v := stressCatalog(t)
	builds := stressBuilds(t, c, v, stressN(3), 33)
	names := make([]string, 0, len(builds))
	for k := range builds {
		names = append(names, k)
	}
	sort.Strings(names)
	rows, traits := 0, 0
	for i, k := range names {
		if os.Getenv("STRESS") != "" && i%5 != 0 {
			continue
		}
		b := builds[k]
		before := jsonOf(b)
		res := v.Optimize(context.Background(), b, OptOptions{Goal: GoalDamage, Cards: true, Enchants: true, Traits: true}, "pt")
		if jsonOf(b) != before {
			t.Fatalf("%s: the optimizer changed the build", k)
		}
		base := v.s.optScore(b, GoalDamage)
		if base <= 0 {
			continue
		}
		if tr := res.Traits; tr != nil {
			traits++
			ap := v.ApplyTraits(b, tr.Traits, tr.Enchants)
			if got := v.s.optScore(ap, GoalDamage) / base; math.Abs(got-tr.Ratio) > 1e-9*got {
				t.Errorf("%s: traits say %v, applied %v", k, tr.Ratio, got)
			}
		}
		for _, g := range res.Groups {
			for _, o := range g.Options {
				if o.Skill != "" {
					continue // measured on another skill
				}
				rows++
				ap := v.Swap(b, o.Slot, o.Item.ID, o.Index)
				calc.Normalize(&ap, c)
				if got := fmtInt(int64(v.s.optScore(ap, GoalDamage) + 0.5)); got != o.Value {
					t.Errorf("%s %s/%d %s: says %s, applied %s", k, o.Slot, o.Index, o.Item.Name, o.Value, got)
				}
			}
		}
	}
	t.Logf("%d rows, %d trait suggestions", rows, traits)
}
