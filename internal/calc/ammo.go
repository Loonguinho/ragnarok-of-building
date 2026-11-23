package calc

import (
	"slices"
	"strings"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

// Ammunition (rAthena sd->bonus.arrow_*, latam-ro-calc getExtraAtk): its ATK
// joins the equipment ATK of the attacks that fire it, which also take its
// element; its other bonuses count while the weapon shoots it. A skill that
// throws kunai or cannonballs takes their ATK whatever the weapon.

// ammoWeapons: the weapons each kind of ammunition is shot with.
var ammoWeapons = map[string][]string{
	"Arrow":   {"Bow", "Musical", "Whip"},
	"Bullet":  {"Revolver", "Rifle", "Gatling", "Shotgun"},
	"Grenade": {"Grenade"},
}

// FiresAmmo reports whether the weapon type shoots the ammunition.
func FiresAmmo(it *gamedata.Item, weapon string) bool {
	return it != nil && slices.Contains(ammoWeapons[it.SubType], weapon)
}

func applyAmmo(r *Result, base script.Context, bonus, extra map[string]int) {
	if r.Ammo == nil {
		return
	}
	r.AmmoATK = r.Ammo.Attack
	r.AmmoFires = FiresAmmo(r.Ammo, r.WeaponType)
	res := script.Eval(itemScript(r.Ammo, &r.SkippedScriptEffects), base)
	for k, v := range res.Extra {
		if e, ok := strings.CutPrefix(k, "bAtkEle Ele_"); ok && v != 0 {
			r.AmmoEle = e
			delete(res.Extra, k)
		}
	}
	if r.AmmoFires {
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
		r.SkippedScriptEffects += res.Skipped
	}
}

// ammoSkills: the skills that throw ammunition whatever the weapon (rAthena
// skill_db Requires: Ammo, beyond arrows, bullets and grenades).
var ammoSkills = map[string]string{
	"NJ_SYURIKEN": "Shuriken", "NJ_KUNAI": "Kunai", "KO_HAPPOKUNAI": "Kunai",
	"SS_FUUMASHOUAKU": "Kunai", "SS_FUUMAKOUCHIKU": "Kunai", "SS_KUNAIWAIKYOKU": "Kunai", "SS_KUNAIKAITEN": "Kunai", "SS_KUNAIKUSSETSU": "Kunai",
	"AS_VENOMKNIFE": "Dagger", "NC_ARMSCANNON": "Cannonball", "GN_CARTCANNON": "Cannonball",
}

// usesAmmo reports whether a ranged attack with this skill ("" = basic
// attack) fires or throws the equipped ammunition.
func usesAmmo(r Result, skill string) bool {
	if r.Ammo == nil {
		return false
	}
	return r.AmmoFires || ammoSkills[skill] == r.Ammo.SubType && skill != ""
}

// ammoATK is the ammunition's part in a ranged attack: a cannonball counts as
// mastery ATK, after P.ATK (latam-ro-calc's cannonballAtk), the rest as
// equipment ATK.
func ammoATK(r Result, skill string) (equip, mastery int) {
	switch {
	case !usesAmmo(r, skill):
		return 0, 0
	case r.Ammo.SubType == "Cannonball":
		return 0, r.AmmoATK
	}
	return r.AmmoATK, 0
}
