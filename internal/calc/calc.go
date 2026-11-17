// Package calc computes a character's status window from a build: status
// points, stat costs, derived stats, HP/SP/AP and ASPD, using rAthena's
// renewal formulas (status.cpp / pc.cpp) and item bonuses from scripts.
// It is pure: no I/O, deterministic, safe for concurrent use.
package calc

import (
	"math"
	"slices"
	"sort"
	"strings"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

// Stat indexes.
const (
	STR = iota
	AGI
	VIT
	INT
	DEX
	LUK
)

// Trait indexes (4th jobs).
const (
	POW = iota
	STA
	WIS
	SPL
	CON
	CRT
)

// Equipment slots, in Equipment window order.
const (
	HeadTop   = "Head_Top"
	HeadMid   = "Head_Mid"
	HeadLow   = "Head_Low"
	Armor     = "Armor"
	Weapon    = "Right_Hand"
	Shield    = "Left_Hand"
	Garment   = "Garment"
	Shoes     = "Shoes"
	AccRight  = "Right_Accessory"
	AccLeft   = "Left_Accessory"
	ShArmor   = "Shadow_Armor"
	ShWeapon  = "Shadow_Weapon"
	ShShield  = "Shadow_Shield"
	ShShoes   = "Shadow_Shoes"
	ShAccR    = "Shadow_Right_Accessory"
	ShAccL    = "Shadow_Left_Accessory"
	maxRefine = 20
)

// Slots lists every slot the build supports.
var Slots = []string{HeadTop, HeadMid, HeadLow, Armor, Weapon, Shield, Garment, Shoes, AccRight, AccLeft,
	ShArmor, ShWeapon, ShShield, ShShoes, ShAccR, ShAccL}

// Equip is one equipped item.
type Equip struct {
	Item   int    `json:"i"`
	Refine int    `json:"r,omitempty"`
	Cards  [4]int `json:"c,omitempty"`
}

// Build is everything the player chooses.
type Build struct {
	Job    int              `json:"j"`
	BaseLv int              `json:"b"`
	JobLv  int              `json:"jl"`
	Stats  [6]int           `json:"s"`
	Traits [6]int           `json:"t,omitempty"`
	Equip  map[string]Equip `json:"e,omitempty"`
	Skills map[int]int      `json:"k,omitempty"`
}

// StatLine is one row of the Status window.
type StatLine struct {
	Base  int // allocated
	Bonus int // job bonus + equipment
	Cost  int // points to raise by one (0 when maxed)
}

// Result is the computed Status window.
type Result struct {
	Stats       [6]StatLine
	Traits      [6]StatLine
	Points      int // status points left (negative = over budget)
	TraitPoints int
	MaxStat     int
	MaxTrait    int
	MaxBaseLv   int
	MaxJobLv    int

	HP, SP, AP           int
	StatusATK, EquipATK  int // ATK shown as "StatusATK + EquipATK"
	MATKMin, MATKMax     int
	EquipMATK            int
	HIT, FLEE, FLEE2     int // FLEE2 = perfect dodge
	CRIT                 float64
	HardDEF, SoftDEF     int
	HardMDEF, SoftMDEF   int
	ASPD                 int
	PATK, SMATK          int
	RES, MRES            int
	HPLUS, CRATE         int
	Weight               int
	WeaponType           string
	Bonuses              map[string]int // every applied script bonus, for display
	Extra                map[string]int // bonus2..5 effects (damage modifiers), for display/damage
	SkippedScriptEffects int            // bonuses the evaluator could not apply
	Warnings             []string
}

// Normalize clamps a build to the job's limits so the calculation and the UI
// always agree on what is possible.
func Normalize(b *Build, c *gamedata.Catalog) {
	j := c.Jobs[b.Job]
	maxBase, maxJob := 99, 50
	if j != nil && j.MaxBaseLv > 0 {
		maxBase, maxJob = j.MaxBaseLv, j.MaxJobLv
	}
	b.BaseLv = clamp(b.BaseLv, 1, maxBase)
	b.JobLv = clamp(b.JobLv, 1, maxJob)
	ms := maxStat(j)
	for i := range b.Stats {
		b.Stats[i] = clamp(b.Stats[i], 1, ms)
	}
	for i := range b.Traits {
		if j == nil || !j.Fourth {
			b.Traits[i] = 0
		} else {
			b.Traits[i] = clamp(b.Traits[i], 0, 100)
		}
	}
	for slot, e := range b.Equip {
		it := c.Items[e.Item]
		if !slices.Contains(Slots, slot) || it == nil || !Fits(it, slot) {
			delete(b.Equip, slot)
			continue
		}
		e.Refine = clamp(e.Refine, 0, maxRefine)
		if !it.Refineable {
			e.Refine = 0
		}
		for i, cd := range e.Cards {
			card := c.Items[cd]
			if cd != 0 && (i >= it.Slots || card == nil || !CardFits(card, slot)) {
				e.Cards[i] = 0
			}
		}
		b.Equip[slot] = e
	}
	// A two-handed weapon leaves no room for a shield.
	if w, ok := b.Equip[Weapon]; ok && c.Items[w.Item].HasLocation("Both_Hand") {
		delete(b.Equip, Shield)
	}
}

// Fits reports whether an item can be equipped in a slot.
func Fits(it *gamedata.Item, slot string) bool {
	switch it.Type {
	case "Weapon", "Armor", "ShadowGear":
	default:
		return false
	}
	for _, l := range it.Locations {
		switch {
		case l == slot:
			return true
		case l == "Both_Hand" && slot == Weapon:
			return true
		case l == "Both_Accessory" && (slot == AccRight || slot == AccLeft):
			return true
		case l == "Shadow_Both_Accessory" && (slot == ShAccR || slot == ShAccL):
			return true
		}
	}
	return false
}

// CardFits reports whether a card can be compounded on an item in a slot.
func CardFits(card *gamedata.Item, slot string) bool {
	if card.Type != "Card" {
		return false
	}
	for _, l := range card.Locations {
		switch {
		case l == slot:
			return true
		case l == "Both_Hand" && slot == Weapon:
			return true
		case l == "Both_Accessory" && (slot == AccRight || slot == AccLeft):
			return true
		case strings.HasPrefix(l, "Head_") && strings.HasPrefix(slot, "Head_"):
			return true
		}
	}
	return false
}

func maxStat(j *gamedata.Job) int {
	// rAthena renewal defaults: max_parameter/third/fourth 130, baby 117.
	if j != nil && j.Baby {
		return 117
	}
	return 130
}

// StatCost is PC_STATUS_POINT_COST (renewal): points to raise a stat from x to x+1.
func StatCost(x int) int {
	if x < 100 {
		return 2 + (x-1)/10
	}
	return 16 + 4*((x-100)/5)
}

// StatSpent is the total points used to raise a stat from 1 to x.
func StatSpent(x int) int {
	n := 0
	for v := 1; v < x; v++ {
		n += StatCost(v)
	}
	return n
}

// Compute builds the Status window. The build should be Normalized first.
func Compute(b Build, c *gamedata.Catalog) Result {
	var r Result
	j := c.Jobs[b.Job]
	if j == nil {
		r.Warnings = append(r.Warnings, "classe desconhecida")
		return r
	}
	r.MaxStat, r.MaxTrait = maxStat(j), 100
	r.MaxBaseLv, r.MaxJobLv = j.MaxBaseLv, j.MaxJobLv
	if !j.HasStats() {
		r.Warnings = append(r.Warnings, "tabelas do rAthena ausentes: rode go run ./cmd/fetchrathena e o import")
	}

	// Points.
	total := at(c.StatPoints, b.BaseLv)
	used := 0
	for _, v := range b.Stats {
		used += StatSpent(v)
	}
	r.Points = total - used
	if j.Fourth {
		tp := at(c.TraitPoints, b.BaseLv) + 7 // + trait_points_job_change
		tu := 0
		for _, v := range b.Traits {
			tu += v
		}
		r.TraitPoints = tp - tu
	}

	// Job bonus.
	var jb [12]int
	if b.JobLv < len(j.JobBonus) {
		jb = j.JobBonus[b.JobLv]
	}

	// Equipment bonuses: first pass with base stats for readparam().
	param := map[string]int{"bStr": b.Stats[STR] + jb[0], "bAgi": b.Stats[AGI] + jb[1], "bVit": b.Stats[VIT] + jb[2],
		"bInt": b.Stats[INT] + jb[3], "bDex": b.Stats[DEX] + jb[4], "bLuk": b.Stats[LUK] + jb[5]}
	equipIDs := map[int]bool{}
	for _, e := range b.Equip {
		equipIDs[e.Item] = true
		for _, cd := range e.Cards {
			if cd > 0 {
				equipIDs[cd] = true
			}
		}
	}
	bonus := map[string]int{}
	extra := map[string]int{}
	r.WeaponType = "Fist"
	shield := false
	seen := map[int]bool{} // multi-slot items (e.g. upper+mid headgear) count once
	refineDef := 0         // armor refine DEF in 1/100, summed then divided (rAthena)
	for _, slot := range Slots {
		e, ok := b.Equip[slot]
		if !ok {
			continue
		}
		it := c.Items[e.Item]
		if it == nil {
			continue
		}
		if slot == Weapon && it.Type == "Weapon" {
			r.WeaponType = it.SubType
		}
		if slot == Shield && it.Type == "Armor" {
			shield = true
		}
		if seen[e.Item] && len(it.Locations) > 1 {
			continue
		}
		seen[e.Item] = true
		ctx := script.Context{Refine: e.Refine, BaseLv: b.BaseLv, JobLv: b.JobLv, Param: param, Class: b.Job, EquipIDs: equipIDs}
		res := script.Eval(it.Script, ctx)
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
		r.SkippedScriptEffects += res.Skipped
		for _, cd := range e.Cards {
			if ci := c.Items[cd]; cd > 0 && ci != nil {
				cr := script.Eval(ci.Script, script.Context{BaseLv: b.BaseLv, JobLv: b.JobLv, Param: param, Class: b.Job, EquipIDs: equipIDs})
				addAll(bonus, cr.Bonus)
				addAll(extra, cr.Extra)
				r.SkippedScriptEffects += cr.Skipped
			}
		}
		// Hard DEF/MDEF, weapon/equip ATK with refine (rAthena refine.yml).
		switch {
		case it.Type == "Weapon" && slot == Weapon:
			r.EquipATK += it.Attack + refineRaw(c, "Weapon", it.WeaponLv, e.Refine)/100
			r.EquipMATK += it.MagicAttack
		case it.Type == "Weapon" && slot == ShWeapon:
			r.EquipATK += it.Attack
		case it.Type == "Armor" || it.Type == "ShadowGear":
			r.HardDEF += it.Defense
			if it.Refineable && it.Type == "Armor" {
				refineDef += refineRaw(c, "Armor", max(it.ArmorLv, 1), e.Refine)
			}
			r.EquipMATK += it.MagicAttack
			r.EquipATK += it.Attack
		}
	}
	r.Bonuses = bonus
	r.Extra = extra
	r.HardDEF += refineDef / 100

	// Final stats.
	all := bonus["bAllStats"]
	eq := [6]int{
		bonus["bStr"] + all + bonus["bAgiDexStr"],
		bonus["bAgi"] + all + bonus["bAgiVit"] + bonus["bAgiDexStr"],
		bonus["bVit"] + all + bonus["bAgiVit"],
		bonus["bInt"] + all,
		bonus["bDex"] + all + bonus["bAgiDexStr"],
		bonus["bLuk"] + all,
	}
	var st [6]int
	for i := range st {
		r.Stats[i] = StatLine{Base: b.Stats[i], Bonus: jb[i] + eq[i]}
		if b.Stats[i] < r.MaxStat {
			r.Stats[i].Cost = StatCost(b.Stats[i])
		}
		st[i] = max(0, b.Stats[i]+jb[i]+eq[i])
	}
	allT := bonus["bAllTraitStats"]
	teq := [6]int{bonus["bPow"] + allT, bonus["bSta"] + allT, bonus["bWis"] + allT, bonus["bSpl"] + allT, bonus["bCon"] + allT, bonus["bCrt"] + allT}
	var tr [6]int
	if j.Fourth {
		for i := range tr {
			r.Traits[i] = StatLine{Base: b.Traits[i], Bonus: jb[6+i] + teq[i]}
			if b.Traits[i] < r.MaxTrait {
				r.Traits[i].Cost = 1
			}
			tr[i] = max(0, b.Traits[i]+jb[6+i]+teq[i])
		}
	}
	lv := b.BaseLv

	// ATK / MATK (status_base_atk, status_base_matk_min/max: renewal, players).
	// Ranged weapons swap the roles of STR and DEX.
	ranged := isRanged(r.WeaponType)
	main, sub := st[STR], st[DEX]
	if ranged {
		main, sub = st[DEX], st[STR]
	}
	r.StatusATK = (main*10+sub*10/5+st[LUK]*10/3+lv*10/4)/10 + 5*tr[POW]
	r.StatusATK += bonus["bBaseAtk"]
	r.EquipATK += bonus["bAtk"] + bonus["bAtk2"]
	matk := st[INT] + st[INT]/2 + st[DEX]/5 + st[LUK]/3 + lv/4 + 5*tr[SPL]
	r.MATKMin, r.MATKMax = matk, matk
	r.EquipMATK += bonus["bMatk"]

	// HIT / FLEE / CRIT / DEF / MDEF (status_calc_misc, renewal).
	r.HIT = lv + st[DEX] + st[LUK]/3 + 175 + 2*tr[CON] + bonus["bHit"]
	r.FLEE = lv + st[AGI] + st[LUK]/5 + 100 + 2*tr[CON] + bonus["bFlee"]
	r.FLEE2 = (st[LUK]+10)/10 + bonus["bFlee2"]
	r.CRIT = float64(lv/10+10+st[LUK]*3)/10 + float64(bonus["bCritical"])
	r.SoftDEF = int(float64(lv+st[VIT])/2+float64(st[AGI])/5) + bonus["bDef2"]
	r.SoftMDEF = int(float64(st[INT])+float64(lv)/4+float64(st[DEX]+st[VIT])/5) + bonus["bMdef2"]
	r.HardDEF += bonus["bDef"]
	r.HardMDEF += bonus["bMdef"]
	r.PATK = tr[POW]/3 + tr[CON]/5 + bonus["bPAtk"]
	r.SMATK = tr[SPL]/3 + tr[CON]/5 + bonus["bSMatk"]
	r.RES = tr[STA] + tr[STA]/3*5 + bonus["bRes"]
	r.MRES = tr[WIS] + tr[WIS]/3*5 + bonus["bMRes"]
	r.HPLUS = tr[CRT] + bonus["bHPlus"]
	r.CRATE = tr[CRT]/3 + bonus["bCRate"]

	// HP / SP / AP (status_calc_maxhp_pc / maxsp / maxap).
	r.HP = maxPool(at(j.BaseHP, lv), st[VIT], j.Upper, eq[VIT], bonus["bMaxHP"], bonus["bMaxHPrate"])
	r.SP = maxPool(at(j.BaseSP, lv), st[INT], j.Upper, eq[INT], bonus["bMaxSP"], bonus["bMaxSPrate"])
	if j.Fourth {
		ap := float64(at(j.BaseAP, lv)) + float64(bonus["bMaxAP"])
		ap += math.Trunc(ap * float64(bonus["bMaxAPrate"]) / 100)
		r.AP = int(math.Max(ap, 0))
	}

	// ASPD (status_base_amotion_pc, RENEWAL_ASPD).
	r.ASPD = aspd(j, r.WeaponType, shield, st[AGI], st[DEX], ranged, bonus["bAspdRate"], bonus["bAspd"])

	if r.Points < 0 {
		r.Warnings = append(r.Warnings, "pontos de atributo insuficientes")
	}
	if r.TraitPoints < 0 {
		r.Warnings = append(r.Warnings, "pontos de traço insuficientes")
	}
	return r
}

// refineRaw returns the cumulative refine bonus in 1/100 (rAthena refine.yml).
func refineRaw(c *gamedata.Catalog, group string, level, refine int) int {
	t := c.Refine[group][level]
	if refine <= 0 || len(t) == 0 {
		return 0
	}
	return t[min(refine, len(t)-1)]
}

func maxPool(base, stat int, upper bool, equipStat, fix, rate int) int {
	d := float64(base)
	if stat > 0 {
		d *= 1 + float64(stat)*0.01
	}
	if upper {
		d *= 1.25
	}
	d += float64(equipStat) // stat from equipment gives +1 extra
	d += float64(fix)
	d += math.Trunc(d * float64(rate) / 100)
	if d < 1 {
		d = 1
	}
	return int(d)
}

func aspd(j *gamedata.Job, weapon string, shield bool, agi, dex int, ranged bool, rate, add int) int {
	base, ok := j.BaseASPD[weapon]
	if !ok {
		base = j.BaseASPD["Fist"]
	}
	if shield {
		base += j.BaseASPD["Shield"]
	}
	var t float64
	if ranged {
		t = float64(dex*dex)/7 + float64(agi*agi)*0.5
	} else {
		t = float64(dex*dex)/5 + float64(agi*agi)*0.5
	}
	t = math.Sqrt(t)*0.25 + 196
	v := int(t+float64(rate)*float64(agi)/200) - min(base, 200)
	v += add
	return clamp(v, 0, 193)
}

func isRanged(w string) bool {
	switch w {
	case "Bow", "Musical", "Whip", "Revolver", "Rifle", "Gatling", "Shotgun", "Grenade":
		return true
	}
	return false
}

func at(t []int, lv int) int {
	if lv < 1 || lv > len(t) {
		if len(t) > 0 && lv > len(t) {
			return t[len(t)-1]
		}
		return 0
	}
	return t[lv-1]
}

func addAll(dst, src map[string]int) {
	for k, v := range src {
		dst[k] += v
	}
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

// SortedBonuses returns applied bonuses in a stable order for display.
func (r Result) SortedBonuses() [][2]any {
	keys := make([]string, 0, len(r.Bonuses))
	for k, v := range r.Bonuses {
		if v != 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([][2]any, len(keys))
	for i, k := range keys {
		out[i] = [2]any{k, r.Bonuses[k]}
	}
	return out
}
