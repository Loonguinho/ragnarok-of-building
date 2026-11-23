// Package calc computes a character's status window from a build: status
// points, stat costs, derived stats, HP/SP/AP and ASPD, using rAthena's
// renewal formulas (status.cpp / pc.cpp) and item bonuses from scripts.
// It is pure: no I/O, deterministic, safe for concurrent use.
package calc

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strconv"
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
	HeadTop  = "Head_Top"
	HeadMid  = "Head_Mid"
	HeadLow  = "Head_Low"
	Armor    = "Armor"
	Weapon   = "Right_Hand"
	Shield   = "Left_Hand"
	Garment  = "Garment"
	Shoes    = "Shoes"
	AccRight = "Right_Accessory"
	AccLeft  = "Left_Accessory"
	// Costumes (visual): each holds one enchant stone.
	CosTop     = "Costume_Head_Top"
	CosMid     = "Costume_Head_Mid"
	CosLow     = "Costume_Head_Low"
	CosGarment = "Costume_Garment"
	ShArmor    = "Shadow_Armor"
	ShWeapon   = "Shadow_Weapon"
	ShShield   = "Shadow_Shield"
	ShShoes    = "Shadow_Shoes"
	ShAccR     = "Shadow_Right_Accessory"
	ShAccL     = "Shadow_Left_Accessory"
	maxRefine  = 20
	// MaxRandomOpts is how many random options an item holds (client: 5).
	MaxRandomOpts = 5
)

// Slots lists every slot the build supports.
var Slots = []string{HeadTop, HeadMid, HeadLow, Armor, Weapon, Shield, Garment, Shoes, AccRight, AccLeft,
	ShArmor, ShWeapon, ShShield, ShShoes, ShAccR, ShAccL, CosTop, CosMid, CosLow, CosGarment}

// IsCostume reports a costume (visual) slot.
func IsCostume(slot string) bool { return strings.HasPrefix(slot, "Costume_") }

// Equip is one equipped item.
type Equip struct {
	Item   int `json:"i"`
	Refine int `json:"r,omitempty"`
	Grade  int `json:"g,omitempty"` // enchant grade: 0 none, 1 D, 2 C, 3 B, 4 A
	// Opts are random options ("bônus aleatórios"): [option ID, value].
	Opts  [][2]int `json:"o,omitempty"`
	Cards [4]int   `json:"c,omitempty"`
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
	Target int              `json:"m,omitempty"` // monster ID for the damage window
	Atk    AttackChoice     `json:"a,omitempty"`
	Buffs  map[string]int   `json:"bf,omitempty"` // BuffDef.Key -> level/value
	// Pet: its egg item ID (0 = none) and intimacy, 0-1000.
	Pet         int `json:"p,omitempty"`
	PetIntimacy int `json:"pi,omitempty"`
}

// Clone copies the build's maps (Normalize and Compute write to them), so
// the copy can go to another goroutine.
func (b Build) Clone() Build {
	b.Equip, b.Skills, b.Buffs = maps.Clone(b.Equip), maps.Clone(b.Skills), maps.Clone(b.Buffs)
	b.Atk.States = maps.Clone(b.Atk.States)
	return b
}

// AttackChoice is what the damage window uses against the target.
type AttackChoice struct {
	Skill   int    `json:"s,omitempty"` // 0 = basic attack
	Level   int    `json:"l,omitempty"`
	Ratio   int    `json:"r,omitempty"` // % ATK; 0 = take it from the skill description
	Hits    int    `json:"h,omitempty"`
	Element string `json:"e,omitempty"` // "" = weapon element
	// Class states for LATAM formulas (e.g. Celestial Space) and stacks.
	States map[string]int `json:"st,omitempty"`
	Stack  int            `json:"sk,omitempty"`
	// Variant of skills with several damage lines (0 = first).
	Variant int `json:"v,omitempty"`
}

