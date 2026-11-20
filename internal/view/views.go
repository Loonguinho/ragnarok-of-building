package view

import (
	"context"
	"strings"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// Views is the simulator's window contents, for the interface (internal/ui).
type Views struct{ s *Server }

// The view types.
type (
	BuildView    = buildView
	SlotView     = slotView
	ItemView     = itemView
	OptLine      = optLine
	BonusLine    = bonusLine
	ComboView    = comboView
	DamageView   = damageView
	AtkSkill     = atkSkill
	SkillRow     = skillRow
	BuffsView    = buffsView
	BuffGroup    = buffGroup
	BuffItem     = buffItem
	EffectItem   = effectItem
	LearnItem    = learnItem
	JobOption    = jobOption
	ClassFamily  = classFamily
	SkillTab     = skillTab
	SkillCell    = skillCell
	SkillInfo    = skillJS
	StatLine     = statLine
	SwapView     = swapView
	OptResult    = optResult
	OptGroup     = optGroup
	OptOption    = optOption
	ReplayDamage = rdmgView
	ReplayChar   = rdmgChar
	ReplaySkill  = rdmgSkill
	ReplayMob    = rdmgMob
)

// SlotInfo is one Equipment window slot: key, label (Portuguese, translate
// with Tr) and side ("L" or "R").
type SlotInfo = slotInfo

// EquipSlots, ShadowSlots and CostumeSlots are the Equipment window's tabs.
func EquipSlots() []SlotInfo   { return equipSlots }
func ShadowSlots() []SlotInfo  { return shadowSlots }
func CostumeSlots() []SlotInfo { return costumeSlots }

// Catalog is the game data the views read.
func (v *Views) Catalog() *gamedata.Catalog { return v.s.opt.Catalog }

// Build is the whole build page: Normalize + Compute and every window's
// contents. The build in the result is the normalized one.
func (v *Views) Build(b calc.Build, l string) BuildView {
	note := v.s.loadTarget(b.Target, l)
	bv := v.s.buildView(b, l)
	if note != "" {
		bv.Notes = append(bv.Notes, note)
	}
	return bv
}

// SkillTree is the Habilidades window for a job: one tab per class step,
// the per-skill rules (max level, prerequisites, point group) and the
// skill point groups.
type SkillTree struct {
	Tabs   []SkillTab
	Skills map[int]SkillInfo
	Groups []gamedata.SkillGroup
}

// SkillTree returns the tree of jobID at jobLv; ok false for an unknown job.
func (v *Views) SkillTree(jobID, jobLv int, l string) (SkillTree, bool) {
	d := v.s.skillTreeData(jobID, jobLv, l, nil)
	if d == nil {
		return SkillTree{}, false
	}
	t := d["Tree"].(map[string]any)
	return SkillTree{Tabs: d["Tabs"].([]skillTab), Skills: t["skills"].(map[int]skillJS),
		Groups: t["groups"].([]gamedata.SkillGroup)}, true
}

// DefaultBuild is the build a new page starts with.
func (v *Views) DefaultBuild() calc.Build { return defaultBuild(v.s.opt.Catalog) }

// NewBuild is a blank build for a class, at its max levels.
func (v *Views) NewBuild(jobID int) calc.Build {
	if j := v.s.opt.Catalog.Jobs[jobID]; j != nil && j.HasStats() {
		return calc.Build{Job: j.ID, BaseLv: j.MaxBaseLv, JobLv: j.MaxJobLv, Stats: [6]int{1, 1, 1, 1, 1, 1}}
	}
	return v.DefaultBuild()
}

// PickItems lists what the item picker offers (see pickItems); with a
// target, gains has each result's expected damage change.
func (v *Views) PickItems(b calc.Build, slot string, card bool, idx int, q, l string) (items []ItemView, gains map[int]StatLine) {
	item := 0
	if e, ok := b.Equip[slot]; ok {
		item = e.Item
	}
	items = v.s.pickItems(slot, card, idx, item, b.Job, q, l)
	cardIdx := -1
	if card {
		cardIdx = idx
	}
	if slot != "" {
		gains = v.s.pickGains(b, slot, items, cardIdx, l)
	}
	return items, gains
}

// Swap puts item id in the slot (card >= 0: in that card position), with
// the equipment rules: linked headgear slots, two-handed weapons, refine
// and cards carried over.
func (v *Views) Swap(b calc.Build, slot string, id, card int) calc.Build {
	return v.s.swapItem(b, slot, id, card)
}

// SwapCompare is the item tooltip's comparison: what putting item id in the
// slot (or card position card >= 0) changes.
func (v *Views) SwapCompare(b calc.Build, slot string, id, card int, l string) SwapView {
	c := v.s.opt.Catalog
	cur := b.Equip[slot]
	curName := ""
	if ci := c.Items[cur.Item]; ci != nil && card < 0 {
		curName = ci.Name.Get(l)
	}
	if card >= 0 {
		if cc := c.Items[cur.Cards[min(card, 3)]]; cc != nil {
			curName = cc.Name.Get(l)
		}
	}
	return v.s.swapCompare(b, v.s.swapItem(b, slot, id, card), curName, l)
}

// Item is one item as the tooltips and lists show it (nil when unknown).
func (v *Views) Item(id int, l string) *ItemView {
	it := v.s.opt.Catalog.Items[id]
	if it == nil {
		return nil
	}
	iv := viewItem(it, l)
	return &iv
}

// ItemTypes are the item database filters (value, Portuguese label).
func ItemTypes() [][2]string { return itemTypes }

// SearchItems is the item database search.
func (v *Views) SearchItems(q, typ, l string) []ItemView { return v.s.searchItems(q, typ, l) }

// MonsterRow is one monster picker result, with translated race and element.
type MonsterRow struct {
	M             *gamedata.Monster
	Name          string
	Race, Element string
}

// SearchMonsters is the monster picker.
func (v *Views) SearchMonsters(q, l string) []MonsterRow {
	var out []MonsterRow
	for _, m := range v.s.opt.Catalog.SearchMonsters(q, 40) {
		out = append(out, MonsterRow{M: m, Name: m.NameIn(l), Race: tr(l, racePT(m.Race)), Element: tr(l, elementPT[m.Element])})
	}
	return out
}

// Optimize runs the optimizer: items, cards and enchants, one swap at a
// time, ranked by the goal (it stops itself after a while).
func (v *Views) Optimize(ctx context.Context, b calc.Build, o OptOptions, l string) OptResult {
	return v.s.optimizeWith(ctx, b, o, l)
}

// ImportReplay reads a .rrf: its build, notes and damage window (nil when
// the recording has no damage data).
func (v *Views) ImportReplay(buf []byte, l string) (calc.Build, []string, *ReplayDamage, error) {
	return v.s.importReplay(buf, l)
}

// ClassFamilies is the class picker, grouped by first class.
func (v *Views) ClassFamilies(l string, current int) []ClassFamily {
	return v.s.classFamilies(l, current)
}

// RandomOpts lists the random options an item position can roll, sorted
// by name (Min, Max 0: range unknown).
func (v *Views) RandomOpts(item, pos int, l string) []OptLine {
	return randomOptList(v.s.opt.Catalog, item, pos, l)
}

// CreatePaste stores the build code (and an optional text) on paste.rs and
// returns the link.
func (v *Views) CreatePaste(ctx context.Context, code, text string) (string, error) {
	return v.s.createPaste(ctx, code, text)
}

// ReadPaste returns the build code in a paste.rs link.
func (v *Views) ReadPaste(ctx context.Context, link string) (string, error) {
	code, _, err := v.s.readPaste(ctx, link)
	return code, err
}

// FindBuildCode finds a build code in pasted text (a link or a bare code).
func FindBuildCode(text string) string { return findBuildCode([]byte(strings.TrimSpace(text))) }

// EncodeBuild and DecodeBuild are the share codes.
func EncodeBuild(b calc.Build) string            { return encodeBuild(b) }
func DecodeBuild(code string) (calc.Build, bool) { return decodeBuild(code) }

// Tr translates an interface text (Portuguese key) to the language l.
func Tr(l, s string) string { return tr(l, s) }

// FmtInt formats with the simulator's thousands separator.
func FmtInt(n int64) string { return fmtInt(n) }

// Langs are the interface languages: code and name, in selector order.
func Langs() [][2]string {
	out := make([][2]string, len(uiLangs))
	for i, l := range uiLangs {
		out[i] = [2]string{l.Code, l.Label}
	}
	return out
}
