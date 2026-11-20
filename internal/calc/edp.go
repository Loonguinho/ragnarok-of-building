package calc

// Encantar com Veneno Mortal (ASC_EDP), as latam-ro-calc measured it on
// LATAM recordings (sc-edp-dummy-elements.rrf, sc-edp-element-states.rrf):
//
//   - weapon ATK +25%, and only that share takes the poison element against
//     the target (iROwiki's reading; the rest keeps the weapon's element);
//   - the weapon and equipment ATK after the card modifiers is ×4 (the
//     %ATK share is not);
//   - the status ATK takes no endow while it is up.
//
// Every physical attack gets it except Meteor Assault. The client lists it
// as a skill to cast, not a status, so it is a state of the Assassin Cross
// line (Guillotine Cross and Shadow Cross inherit it).
const (
	edpWeaponBonus = 0.25
	edpCardMul     = 4
	edpState       = "Enchant Deadly Poison"
)

var edpExcluded = map[string]bool{"ASC_METEORASSAULT": true}

func init() {
	regStates("JT_ASSASSIN_H", []StateOption{
		{Key: stateKey(edpState), Label: "Encantar com Veneno Mortal"},
	})
}

// edpActive reports whether EDP raises this attack.
func edpActive(b Build, skillCode string) bool {
	return b.Atk.States[stateKey(edpState)] != 0 && !edpExcluded[skillCode]
}