// Elements an attack can be forced to (endows, elemental skills).
var Elements = []string{"Neutral", "Water", "Earth", "Fire", "Wind", "Poison", "Holy", "Dark", "Ghost", "Undead"}

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

	HP, SP, AP          int
	StatusATK, EquipATK int // ATK shown as "StatusATK + EquipATK"
	// Parts of EquipATK the damage formula treats differently.
	WeaponATK        int // weapon's own ATK (after bWeaponAtkRate)
	WeaponRefineATK  int // refine bonus ATK
	OtherEquipATK    int // armor ATK, bAtk, shadow weapon
	WeaponLv         int
	WeaponRefine     int
	WeaponWeight     int           // tenths
	ShieldWeight     int           // tenths; 0 without a shield
	Combos           [][]int       // item sets whose combo bonus is active
	ComboEffects     []ComboEffect // what each active combo adds (same order)
	ShieldRefine     int
	Ranged           bool
	CritTenths       int    // CRIT in 1/10 % (rAthena cri), for crit chance
	PerfectHit       int    // % of attacks that ignore the target's flee
	st, tr           [6]int // final stats/traits (for the damage formula)
	MATKMin, MATKMax int
	EquipMATK        int
	// Parts of EquipMATK for the magic formula.
	WeaponMATK           int // weapon's own MATK
	WeaponRefineMATK     int // refine bonus MATK (not for bows)
	OtherEquipMATK       int // armor MATK, bMatk
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
	ItemEffects          []ItemEffect   // triggered item bonuses the build can turn on
	// PetBonus and PetExtra: what the pet gives (already in Bonuses/Extra).
	PetBonus, PetExtra map[string]int
	Warnings           []string
}

// Normalize clamps a build to the job's limits so the calculation and the UI
// always agree on what is possible. It returns a note for every item it had
// to remove because the class cannot equip it.
func Normalize(b *Build, c *gamedata.Catalog) (notes []string) {
	j := c.Jobs[b.Job]
	maxBase, maxJob := 99, 50
	if j != nil && j.MaxBaseLv > 0 {
		maxBase, maxJob = j.MaxBaseLv, j.MaxJobLv
	}
	b.BaseLv = clamp(b.BaseLv, 1, maxBase)
	b.JobLv = clamp(b.JobLv, 1, maxJob)
	normalizeBuffs(b)
	// A party buff the class has as its own (the Shura's Chakras) moves to
	// the class buff: the Buffs window shows only that one.
	if len(b.Buffs) > 0 {
		for _, cb := range ClassBuffsFor(c, b.Job) {
			if cb.Skill == "" {
				continue
			}
			for _, d := range Buffs {
				if v := b.Buffs[d.Key]; d.Skill == cb.Skill && v > 0 {
					delete(b.Buffs, d.Key)
					if b.Atk.States == nil {
						b.Atk.States = map[string]int{}
					}
					if b.Atk.States[cb.Key] == 0 && cb.validValue(v) {
						b.Atk.States[cb.Key] = v
					}
				}
			}
		}
	}
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
		// Class restrictions from rAthena are not always right for LATAM
		// (replays show gear worn that rAthena forbids), so they only warn.
		if !j.CanEquip(it) {
			notes = append(notes, it.Name.Get("pt")+": pelos dados do rAthena esta classe não usaria este item.")
		}
		e.Refine = clamp(e.Refine, 0, maxRefine)
		if !it.Refineable {
			e.Refine = 0
		}
		e.Grade = clamp(e.Grade, 0, 4)
		if !it.Gradable {
			e.Grade = 0
		}
		// Up to 5 known options; values fit the client's int16.
		var opts [][2]int
		for _, o := range e.Opts {
			if c.RandomOpts[o[0]] != nil && len(opts) < MaxRandomOpts {
				opts = append(opts, [2]int{o[0], clamp(o[1], -32768, 32767)})
			}
		}
		e.Opts = opts
		// A costume holds enchant stones: those of its own slot, or the
		// older enchants that name no slot (Fixa, Ganância...). LATAM keeps
		// the stone in a slot's own position (the 3rd on a lower costume),
		// so any position counts.
		if IsCostume(slot) {
			e.Refine, e.Grade, e.Opts = 0, 0, nil
			for i := range e.Cards {
				card := c.Items[e.Cards[i]]
				if card == nil || !(CardFits(card, slot) || card.Type == "Card" && card.SubType == "Enchant" && len(card.Locations) == 0) {
					e.Cards[i] = 0
				}
			}
			b.Equip[slot] = e
			continue
		}
		// The client keeps cards and enchants in the same 4 positions: the
		// item's card slots first, enchants after them.
		for i, cd := range e.Cards {
			if cd == 0 {
				continue
			}
			card := c.Items[cd]
			switch {
			case card == nil:
				e.Cards[i] = 0
			case card.Type == "": // no rAthena data (LATAM-only): keep, cannot validate
			case i < it.Slots && !CardFits(card, slot):
				e.Cards[i] = 0
			case i >= it.Slots && card.SubType != "Enchant":
				e.Cards[i] = 0
			}
		}
		b.Equip[slot] = e
	}
	// A two-handed weapon leaves no room for a shield.
	if w, ok := b.Equip[Weapon]; ok && c.Items[w.Item].HasLocation("Both_Hand") {
		delete(b.Equip, Shield)
	}
	normalizeSkills(b, c)
	if c.Pets[b.Pet] == nil {
		b.Pet, b.PetIntimacy = 0, 0
	}
	b.PetIntimacy = clamp(b.PetIntimacy, 0, 1000)
	if c.Monster(b.Target) == nil {
		b.Target = 0
	}
	at := &b.Atk
	if at.Skill != 0 && b.Skills[at.Skill] == 0 {
		*at = AttackChoice{Ratio: at.Ratio, Hits: at.Hits, Element: at.Element} // skill no longer learned
	}
	if at.Skill != 0 {
		at.Level = clamp(at.Level, 1, b.Skills[at.Skill])
	} else {
		at.Level = 0
	}
	at.Ratio = clamp(at.Ratio, 0, 1000000)
	at.Hits = clamp(at.Hits, 0, 50)
	if at.Element != "" && !slices.Contains(Elements, at.Element) {
		at.Element = ""
	}
	// Only the selectors the class shows are kept: formula states and the
	// class's own buffs (which share their keys).
	allowed := map[string]bool{}
	top := map[string]int{} // highest choice of each state (10 when it lists none above)
	for _, o := range StatesFor(c, b.Job) {
		allowed[o.Key] = true
		top[o.Key] = 10
		for _, ch := range o.Choices {
			if n, err := strconv.Atoi(ch[0]); err == nil && n > top[o.Key] {
				top[o.Key] = n
			}
		}
	}
	own := map[string]*ClassBuff{}
	for _, cb := range ClassBuffsFor(c, b.Job) {
		own[cb.Key] = &cb
		allowed[cb.Key] = true
	}
	for k, v := range at.States {
		switch {
		case !allowed[k] || v == 0:
			delete(at.States, k)
		case own[k] != nil && own[k].validValue(v):
		default:
			at.States[k] = clamp(v, 0, max(10, top[k]))
		}
	}
	// One class buff per exclusive group (the three Mantras...): the first in order.
	excl := map[string]bool{}
	for _, cb := range ClassBuffsFor(c, b.Job) {
		if cb.Excl == "" || at.States[cb.Key] == 0 {
			continue
		}
		if excl[cb.Excl] {
			delete(at.States, cb.Key)
		}
		excl[cb.Excl] = true
	}
	at.Stack = clamp(at.Stack, 0, 10)
	slices.Sort(notes)
	return slices.Compact(notes) // multi-slot items are reported once
}

