package calc

// Encantar com Veneno Mortal (ASC_EDP), as latam-ro-calc measured it on
// LATAM recordings (sc-edp-dummy-elements.rrf, sc-edp-element-states.rrf):
//
//   - weapon ATK +15%, whatever the target's element: an Executor replay
//     (Executor_JorDun01.rrf, 2026-11-22) gains the same ×2,687 with EDP
//     on a Poison 3 Rgan as on a Water 3 Cave Flower, so the share is not
//     poison damage (the earlier reading made it 0% on Poison monsters).
//     The 15% is fitted, not a known rule: with every other term matching
//     the recordings without EDP to 0,1%, the bonus that lands each packet
//     exactly is 12,7% (Rgan), 13,3% (Hallucigenia Baby) and 19,0% (another
//     Executor on the Dark dummy, LONGUINHO 6/7.rrf, Cross Impact), against
//     the 25% iROwiki gives; 15% keeps all three within 1,7%;
//   - the weapon and equipment ATK after the card modifiers is ×4 (the
//     %ATK share is not);
//   - the status ATK takes no endow while it is up.
//
// Every physical attack gets it except Meteor Assault. The client lists it
// as a skill to cast, not a status, so it is a state of the Assassin Cross
// line (Guillotine Cross and Shadow Cross inherit it).
const (
	edpWeaponBonus = 0.15
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
