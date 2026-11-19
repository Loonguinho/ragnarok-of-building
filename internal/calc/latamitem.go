package calc

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

// LATAM item scripts: latam-ro-calc (MIT) keeps, for every item, its
// bonuses in a notation of its own (item.json):
//
//	"atk": ["130", "2---15", "EQUIP_ID[480091]===50"]
//
// 130 flat, +15 every 2 refines, +50 with item 480091 equipped. rAthena's
// scripts are kRO's and sometimes not LATAM's (Escudo Ilusión B), but
// latam-ro-calc's are incomplete for many older items, so neither wins
// outright: DecideLatamKeys takes, bonus by bonus, the LATAM value only
// where the client's PT description agrees with it and not with rAthena
// (Item.LatamKeys). Sets stay with the item combos (rAthena's and the
// descriptions'), so the LATAM script's own set lines are not used. The
// condition and value rules below are a port of calculator.ts
// (validateCondition, calcConstantBonus, calcStepBonus).

// latamEnv is what the conditions of a build's items read.
type latamEnv struct {
	b          Build
	c          *gamedata.Catalog
	equipped   map[int]bool    // items, cards and enchants
	names      map[string]bool // LatamName of everything equipped
	nameRefine map[string]int  // LatamName -> refine (main items)
	learned    map[string]int  // normName(English skill name) -> learned level
	chain      []string        // job codes of the class line
	weapon     *gamedata.Item
	combos     map[string]bool // EQUIP[...] lines already counted (isAreadyCalcCombo)
	today      string          // yyyy-mm-dd in São Paulo, for UNTIL[...]
}

// latamSlots: latam-ro-calc's positions. Its accLeft is the right
// accessory (replay-to-model.ts: EQP_ACC_R -> accLeft).
var latamSlots = map[string]string{
	"weapon": Weapon, "leftWeapon": Shield, "shield": Shield, "headUpper": HeadTop, "headMiddle": HeadMid,
	"headLower": HeadLow, "armor": Armor, "garment": Garment, "boot": Shoes, "accLeft": AccRight, "accRight": AccLeft,
	"shadowWeapon": ShWeapon, "shadowArmor": ShArmor, "shadowShield": ShShield, "shadowBoot": ShShoes,
	"shadowEarring": ShAccR, "shadowPendant": ShAccL, "costumeUpper": CosTop, "costumeMiddle": CosMid,
	"costumeLower": CosLow, "costumeGarment": CosGarment,
}

var latamPosOf = func() map[string]string {
	m := map[string]string{}
	for k, v := range latamSlots {
		if k != "leftWeapon" {
			m[v] = k
		}
	}
	return m
}()

// latamWeaponType: our weapon subtype -> latam-ro-calc's typeName and subTypeName.
var latamWeaponType = map[string][2]string{
	"Dagger": {"dagger", "Dagger"}, "1hSword": {"sword", "Sword"}, "2hSword": {"twohandSword", "Two-Handed Sword"},
	"1hSpear": {"spear", "Spear"}, "2hSpear": {"twohandSpear", "Two-Handed Spear"}, "1hAxe": {"axe", "Axe"},
	"2hAxe": {"twohandAxe", "Two-Handed Axe"}, "Mace": {"mace", "Mace"}, "2hMace": {"twohandMace", "Two-Handed Mace"},
	"Staff": {"rod", "Rod"}, "2hStaff": {"twohandRod", "Two-Handed Rod"}, "Bow": {"bow", "Bow"},
	"Knuckle": {"fist", "Fistweapon"}, "Musical": {"instrument", "Instrument"}, "Whip": {"whip", "Whip"},
	"Book": {"book", "Book"}, "Katar": {"katar", "Katar"}, "Revolver": {"gun", "Revolver"}, "Rifle": {"gun", "Rifle"},
	"Gatling": {"gun", "Gatling Gun"}, "Shotgun": {"gun", "Shotgun"}, "Grenade": {"gun", "Grenade Launcher"},
	"Huuma": {"shuriken", "Shuriken"},
}

// latamClassAlias: USED[...] class names that are not a job code's words.
var latamClassAlias = map[string]string{
	"Mage": "JT_MAGICIAN", "Taekwondo": "JT_TAEKWON", "Taekwon": "JT_TAEKWON", "Doram": "JT_DO_SUMMONER",
	"Summoner": "JT_DO_SUMMONER", "SoulLinker": "JT_LINKER", "AssassinCross": "JT_ASSASSIN_H", "Champion": "JT_MONK_H",
	"Stalker": "JT_ROGUE_H", "Gypsy": "JT_DANCER_H", "Clown": "JT_BARD_H", "LordKnight": "JT_KNIGHT_H",
	"HighPriest": "JT_PRIEST_H", "HighWizard": "JT_WIZARD_H", "Whitesmith": "JT_BLACKSMITH_H", "Sniper": "JT_HUNTER_H",
	"Paladin": "JT_CRUSADER_H", "Professor": "JT_SAGE_H", "Creator": "JT_ALCHEMIST_H", "StarGladiator": "JT_STAR",
}

var saoPaulo = time.FixedZone("BRT", -3*3600) // no daylight saving since 2019