// treeSkills maps every skill in a job chain's tabs to its class step index.
func treeSkills(c *gamedata.Catalog, job int) map[int]int {
	out := map[int]int{}
	for gi, g := range c.SkillBudgets(job, 0) {
		for _, id := range g.Jobs {
			for _, sk := range c.Jobs[id].Tree {
				if _, ok := out[sk]; !ok {
					out[sk] = gi
				}
			}
		}
	}
	return out
}

// normalizeSkills drops skills outside the job's trees and clamps levels.
func normalizeSkills(b *Build, c *gamedata.Catalog) {
	if len(b.Skills) == 0 {
		return
	}
	// Skills outside the job's trees (e.g. learned by quest, like Greed) are
	// kept when they exist; they do not count toward the skill point budget.
	for id, lv := range b.Skills {
		sk := c.Skills[id]
		if sk == nil || lv <= 0 {
			delete(b.Skills, id)
			continue
		}
		b.Skills[id] = min(lv, sk.MaxLv)
	}
}

// SkillPointCheck returns the class step whose skills, with the later steps',
// need more points than those steps give, or -1 if all fit.
func SkillPointCheck(b Build, c *gamedata.Catalog) int {
	groups := c.SkillBudgets(b.Job, b.JobLv)
	for _, g := range groups {
		if g.Budget < 0 {
			return -1
		}
	}
	used := make([]int, len(groups))
	allowed := treeSkills(c, b.Job)
	for id, lv := range b.Skills {
		gi, inTree := allowed[id]
		if sk := c.Skills[id]; sk != nil && inTree && !sk.Free() {
			used[gi] += lv
		}
	}
	// A point earned in a class step can be spent on that step's skills or an
	// earlier step's (a Knight still raises Swordman skills), never a later
	// one: the skills from each step on must fit the points from that step on
	// (pc_calc_skilltree_normalize_job). The deepest step that fails is the
	// one to report.
	u, budget := 0, 0
	for i := len(groups) - 1; i >= 0; i-- {
		u += used[i]
		budget += groups[i].Budget
		if u > budget {
			return i
		}
	}
	return -1
}

// Fits reports whether an item can be equipped in a slot. Items without
// rAthena data (LATAM-only, Type "") cannot be checked and are allowed.
func Fits(it *gamedata.Item, slot string) bool {
	switch it.Type {
	case "":
		return true
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
		// Accessories go in either slot in-game, whatever side rAthena lists
		// (replays show "right" accessories worn on the left).
		case (l == "Both_Accessory" || l == "Right_Accessory" || l == "Left_Accessory") && (slot == AccRight || slot == AccLeft):
			return true
		case l == "Shadow_Both_Accessory" && (slot == ShAccR || slot == ShAccL):
			return true
		}
	}
	return false
}

// CardFits reports whether a card can be compounded on an item in a slot.
// A costume holds only the stones of its slot (their location comes from
// the client name); other enchants have no location and fit any other item.
func CardFits(card *gamedata.Item, slot string) bool {
	if card.Type != "Card" {
		return false
	}
	stone := false
	for _, l := range card.Locations {
		if IsCostume(l) {
			stone = true
			if l == slot {
				return true
			}
		}
	}
	if stone || IsCostume(slot) {
		return false
	}
	if card.SubType == "Enchant" { // enchants have no location: they fit any item
		return true
	}
	for _, l := range card.Locations {
		switch {
		case l == slot:
			return true
		case l == "Both_Hand" && slot == Weapon:
			return true
		case (l == "Both_Accessory" || l == "Right_Accessory" || l == "Left_Accessory") && (slot == AccRight || slot == AccLeft):
			return true
		case strings.HasPrefix(l, "Head_") && strings.HasPrefix(slot, "Head_"):
			return true
		}
	}
	return false
}

// maxStat is the highest base stat a class can reach (rAthena's
// conf/battle/player.conf, pc_maxparameter): 99 for first, second and
// transcendent classes, 80 for their baby versions, 117 for baby 3rd
// classes and 130 for 3rd and 4th classes and the expanded ones
// (Kagerou/Oboro, Rebellion, Summoner and later).
func maxStat(j *gamedata.Job) int {
	if j == nil {
		return 130
	}
	switch j.EquipTier {
	case "Normal", "Upper":
		switch strings.TrimSuffix(strings.TrimPrefix(j.Code, "JT_"), "_H") {
		case "KAGEROU", "OBORO", "REBELLION", "DO_SUMMONER", "SPIRIT_HANDLER":
			return 130
		}
		if j.Fourth { // 4th classes rAthena files as Normal
			return 130
		}
		return 99
	case "Baby":
		return 80
	case "Third_Baby":
		return 117
	case "":
		if j.Baby {
			return 117
		}
	}
	return 130
}