func newLatamEnv(b Build, c *gamedata.Catalog) *latamEnv {
	env := &latamEnv{b: b, c: c, equipped: map[int]bool{}, names: map[string]bool{}, nameRefine: map[string]int{},
		learned: map[string]int{}, combos: map[string]bool{}, today: time.Now().In(saoPaulo).Format("2006-01-02")}
	for slot, e := range b.Equip {
		if it := c.Items[e.Item]; it != nil {
			env.equipped[e.Item] = true
			if it.LatamName != "" {
				env.names[it.LatamName] = true
				env.nameRefine[it.LatamName] = e.Refine
			}
			if slot == Weapon {
				env.weapon = it
			}
		}
		for _, cd := range e.Cards {
			if ci := c.Items[cd]; cd > 0 && ci != nil {
				env.equipped[cd] = true
				if ci.LatamName != "" {
					env.names[ci.LatamName] = true
				}
			}
		}
	}
	for id, lv := range b.Skills {
		if s := c.Skills[id]; s != nil {
			env.learned[normName(s.Name.Get("en"))] = lv
		}
	}
	for _, id := range c.Chain(b.Job) {
		if j := c.Jobs[id]; j != nil {
			env.chain = append(env.chain, j.Code)
		}
	}
	return env
}

// model reads latam-ro-calc's model fields: base (allocated) stats and levels.
func (e *latamEnv) model(name string) float64 {
	switch name {
	case "level":
		return float64(e.b.BaseLv)
	case "jobLevel":
		return float64(e.b.JobLv)
	}
	for i, n := range []string{"str", "agi", "vit", "int", "dex", "luk"} {
		if n == name {
			return float64(e.b.Stats[i])
		}
	}
	for i, n := range []string{"pow", "sta", "wis", "spl", "con", "crt"} {
		if n == name {
			return float64(e.b.Traits[i])
		}
	}
	return 0
}

func (e *latamEnv) slotRefine(pos string) int { return e.b.Equip[latamSlots[pos]].Refine }

func (e *latamEnv) skillByID(id int) int { return e.b.Skills[id] }

// isClass: USED[...] matches the class or one of its line (any version).
func (e *latamEnv) isClass(name string) bool {
	if code, ok := latamClassAlias[name]; ok {
		for _, c := range e.chain {
			if c == code {
				return true
			}
		}
		return false
	}
	flat := strings.ToUpper(strings.NewReplacer(" ", "", "_", "").Replace(name))
	for _, c := range e.chain {
		f := strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(c, "JT_"), "_H"), "_", "")
		if f == flat {
			return true
		}
	}
	return false
}

func (e *latamEnv) weaponNames() [2]string {
	if e.weapon == nil || e.weapon.Type != "Weapon" {
		return [2]string{}
	}
	return latamWeaponType[e.weapon.SubType]
}