// TranscendentStatPoints are the extra status points of transcendent
// classes and primary 4th classes (rAthena transcendent_status_points;
// pc_resetstate adds them on top of the level table).
const TranscendentStatPoints = 52

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
	// LATAM caps traits at 100 (confirmed in game, 2026-11); rAthena now uses 110.
	r.MaxStat, r.MaxTrait = maxStat(j), 100
	r.MaxBaseLv, r.MaxJobLv = j.MaxBaseLv, j.MaxJobLv
	if !j.HasStats() {
		r.Warnings = append(r.Warnings, "tabelas do rAthena ausentes: rode go run ./cmd/fetchrathena e o import")
	}

	// Points.
	total := at(c.StatPoints, b.BaseLv)
	if j.Upper { // JOBL_UPPER or primary 4th (Job.Upper)
		total += TranscendentStatPoints
	}
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

	// readparam(bStr) is the allocated points only, without the class
	// bonus (rAthena status.str; latam-ro-calc): an Executor replay's
	// Constelação do Poder (POD ÷ 15) and Orelhas Espirituais ((FOR + VIT)
	// ÷ 15) match the status window only this way.
	param := map[string]int{"bStr": b.Stats[STR], "bAgi": b.Stats[AGI], "bVit": b.Stats[VIT],
		"bInt": b.Stats[INT], "bDex": b.Stats[DEX], "bLuk": b.Stats[LUK],
		// Traits (4th jobs): readparam(bCon) etc. in the Nebula/Constellation enchants.
		"bPow": b.Traits[POW], "bSta": b.Traits[STA], "bWis": b.Traits[WIS],
		"bSpl": b.Traits[SPL], "bCon": b.Traits[CON], "bCrt": b.Traits[CRT]}
	equipIDs := map[int]bool{}
	for _, e := range b.Equip {
		equipIDs[e.Item] = true
		for _, cd := range e.Cards {
			if cd > 0 {
				equipIDs[cd] = true
			}
		}
	}
	if b.Pet > 0 { // the pet's egg counts as worn for sets (Cesta de Mascotes, Espetinho Doce)
		equipIDs[b.Pet] = true
	}
	slotRefine, slotGrade := slotStates(b)
	sbase := scriptBase(b, c, param, equipIDs, slotRefine, slotGrade)
	lenv := newLatamEnv(b, c) // LATAM item scripts (latamitem.go)
	effects := &itemEffects{b: &b, c: c, env: lenv, seen: map[int]bool{}}
	bonus := map[string]int{}
	extra := map[string]int{}
	r.WeaponType = "Fist"
	shield := false
	seen := map[int]bool{} // multi-slot items (e.g. upper+mid headgear) count once
	refineDef := 0         // armor refine DEF in 1/100, summed then divided (rAthena)
	wlv5Traits := 0        // P.ATK/S.MATK from level 5 weapon refine
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
			r.ShieldWeight, r.ShieldRefine = it.Weight, e.Refine
		}
		if seen[e.Item] && multiHead(it) {
			continue
		}
		seen[e.Item] = true
		ctx := sbase
		ctx.Refine, ctx.Grade = e.Refine, e.Grade
		ctx.ArmorLv, ctx.WeaponLv = it.ArmorLv, it.WeaponLv
		res := script.Eval(itemScript(it, &r.SkippedScriptEffects), ctx)
		lenv.overLatam(it, e.Item, slot, true, e.Refine, &res)
		effects.add(e.Item, it, ctx, res.Autobonus, slot, true, e.Refine)
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
		r.SkippedScriptEffects += res.Skipped
		for _, o := range e.Opts {
			if ro := c.RandomOpts[o[0]]; ro != nil {
				octx := ctx
				octx.OptValue = o[1]
				or := script.Eval(ro.Script, octx)
				addAll(bonus, or.Bonus)
				addAll(extra, or.Extra)
				r.SkippedScriptEffects += or.Skipped
			}
		}
		for _, cd := range e.Cards {
			if ci := c.Items[cd]; cd > 0 && ci != nil {
				// Cards and enchants read the refine and grade of the item they are in.
				cctx := sbase
				cctx.Refine, cctx.Grade = e.Refine, e.Grade
				cctx.ArmorLv, cctx.WeaponLv = it.ArmorLv, it.WeaponLv // the item the card is in
				cr := script.Eval(itemScript(ci, &r.SkippedScriptEffects), cctx)
				lenv.overLatam(ci, cd, slot, false, e.Refine, &cr)
				effects.add(cd, ci, cctx, cr.Autobonus, slot, false, e.Refine)
				addAll(bonus, cr.Bonus)
				addAll(extra, cr.Extra)
				r.SkippedScriptEffects += cr.Skipped
			}
		}
		// Hard DEF/MDEF, weapon/equip ATK with refine (rAthena refine.yml).
		switch {
		case it.Type == "Weapon" && slot == Weapon:
			r.WeaponATK = it.Attack
			// A grade adds a % of the refine bonus (rAthena status_calc_pc).
			gradePct := c.GradeBonus["Weapon"][it.WeaponLv][e.Grade]
			refAtk := refineRaw(c, "Weapon", it.WeaponLv, e.Refine) / 100
			r.WeaponRefineATK = refAtk + refAtk*gradePct/100
			r.WeaponLv, r.WeaponRefine, r.WeaponWeight = it.WeaponLv, e.Refine, it.Weight
			r.WeaponMATK = it.MagicAttack
			if it.SubType != "Bow" {
				r.WeaponRefineMATK = refAtk + refAtk*gradePct/100
			}
			if it.WeaponLv == 5 { // 4th-gen weapons: refine also gives P.ATK/S.MATK
				wlv5Traits += e.Refine * 2
			}
		case it.Type == "Weapon" && slot == ShWeapon:
			r.OtherEquipATK += it.Attack
		case it.Type == "Armor" || it.Type == "ShadowGear":
			r.HardDEF += it.Defense
			if it.Refineable && it.Type == "Armor" {
				refineDef += refineRaw(c, "Armor", max(it.ArmorLv, 1), e.Refine)
			}
			r.OtherEquipMATK += it.MagicAttack
			r.OtherEquipATK += it.Attack
		}
	}
	r.Combos, r.ComboEffects = applyCombos(c, &b, sbase, bonus, extra, &r.SkippedScriptEffects)
	r.ItemEffects = effects.list
	for _, ef := range effects.list {
		if ef.On {
			addAll(bonus, ef.Bonus)
			addAll(extra, ef.Extra)
		}
	}
	if p := c.Pets[b.Pet]; p != nil {
		ctx := sbase
		ctx.PetIntimacy = b.PetIntimacy
		res := script.Eval(p.Script, ctx)
		r.PetBonus, r.PetExtra = res.Bonus, res.Extra
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
	}
	var baseJob [6]int
	for i := range baseJob {
		baseJob[i] = b.Stats[i] + jb[i]
	}
	applyBuffs(&b, baseJob, bonus, extra)
	applyClassBuffs(&b, c, bonus, extra)                  // the class's own buffs (Buffs window, Próprios)
	applyConsumables(&b, c, lenv, bonus, extra)           // consumables (Buffs window, Consumíveis)
	LatamPassives(b, c, r.WeaponType, bonus, extra)       // hand-written class rules (latam_class.go)
	applyClassPassives(&b, c, r.WeaponType, bonus, extra) // the class passives generated from latam-ro-calc
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
	r.Ranged = ranged
	main, sub := st[STR], st[DEX]
	if ranged {
		main, sub = st[DEX], st[STR]
	}
	r.StatusATK = (main*10+sub*10/5+st[LUK]*10/3+lv*10/4)/10 + 5*tr[POW]
	// bBaseAtk ("ATQ +X" of items) is equipment ATK in renewal (pc.cpp
	// SP_BASE_ATK -> bonus.eatk, battle_calc_equip_attack), not status ATK.
	// Weapon ATK (scaled by bWeaponAtkRate, as rAthena does on wa->atk) + refine,
	// and other equipment ATK (armor ATK, bAtk): kept apart for the damage formula.
	r.WeaponATK += r.WeaponATK * bonus["bWeaponAtkRate"] / 100
	r.OtherEquipATK += bonus["bAtk"] + bonus["bAtk2"] + bonus["bBaseAtk"]
	r.EquipATK = r.WeaponATK + r.WeaponRefineATK + r.OtherEquipATK
	matk := st[INT] + st[INT]/2 + st[DEX]/5 + st[LUK]/3 + lv/4 + 5*tr[SPL]
	r.MATKMin, r.MATKMax = matk, matk
	r.OtherEquipMATK += bonus["bMatk"]
	r.EquipMATK = r.WeaponMATK + r.WeaponRefineMATK + r.OtherEquipMATK

	// HIT / FLEE / CRIT / DEF / MDEF (status_calc_misc, renewal).
	r.HIT = lv + st[DEX] + st[LUK]/3 + 175 + 2*tr[CON] + bonus["bHit"]
	r.FLEE = lv + st[AGI] + st[LUK]/5 + 100 + 2*tr[CON] + bonus["bFlee"]
	r.FLEE2 = (st[LUK]+10)/10 + bonus["bFlee2"]
	r.CritTenths = lv/10 + 10 + st[LUK]*3 + 10*bonus["bCritical"] + bonus["bCritTenths"]
	if r.WeaponType == "Katar" { // status_calc_bl: katars double CRIT (after every bonus)
		r.CritTenths *= 2
	}
	r.CRIT = float64(r.CritTenths) / 10
	// Perfect hit: rolled before the target's flee (is_attack_hitting). SOR ÷ 10
	// follows latam-ro-calc's LATAM measurements; items add bPerfectHitRate.
	r.PerfectHit = st[LUK]/10 + bonus["bPerfectHitRate"] + bonus["bPerfectHitAddRate"]
	r.SoftDEF = int(float64(lv+st[VIT])/2+float64(st[AGI])/5) + bonus["bDef2"]
	r.SoftMDEF = int(float64(st[INT])+float64(lv)/4+float64(st[DEX]+st[VIT])/5) + bonus["bMdef2"]
	r.HardDEF += bonus["bDef"]
	r.HardMDEF += bonus["bMdef"]
	r.PATK = tr[POW]/3 + tr[CON]/5 + bonus["bPAtk"] + wlv5Traits
	r.SMATK = tr[SPL]/3 + tr[CON]/5 + bonus["bSMatk"] + wlv5Traits
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
	r.ASPD = aspd(j, r.WeaponType, shield, st[AGI], st[DEX], ranged, bonus["bAspdRate"], bonus["bAspdFixed"], bonus["bAspd"])

	for _, slot := range Slots {
		if e, ok := b.Equip[slot]; ok {
			if it := c.Items[e.Item]; it != nil && it.EquipLvMin > b.BaseLv {
				r.Warnings = append(r.Warnings, fmt.Sprintf("%s exige nível base %d", it.Name.Get("pt"), it.EquipLvMin))
			}
		}
	}
	r.st, r.tr = st, tr
	if r.Points < 0 {
		r.Warnings = append(r.Warnings, "pontos de atributo insuficientes")
	}
	if gi := SkillPointCheck(b, c); gi >= 0 {
		g := c.SkillBudgets(b.Job, b.JobLv)[gi]
		r.Warnings = append(r.Warnings, "pontos de habilidade insuficientes a partir de "+c.Jobs[g.Jobs[len(g.Jobs)-1]].Name.Get("pt"))
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

// aspd is rAthena's renewal ASPD (status_base_amotion_pc + status_calc_bl),
// which latam-ro-calc's LATAM measurements follow:
//  1. √(DES²/5 + AGI²/2) × 0,25 + 196 (DES²/7 for ranged weapons), plus the
//     "fixed" bonuses of potions and quicken skills × AGI ÷ 200, minus the
//     weapon's (and shield's) delay;
//  2. ASPD % (equipment, Aumentar Agilidade...) on the room left to 195:
//     + max(195 − ASPD, 2) × % ÷ 100;
//  3. flat ASPD (bAspd).
func aspd(j *gamedata.Job, weapon string, shield bool, agi, dex int, ranged bool, rate, fixed, add int) int {
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
	v := int(t+float64(fixed)*float64(agi)/200) - min(base, 200)
	v += max(195-v, 2) * rate / 100
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

// FinalStats returns STR..LUK with job and equipment bonuses.
func (r Result) FinalStats() [6]int { return r.st }

// FinalTraits returns POW..CRT with job and equipment bonuses.
func (r Result) FinalTraits() [6]int { return r.tr }

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

// eqiSlots maps rAthena's equipment positions (getequiprefinerycnt(EQI_x))
// to the build's slots.
var eqiSlots = map[string]string{
	"EQI_HEAD_TOP": HeadTop, "EQI_HEAD_MID": HeadMid, "EQI_HEAD_LOW": HeadLow, "EQI_ARMOR": Armor,
	"EQI_HAND_R": Weapon, "EQI_HAND_L": Shield, "EQI_GARMENT": Garment, "EQI_SHOES": Shoes,
	"EQI_ACC_R": AccRight, "EQI_ACC_L": AccLeft,
	"EQI_SHADOW_ARMOR": ShArmor, "EQI_SHADOW_WEAPON": ShWeapon, "EQI_SHADOW_SHIELD": ShShield,
	"EQI_SHADOW_SHOES": ShShoes, "EQI_SHADOW_ACC_R": ShAccR, "EQI_SHADOW_ACC_L": ShAccL,
	"EQI_COSTUME_HEAD_TOP": CosTop, "EQI_COSTUME_HEAD_MID": CosMid, "EQI_COSTUME_HEAD_LOW": CosLow,
	"EQI_COSTUME_GARMENT": CosGarment,
}

// slotStates are the refine and enchant grade of every position (0 when
// empty), for scripts that read another position than their own.
func slotStates(b Build) (refine, grade map[string]int) {
	refine, grade = map[string]int{}, map[string]int{}
	for eqi, slot := range eqiSlots {
		e := b.Equip[slot]
		refine[eqi], grade[eqi] = e.Refine, e.Grade
	}
	return refine, grade
}

// scriptBase is what every item, card and combo script of the build can read:
// levels, base stats, the class as rAthena sees it, learned skills and the
// other equipment positions.
func scriptBase(b Build, c *gamedata.Catalog, param map[string]int, equipIDs map[int]bool, slotRefine, slotGrade map[string]int) script.Context {
	ctx := script.Context{BaseLv: b.BaseLv, JobLv: b.JobLv, Param: param, Class: b.Job, EquipIDs: equipIDs,
		SlotRefine: slotRefine, SlotGrade: slotGrade, SlotItem: map[string]int{}, Skills: map[string]int{}}
	for eqi, slot := range eqiSlots {
		ctx.SlotItem[eqi] = b.Equip[slot].Item
	}
	for id, lv := range b.Skills {
		if s := c.Skills[id]; s != nil {
			ctx.Skills[s.Code] = lv
		}
	}
	ctx.EAClass, ctx.BaseJob, ctx.BaseClass, ctx.Upper = classInfo(c, b.Job)
	ctx.ItemInfo = func(id int, info string) (int, bool) {
		it := c.Items[id]
		if it == nil {
			return 0, id == 0 // an empty position reads as 0
		}
		switch info {
		case "VIEW":
			if it.Type == "Weapon" {
				v, ok := script.WeaponTypes[it.SubType]
				return v, ok
			}
		case "WEAPONLEVEL":
			return it.WeaponLv, true
		case "ARMORLEVEL":
			return it.ArmorLv, true
		}
		return 0, false
	}
	return ctx
}

// classInfo is a job as rAthena scripts see it: eaclass() (class line +
// EAJL flags), BaseJob (the line's 2nd class: Cavaleiro for a Cavaleiro
// Rúnico), BaseClass (its 1st class) and Upper (1 transcendent, 2 baby).
func classInfo(c *gamedata.Catalog, jobID int) (eaclass, baseJob, baseClass, upper int) {
	j := c.Jobs[jobID]
	if j == nil {
		return 0, 0, 0, 0
	}
	chain := c.Chain(jobID)
	// BaseJob: the deepest class of the line that is neither transcendent nor
	// 3rd/4th (Super Novice for an Hiperaprendiz, Invocador for an Animista).
	baseJob = chain[0]
	for _, id := range chain {
		x := c.Jobs[id]
		if x == nil || x.Upper || x.Fourth || strings.HasPrefix(x.EquipTier, "Third") || x.EquipTier == "Fourth" || x.Baby {
			continue
		}
		baseJob = id
	}
	// BaseClass: the 1st class (Novice for the Super Novice line).
	baseClass = chain[0]
	if chain[0] == 0 && len(chain) > 1 { // lines that start as Aprendiz
		baseClass = chain[1]
	}
	if code := strings.TrimSuffix(c.Jobs[baseJob].Code, "_H"); code == "JT_SUPERNOVICE" || code == "JT_NOVICE" {
		baseClass = 0
	}
	if baseJob == 20 { // Bards and Dancers share one rAthena line
		eaclass = script.EAJLine(19)
	} else {
		eaclass = script.EAJLine(baseJob)
	}
	switch {
	case j.Baby:
		upper = 2
		eaclass |= script.EAJLBaby
	case j.Upper:
		upper = 1
		eaclass |= script.EAJLUpper
	}
	if strings.HasPrefix(j.EquipTier, "Third") || j.EquipTier == "Fourth" || j.Fourth {
		eaclass |= script.EAJLThird
	}
	if j.EquipTier == "Fourth" || (j.Fourth && strings.Contains(j.Code, "SPIRIT_HANDLER")) {
		eaclass |= script.EAJLFourth
	}
	return eaclass, baseJob, baseClass, upper
}