// jsNum is JavaScript's Number(s): "" is 0, anything not a number is NaN (ok false).
func jsNum(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, true
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// jsTruthy: a number that is neither 0 nor NaN.
func jsTruthy(s string) (float64, bool) {
	v, ok := jsNum(s)
	return v, ok && v != 0
}

var (
	reLtMainStat  = regexp.MustCompile(`^(str|int|dex|agi|vit|luk|level):(\d+)&&(\d+===.+)`)
	reLtUntil     = regexp.MustCompile(`UNTIL\[(\d{4}-\d{2}-\d{2})]`)
	reLtWeaponLv  = regexp.MustCompile(`WEAPON_LEVEL\[(\d+)]`)
	reLtLoyalty   = regexp.MustCompile(`LOYALTY\[[^\]]*]`)
	reLtWeaponTy  = regexp.MustCompile(`WEAPON_TYPE\[(\D+)]`)
	reLtGrade     = regexp.MustCompile(`GRADE\[(\D+?)]`)
	reLtGrades    = regexp.MustCompile(`GRADES\[(\D+?)]`)
	reLtSubType   = regexp.MustCompile(`^\[weaponType=(.+?)\]`)
	reLtSumGate   = regexp.MustCompile(`(SUM\[(\D+)==(\d+)])[^-]`)
	reLtUsed      = regexp.MustCompile(`USED\[(.+?)\]`)
	reLtLearn     = regexp.MustCompile(`(LEARN_SKILL\[(.+?)\]=?=?=?)-?\d+`)
	reLtLearn2    = regexp.MustCompile(`(LEARN_SKILL2\[(.+?)\]=?=?=?)`)
	reLtSkillID2  = regexp.MustCompile(`(SKILL_ID2\[(.+?)\]=?=?=?)`)
	reLtLevel     = regexp.MustCompile(`LEVEL\[(.+?)\]`)
	reLtActiveID  = regexp.MustCompile(`ACTIVE_SKILL_ID\[(\d+)]`)
	reLtActive    = regexp.MustCompile(`ACTIVE_SKILL\[(.+)]`)
	reLtXRefine   = regexp.MustCompile(`XREFINEX\[(\D*?)=*=*(\d+)]`)
	reLtPos       = regexp.MustCompile(`POS\[(\D+?)]`)
	reLtItemLv    = regexp.MustCompile(`ITEM_LV\[(.+?)]`)
	reLtMonster   = regexp.MustCompile(`^MONSTER_ID\[([\d|]+)]`)
	reLtSpawn     = regexp.MustCompile(`SPAWN\[(.+?)]`)
	reLtPosSpec   = regexp.MustCompile(`POS_SPECIFIC\[(\D+?)]`)
	reLtEquipID   = regexp.MustCompile(`^EQUIP_ID\[([\d|&]+)]`)
	reLtAmmo      = regexp.MustCompile(`^AMMO_SUBTYPE\[(\d+)]`)
	reLtEquip     = regexp.MustCompile(`^EQUIP\[(.+?)]`)
	reLtSlotRef   = regexp.MustCompile(`^REFINE\[(\D+?)==(\d+)(?:\(\d+\))?]`)
	reLtRefine    = regexp.MustCompile(`(REFINE\[(\d+)?])[^-]`)
	reLtRefFrom   = regexp.MustCompile(`^REFINE_FROM\[(\d+)]$`)
	reLtStepLearn = regexp.MustCompile(`LEARN_SKILL\[(.+?)==(\d+)]`)
	reLtStatRange = regexp.MustCompile(`(level|jobLevel|str|int|dex|agi|vit|luk):(-*\d+)\((.+)\)`)
	reLtStat      = regexp.MustCompile(`(level|jobLevel|str|int|dex|agi|vit|luk):(-*\d+)`)
	reLtStepSum   = regexp.MustCompile(`SUM\[(\D+)==(\d+)]`)
	reLtStepRef   = regexp.MustCompile(`^REFINE\[(\D*?)=*=*(\d+)(?:\((\d+)\))?]`)
	reLtRefName   = regexp.MustCompile(`^REFINE_NAME\[(\D+)==(\d+)?]`)
	reLtGValue    = regexp.MustCompile(`GVALUE\[(\D+)==(\d+)]`)
)

// skillIDGate finds SKILL_ID[...] not preceded by ACTIVE_ (Go regexps have
// no lookbehind) followed by ]=?=?=?-?\d+, and returns the clause to remove
// and the "id==lv" inside.
func skillIDGate(s string, needValue bool) (clause, cond string) {
	for from := 0; ; {
		i := strings.Index(s[from:], "SKILL_ID[")
		if i < 0 {
			return "", ""
		}
		i += from
		from = i + 1
		if strings.HasSuffix(s[:i], "ACTIVE_") {
			continue
		}
		end := strings.Index(s[i:], "]")
		if end < 0 {
			return "", ""
		}
		end += i
		j := end + 1
		for k := 0; k < 3 && j < len(s) && s[j] == '='; k++ {
			j++
		}
		if needValue {
			k := j
			if k < len(s) && s[k] == '-' {
				k++
			}
			if k >= len(s) || s[k] < '0' || s[k] > '9' {
				continue
			}
		}
		return s[i:j], s[i+len("SKILL_ID[") : end]
	}
}

func gradeValue(g string) int {
	switch strings.ToUpper(g) {
	case "D":
		return 1
	case "C":
		return 2
	case "B":
		return 3
	case "A":
		return 4
	}
	return 0
}

func trimEq(s string) string {
	if strings.HasPrefix(s, "===") {
		return strings.Replace(s, "===", "", 1)
	}
	return s
}

// valid is validateCondition: whether a line applies, and what is left of it.
// pos is the latam position of the item (of the item a card is in), main
// whether the line belongs to that item itself.
func (e *latamEnv) valid(pos string, main bool, refine int, s string) (bool, string) {
	rest := s
	if m := reLtMainStat.FindStringSubmatch(rest); m != nil {
		v, _ := jsNum(m[2])
		return e.model(m[1]) >= v, m[3]
	}
	if m := reLtUntil.FindStringSubmatch(rest); m != nil {
		if e.today > m[1] {
			return false, rest
		}
		rest = strings.Replace(rest, m[0], "", 1)
		if strings.HasPrefix(rest, "===") {
			return true, trimEq(rest)
		}
	}
	if m := reLtWeaponLv.FindStringSubmatch(rest); m != nil {
		w := e.weapon
		if strings.HasPrefix(strings.ToLower(pos), "left") {
			if sh := e.c.Items[e.b.Equip[Shield].Item]; sh != nil {
				w = sh
			}
		}
		lv, _ := strconv.Atoi(m[1])
		if w == nil || w.WeaponLv != lv {
			return false, rest
		}
		rest = strings.Replace(rest, m[0], "", 1)
		if strings.HasPrefix(rest, "===") {
			return true, trimEq(rest)
		}
	}
	if reLtLoyalty.MatchString(rest) {
		return false, rest // pets are not simulated
	}
	if m := reLtWeaponTy.FindStringSubmatch(rest); m != nil {
		if !containsStr(strings.Split(m[1], "||"), e.weaponNames()[0]) {
			return false, rest
		}
		rest = strings.Replace(rest, m[0], "", 1)
		if strings.HasPrefix(rest, "===") {
			return true, trimEq(rest)
		}
	}
	if strings.Contains(rest, "GRADE") {
		for i := 0; i < 2; i++ {
			m := reLtGrade.FindStringSubmatch(rest)
			if m == nil {
				break
			}
			part := strings.SplitN(m[1], "==", 2)
			if len(part) < 2 || !e.gradeAtLeast(pos, part[0], part[1]) {
				return false, rest
			}
			rest = strings.Replace(rest, m[0], "", 1)
			if strings.HasPrefix(rest, "===") {
				return true, trimEq(rest)
			}
			if !strings.Contains(rest, "GRADE[") {
				break
			}
		}
		if m := reLtGrades.FindStringSubmatch(rest); m != nil {
			for _, g := range strings.Split(m[1], "&&") {
				part := strings.SplitN(g, "==", 2)
				if len(part) < 2 || !e.gradeAtLeast(pos, part[0], part[1]) {
					return false, rest
				}
			}
			rest = strings.Replace(rest, m[0], "", 1)
			if strings.HasPrefix(rest, "===") {
				return true, trimEq(rest)
			}
		}
	}
	if m := reLtSubType.FindStringSubmatch(rest); m != nil {
		if m[1] != e.weaponNames()[1] {
			return false, rest
		}
		return true, strings.Replace(rest, m[0], "", 1)
	}
	if m := reLtSumGate.FindStringSubmatch(rest); m != nil {
		total := 0.0
		for _, st := range strings.Split(m[2], ",") {
			total += e.model(st)
		}
		if v, _ := jsNum(m[3]); total < v {
			return false, rest
		}
		rest = strings.Replace(rest, m[1], "", 1)
	}
	if m := reLtUsed.FindStringSubmatch(rest); m != nil {
		ok := false
		for _, cl := range strings.Split(m[1], "||") {
			if e.isClass(cl) {
				ok = true
				break
			}
		}
		if !ok {
			return false, rest
		}
		rest = strings.Replace(rest, m[0], "", 1)
	}
	if m := reLtLearn.FindStringSubmatch(rest); m != nil {
		part := strings.SplitN(m[2], "==", 2)
		need := 0.0
		if len(part) == 2 {
			need, _ = jsNum(part[1])
		}
		if float64(e.learned[normName(part[0])]) < need || len(part) < 2 {
			return false, rest
		}
		rest = strings.Replace(rest, m[1], "", 1)
	}
	if clause, cond := skillIDGate(rest, true); clause != "" {
		if !e.skillIDAtLeast(cond) {
			return false, rest
		}
		rest = strings.Replace(rest, clause, "", 1)
	}
	if m := reLtLearn2.FindStringSubmatch(rest); m != nil {
		part := strings.SplitN(m[2], "==", 2)
		need := 0.0
		if len(part) == 2 {
			need, _ = jsNum(part[1])
		}
		if len(part) < 2 || float64(e.learned[normName(part[0])]) < need {
			return false, rest
		}
		rest = strings.Replace(rest, m[1], "", 1)
	}
	if m := reLtSkillID2.FindStringSubmatch(rest); m != nil {
		if !e.skillIDAtLeast(m[2]) {
			return false, rest
		}
		rest = strings.Replace(rest, m[1], "", 1)
	}
	if m := reLtLevel.FindStringSubmatch(rest); m != nil {
		lo, hi := 0, 999
		part := strings.SplitN(m[1], "-", 2)
		lo, _ = strconv.Atoi(part[0])
		if len(part) == 2 {
			hi, _ = strconv.Atoi(part[1])
		}
		if e.b.BaseLv < lo || e.b.BaseLv > hi {
			return false, rest
		}
		rest = strings.Replace(rest, m[0], "", 1)
	}
	// Bonuses while a skill is active: buffs are not tracked per item here.
	if reLtActiveID.MatchString(rest) || reLtActive.MatchString(rest) {
		return false, rest
	}
	for i := 0; i < 3; i++ {
		m := reLtXRefine.FindStringSubmatch(rest)
		if m == nil {
			break
		}
		if n, _ := strconv.Atoi(m[2]); e.slotRefine(m[1]) < n {
			return false, rest
		}
		rest = strings.Replace(rest, m[0], "", 1)
	}
	if m := reLtPos.FindStringSubmatch(rest); m != nil && !strings.Contains(m[0], "POS_SPECIFIC") {
		if !main || m[1] != pos {
			return false, rest
		}
		rest = strings.Replace(rest, m[0], "", 1)
		if strings.HasPrefix(rest, "===") {
			return true, rest
		}
	}
	if m := reLtItemLv.FindStringSubmatch(rest); m != nil {
		part := strings.SplitN(m[1], "==", 2)
		p := part[0]
		if p == "me" {
			p = pos
		}
		lv := -1
		if len(part) == 2 {
			lv, _ = strconv.Atoi(part[1])
		}
		it := e.c.Items[e.b.Equip[latamSlots[p]].Item]
		if it == nil || max(it.ArmorLv, it.WeaponLv) != lv {
			return false, rest
		}
		rest = strings.Replace(rest, m[0], "", 1)
		if strings.HasPrefix(rest, "===") {
			return true, rest
		}
	}
	// Bonuses against listed monsters or maps: item bonuses are target-free here.
	if reLtMonster.MatchString(rest) || reLtSpawn.MatchString(rest) {
		return false, rest
	}
	if m := reLtPosSpec.FindStringSubmatch(rest); m != nil {
		part := strings.SplitN(m[1], "==", 2)
		it := e.c.Items[e.b.Equip[latamSlots[part[0]]].Item]
		if len(part) < 2 || it == nil || it.LatamName != part[1] {
			return false, rest
		}
		rest = strings.Replace(rest, m[0], "", 1)
		if strings.HasPrefix(rest, "===") {
			return true, rest
		}
	}
	if m := reLtEquipID.FindStringSubmatch(rest); m != nil {
		for _, group := range strings.Split(m[1], "&&") {
			if group == "" {
				continue
			}
			any := false
			for _, id := range strings.Split(group, "||") {
				if n, err := strconv.Atoi(id); err == nil && e.equipped[n] {
					any = true
					break
				}
			}
			if !any {
				return false, rest
			}
		}
		return e.valid(pos, main, refine, trimEq(strings.Replace(rest, m[0], "", 1)))
	}
	if reLtAmmo.MatchString(rest) {
		return false, rest // ammunition is not simulated
	}
	if m := reLtEquip.FindStringSubmatch(rest); m != nil {
		for _, group := range strings.Split(m[1], "&&") {
			if group == "" {
				continue
			}
			any := false
			for _, n := range strings.Split(group, "||") {
				if e.names[n] {
					any = true
					break
				}
			}
			if !any {
				return false, rest
			}
		}
		return e.valid(pos, main, refine, trimEq(strings.Replace(rest, m[0], "", 1)))
	}
	if m := reLtSlotRef.FindStringSubmatch(rest); m != nil {
		if strings.Contains(rest, m[0]+"---") {
			return true, rest
		}
		total := 0
		for _, p := range strings.Split(m[1], ",") {
			total += e.slotRefine(p)
		}
		if n, _ := strconv.Atoi(m[2]); total < n {
			return false, rest
		}
		return e.valid(pos, main, refine, trimEq(strings.Replace(rest, m[0], "", 1)))
	}
	if m := reLtRefine.FindStringSubmatch(rest); m != nil && m[2] != "" {
		if n, _ := strconv.Atoi(m[2]); refine >= n {
			return e.valid(pos, main, refine, trimEq(strings.Replace(rest, m[1], "", 1)))
		}
		return false, rest
	}
	return true, rest
}

func (e *latamEnv) gradeAtLeast(pos, slot, want string) bool {
	if slot == "me" {
		slot = pos
	}
	g := e.b.Equip[latamSlots[slot]].Grade
	return g > 0 && gradeValue(want) > 0 && g >= gradeValue(want)
}

func (e *latamEnv) skillIDAtLeast(cond string) bool {
	part := strings.SplitN(cond, "==", 2)
	id, _ := strconv.Atoi(part[0])
	need := 0.0
	if len(part) == 2 {
		need, _ = jsNum(part[1])
	}
	return float64(e.skillByID(id)) >= need
}

// constant is calcConstantBonus ("N===V": V from refine N on).
func (e *latamEnv) constant(refine int, s string) float64 {
	parts := strings.Split(s, "===")
	cond, bonus := parts[0], ""
	if len(parts) > 1 {
		bonus = parts[1]
	}
	condNum, condOK := jsTruthy(cond)
	if condOK && float64(refine) < condNum {
		return 0
	}
	if b, ok := jsTruthy(bonus); condOK && ok {
		return b
	}
	if sc := strings.Split(cond, ":"); len(sc) > 1 && sc[0] != "" && sc[1] != "" {
		if v, _ := jsNum(sc[1]); e.model(sc[0]) >= v {
			b, _ := jsNum(bonus)
			return b
		}
		return 0
	}
	if i := strings.Index(bonus, "("); i >= 0 {
		bonus = bonus[:i]
	}
	if v, ok := jsNum(bonus); ok {
		return v
	}
	return 0
}

// step is calcStepBonus ("N---V": V every N refines, stats, levels...).
func (e *latamEnv) step(refine int, s string) float64 {
	parts := strings.Split(s, "---")
	cond, bonusS := parts[0], ""
	if len(parts) > 1 {
		bonusS = parts[1]
	}
	bonus, bonusOK := jsNum(bonusS)
	if !bonusOK {
		bonus = math.NaN()
	}
	calc := func(actual, every float64) float64 {
		if every == 0 {
			return 0
		}
		return math.Floor(actual/every) * bonus
	}
	if m := reLtRefFrom.FindStringSubmatch(cond); m != nil {
		n, _ := strconv.Atoi(m[1])
		return float64(max(0, refine-n+1)) * bonus
	}
	if c, ok := jsTruthy(cond); ok && bonusOK && bonus != 0 {
		return math.Floor(float64(refine)/c) * bonus
	}
	if m := reLtStepLearn.FindStringSubmatch(cond); m != nil {
		every, _ := jsNum(m[2])
		return calc(float64(e.learned[normName(m[1])]), every)
	}
	if clause, c := skillIDGate(cond, false); clause != "" {
		part := strings.SplitN(c, "==", 2)
		id, _ := strconv.Atoi(part[0])
		every := 0.0
		if len(part) == 2 {
			every, _ = jsNum(part[1])
		}
		return calc(float64(e.skillByID(id)), every)
	}
	if m := reLtStatRange.FindStringSubmatch(cond); m != nil {
		r := strings.SplitN(m[3], "-", 2)
		lo, _ := jsNum(r[0])
		hi := 999.0
		if len(r) == 2 {
			hi, _ = jsNum(r[1])
		}
		every, _ := jsNum(m[2])
		return calc(math.Min(hi, e.model(m[1]))-lo+1, every)
	}
	if m := reLtStat.FindStringSubmatch(cond); m != nil {
		every, _ := jsNum(m[2])
		return calc(e.model(m[1]), every)
	}
	if m := reLtStepSum.FindStringSubmatch(cond); m != nil {
		total := 0.0
		for _, st := range strings.Split(m[1], ",") {
			total += e.model(st)
		}
		every, _ := jsNum(m[2])
		return calc(total, every)
	}
	if m := reLtStepRef.FindStringSubmatch(cond); m != nil {
		total := 0
		for _, p := range strings.Split(m[1], ",") {
			total += e.slotRefine(p)
		}
		if m[3] != "" {
			capN, _ := strconv.Atoi(m[3])
			total = min(total, capN)
		}
		every, _ := jsNum(m[2])
		return calc(float64(total), every)
	}
	if m := reLtRefName.FindStringSubmatch(cond); m != nil && m[2] != "" {
		total := 0
		for _, n := range strings.Split(m[1], ",") {
			total += e.nameRefine[n]
		}
		every, _ := jsNum(m[2])
		return calc(float64(total), every)
	}
	if m := reLtGValue.FindStringSubmatch(cond); m != nil {
		every, _ := jsNum(m[2])
		return calc(float64(e.b.Equip[latamSlots[m[1]]].Grade), every)
	}
	return 0
}

// line is calcScriptEntryValue.
func (e *latamEnv) line(pos string, main bool, refine int, s string) float64 {
	ok, rest := e.valid(pos, main, refine, s)
	if !ok {
		return 0
	}
	var v float64
	switch {
	case strings.Contains(rest, "==="):
		v = e.constant(refine, rest)
	case strings.Contains(rest, "---"):
		v = e.step(refine, rest)
	default:
		v, ok = jsNum(rest)
		if !ok {
			return 0
		}
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// eval sums an item's lines per bonus key (calcItemStatus). Set lines
// (EQUIP[...]/EQUIP_ID[...]) only with sets; then they count once per
// item and key, however many copies are worn.
func (e *latamEnv) eval(it *gamedata.Item, id int, pos string, main bool, refine int, sets bool) map[string]float64 {
	return e.evalScript(it.Latam, id, pos, main, refine, sets)
}

func (e *latamEnv) evalScript(src map[string][]string, id int, pos string, main bool, refine int, sets bool) map[string]float64 {
	out := map[string]float64{}
	for attr, lines := range src {
		sum := 0.0
		for _, l := range lines {
			if strings.HasPrefix(l, "EQUIP[") || strings.HasPrefix(l, "EQUIP_ID[") {
				if !sets {
					continue
				}
				k := fmt.Sprintf("%d-%s-%s", id, attr, l)
				if e.combos[k] {
					continue
				}
				e.combos[k] = true
			}
			sum += e.line(pos, main, refine, l)
		}
		if sum != 0 {
			out[attr] = sum
		}
	}
	return out
}

// ---- latam-ro-calc keys -> rAthena bonuses ----

type ltBonus struct {
	name  string
	scale float64 // value multiplier (-1: a reduction; -1000: seconds -> ms reduction)
}

var latamSimple = map[string]ltBonus{
	"atk": {"bBaseAtk", 1}, "matk": {"bMatk", 1}, "atkPercent": {"bAtkRate", 1}, "matkPercent": {"bMatkRate", 1},
	"melee": {"bShortAtkRate", 1}, "range": {"bLongAtkRate", 1}, "acd": {"bDelayrate", -1},
	"vct": {"bVariableCastrate", -1}, "fct": {"bFixedCast", -1000}, "fctPercent": {"bFixedCastrate", -1},
	"hp": {"bMaxHP", 1}, "hpPercent": {"bMaxHPrate", 1}, "sp": {"bMaxSP", 1}, "spPercent": {"bMaxSPrate", 1},
	"aspd": {"bAspd", 1}, "aspdPercent": {"bAspdRate", 1}, "cri": {"bCritical", 1}, "criDmg": {"bCritAtkRate", 1},
	"pAtk": {"bPAtk", 1}, "sMatk": {"bSMatk", 1}, "res": {"bRes", 1}, "mres": {"bMRes", 1}, "def": {"bDef", 1},
	"mdef": {"bMdef", 1}, "hit": {"bHit", 1}, "flee": {"bFlee", 1}, "perfectDodge": {"bFlee2", 1},
	"perfectHit": {"bPerfectHitAddRate", 1}, "str": {"bStr", 1}, "agi": {"bAgi", 1}, "vit": {"bVit", 1},
	"int": {"bInt", 1}, "dex": {"bDex", 1}, "luk": {"bLuk", 1}, "allStatus": {"bAllStats", 1}, "pow": {"bPow", 1},
	"sta": {"bSta", 1}, "wis": {"bWis", 1}, "spl": {"bSpl", 1}, "con": {"bCon", 1}, "crt": {"bCrt", 1},
	"allTrait": {"bAllTraitStats", 1}, "hplus": {"bHPlus", 1}, "cRate": {"bCRate", 1}, "healPower": {"bHealPower", 1},
	"healReceived": {"bHealPower2", 1}, "spCostPercent": {"bUseSPrate", 1}, "hpRecovRate": {"bHPrecovRate", 1},
	"spRecovRate": {"bSPrecovRate", 1}, "criRange": {"bCriticalLong", 1}, "ignore_size_penalty": {"bNoSizeFix", 1},
}

// latamTarget: prefix -> rAthena bonus2 and how the rest names its argument.
var latamTarget = []struct {
	prefix, name, kind string
}{
	{"p_pene_race_", "bIgnoreDefRaceRate", "race"}, {"p_pene_class_", "bIgnoreDefClassRate", "class"},
	{"m_pene_race_", "bIgnoreMdefRaceRate", "race"}, {"m_pene_class_", "bIgnoreMdefClassRate", "class"},
	{"pene_res_race_", "bIgnoreResRaceRate", "race"}, {"pene_mres_race_", "bIgnoreMresRaceRate", "race"},
	{"p_size_", "bAddSize", "size"}, {"p_race_", "bAddRace", "race"}, {"p_element_", "bAddEle", "ele"},
	{"p_class_", "bAddClass", "class"}, {"m_size_", "bMagicAddSize", "size"}, {"m_race_", "bMagicAddRace", "race"},
	{"m_my_element_", "bMagicAtkEle", "ele"}, {"m_element_", "bMagicAddEle", "ele"}, {"m_class_", "bMagicAddClass", "class"},
	{"subele_", "bSubEle", "ele"}, {"subrace_", "bSubRace", "race"}, {"subsize_", "bSubSize", "size"},
	{"subclass_", "bSubClass", "class"}, {"cri_race_", "bCriticalAddRace", "race"},
}

var latamArg = map[string]map[string]string{
	"race": {"all": "RC_All", "demihuman": "RC_DemiHuman", "player_human": "RC_Player_Human", "player_doram": "RC_Player_Doram",
		"formless": "RC_Formless", "undead": "RC_Undead", "brute": "RC_Brute", "plant": "RC_Plant", "insect": "RC_Insect",
		"fish": "RC_Fish", "demon": "RC_Demon", "angel": "RC_Angel", "dragon": "RC_Dragon"},
	"ele": {"all": "Ele_All", "neutral": "Ele_Neutral", "water": "Ele_Water", "earth": "Ele_Earth", "fire": "Ele_Fire",
		"wind": "Ele_Wind", "poison": "Ele_Poison", "holy": "Ele_Holy", "dark": "Ele_Dark", "ghost": "Ele_Ghost", "undead": "Ele_Undead"},
	"size":  {"all": "Size_All", "s": "Size_Small", "m": "Size_Medium", "l": "Size_Large"},
	"class": {"all": "Class_All", "normal": "Class_Normal", "boss": "Class_Boss"},
}

// latamSkillKeys: per-skill keys (<prefix><skill id>).
var latamSkillKeys = []struct {
	prefix, name string
	scale        float64
}{
	{"cd__", "bSkillCooldown", -1000}, {"vct__", "bVariableCastrate", -1}, {"fct__", "bSkillFixedCast", -1000},
}

// latamCovered: the rAthena bonuses the notation speaks for. An item with a
// LATAM script takes these from it alone.
var latamCovered = func() map[string]bool {
	m := map[string]bool{"bSkillAtk": true, "bAtk": true}
	for _, b := range latamSimple {
		m[b.name] = true
	}
	for _, t := range latamTarget {
		m[t.name] = true
	}
	for _, k := range latamSkillKeys {
		m[k.name] = true
	}
	return m
}()

// addLatam turns the summed keys into rAthena bonuses (Bonus / Extra keys
// as the script evaluator writes them).
func addLatam(c *gamedata.Catalog, vals map[string]float64, res *script.Result) {
	put := func(m map[string]int, k string, v float64) {
		if n := int(math.Round(v)); n != 0 {
			m[k] += n
		}
	}
	for attr, v := range vals {
		if b, ok := latamSimple[attr]; ok {
			if b.name == "bNoSizeFix" {
				if v > 0 {
					res.Bonus[b.name] = 1
				}
				continue
			}
			put(res.Bonus, b.name, v*b.scale)
			continue
		}
		if id, err := strconv.Atoi(attr); err == nil {
			if s := c.Skills[id]; s != nil {
				put(res.Extra, "bSkillAtk "+s.Code, v)
			}
			continue
		}
		done := false
		for _, k := range latamSkillKeys {
			if rest, ok := strings.CutPrefix(attr, k.prefix); ok {
				if id, err := strconv.Atoi(rest); err == nil && c.Skills[id] != nil {
					put(res.Extra, k.name+" "+c.Skills[id].Code, v*k.scale)
				}
				done = true
				break
			}
		}
		if done {
			continue
		}
		switch attr {
		case "pene_res":
			put(res.Extra, "bIgnoreResRaceRate RC_All", v)
			continue
		case "pene_mres":
			put(res.Extra, "bIgnoreMresRaceRate RC_All", v)
			continue
		}
		for _, t := range latamTarget {
			if rest, ok := strings.CutPrefix(attr, t.prefix); ok {
				if arg := latamArg[t.kind][rest]; arg != "" {
					put(res.Extra, t.name+" "+arg, v)
				}
				break
			}
		}
	}
}

// overLatam puts the bonuses an item's LATAM script won (LatamKeys) over
// its rAthena result. slot is where the item is (for a card or enchant,
// the item it is in).
func (e *latamEnv) overLatam(it *gamedata.Item, id int, slot string, main bool, refine int, res *script.Result) {
	if it == nil || len(it.LatamKeys) == 0 {
		return
	}
	if res.Bonus == nil {
		res.Bonus = map[string]int{}
	}
	if res.Extra == nil {
		res.Extra = map[string]int{}
	}
	lt := script.Result{Bonus: map[string]int{}, Extra: map[string]int{}}
	addLatam(e.c, e.eval(it, id, latamPosOf[slot], main, refine, false), &lt)
	for _, k := range it.LatamKeys {
		m, l := res.Bonus, lt.Bonus
		if strings.Contains(k, " ") {
			m, l = res.Extra, lt.Extra
		}
		delete(m, k)
		if v := l[k]; v != 0 {
			m[k] = v
		}
	}
}

// latamVoteSkip: bonuses never taken from the LATAM script. rAthena writes
// an armor's base MDEF as bMdef, which latam-ro-calc keeps out of the script.
var latamVoteSkip = map[string]bool{"bMdef": true}

// DescFunc returns the script read from an item's client description and
// how many of its lines were not understood (clientimport.DescScript).
type DescFunc func(id int, it *gamedata.Item) (script string, skipped int)

// DecideLatamKeys sets Item.LatamKeys: for every item with a LATAM script,
// the bonuses where it agrees with the PT description and rAthena does not
// (each evaluated over a range of refines). Taking a bonus away needs a
// description read whole. For items with no rAthena script (bonuses from
// the description only) the LATAM script fills in the bonuses the
// description does not give (lines it could not read, event text). Latam is dropped from items that won nothing.
// Returns how many items took something.
func DecideLatamKeys(c *gamedata.Catalog, desc DescFunc) int {
	n := 0
	refines := []int{0, 4, 5, 7, 9, 10, 11, 12, 13, 15}
	b := Build{BaseLv: 250, JobLv: 50, Stats: [6]int{100, 100, 100, 100, 100, 100}, Equip: map[string]Equip{}}
	env := newLatamEnv(b, c)
	param := map[string]int{"bStr": 100, "bAgi": 100, "bVit": 100, "bInt": 100, "bDex": 100, "bLuk": 100}
	for id, it := range c.Items {
		it.LatamKeys = nil
		if len(it.Latam) == 0 {
			continue
		}
		dsrc, skipped := desc(id, it)
		base, descOnly := it.Script, false
		if base == "" {
			base, descOnly = dsrc, true
		}
		rs := refines
		if !it.Refineable {
			rs = refines[:1]
		}
		type vec map[string][]int
		ra, lt, ds := vec{}, vec{}, vec{}
		put := func(v vec, i int, res script.Result) {
			add := func(k string, x int) {
				if v[k] == nil {
					v[k] = make([]int, len(rs))
				}
				v[k][i] = x
			}
			for k, x := range res.Bonus {
				if latamCovered[k] && !latamVoteSkip[k] {
					add(k, x)
				}
			}
			for k, x := range res.Extra {
				if name, _, _ := strings.Cut(k, " "); latamCovered[name] {
					add(k, x)
				}
			}
		}
		for i, r := range rs {
			ctx := script.Context{Refine: r, BaseLv: b.BaseLv, JobLv: b.JobLv, Param: param}
			put(ra, i, script.Eval(base, ctx))
			put(ds, i, script.Eval(dsrc, ctx))
			l := script.Result{Bonus: map[string]int{}, Extra: map[string]int{}}
			addLatam(c, env.eval(it, id, "", true, r, false), &l)
			put(lt, i, l)
		}
		zero := make([]int, len(rs))
		get := func(v vec, k string) []int {
			if v[k] == nil {
				return zero
			}
			return v[k]
		}
		keys := map[string]bool{}
		for k := range ra {
			keys[k] = true
		}
		for k := range lt {
			keys[k] = true
		}
		for k := range keys {
			r, l, d := get(ra, k), get(lt, k), get(ds, k)
			if slices.Equal(r, l) {
				continue
			}
			switch {
			case descOnly:
				// What the description does not say (unread lines, event
				// text) the LATAM script fills in.
				if slices.Equal(r, zero) {
					it.LatamKeys = append(it.LatamKeys, k)
				}
			case slices.Equal(l, d) && (skipped == 0 || !slices.Equal(l, zero)):
				it.LatamKeys = append(it.LatamKeys, k)
			}
		}
		if len(it.LatamKeys) == 0 {
			it.Latam = nil
			continue
		}
		slices.Sort(it.LatamKeys)
		n++
	}
	return n
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
