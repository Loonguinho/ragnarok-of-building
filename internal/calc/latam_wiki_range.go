package calc

import "strings"

// Ranged physical skills. In latam-ro-calc a skill that does not say it is
// melee is ranged; here one that says nothing follows the weapon, so these
// counted the melee bonuses with a melee weapon (a knuckle, a katar...).
// Only the skills the LATAM client also calls ranged ("dano físico à
// distância") are listed.
//
// This file's init runs after the other latam_*.go (file name order).
const rangedSkills = `ABC_CHAIN_REACTION_SHOT ABC_FRENZY_SHOT ASC_BREAKER DK_MADNESS_CRUSHER
GC_CROSSRIPPERSLASHER GS_TRACKING HN_SHIELD_CHAIN_RUSH IQ_EXPOSION_BLASTER IQ_MASSIVE_F_BLASTER
KO_HAPPOKUNAI KO_HUUMARANKA LG_BANISHINGPOINT LG_CANNONSPEAR LK_HEADCRUSH LK_JOINTBEAT
NC_AXEBOOMERANG NJ_HUUMA NJ_KUNAI NJ_SYURIKEN NW_BASIC_GRENADE NW_HASTY_FIRE_IN_THE_HOLE
NW_MAGAZINE_FOR_ONE NW_ONLY_ONE_BULLET NW_SPIRAL_SHOOTING NW_THE_VIGILANTE_AT_NIGHT NW_WILD_FIRE
RA_AIMEDBOLT RK_HUNDREDSPEAR RK_SONICWAVE RL_BANISHING_BUSTER RL_FIREDANCE RL_R_TRIP RL_S_STORM
SC_TRIANGLESHOT SH_CHUL_HO_SONIC_CLAW SH_HOGOGONG_STRIKE SH_HOWLING_OF_CHUL_HO SN_SHARPSHOOTING
SR_GATEOFHELL SR_RAMPAGEBLASTER SR_RIDEINLIGHTNING SS_FUUMAKOUCHIKU SS_FUUMASHOUAKU SS_HITOUAKUMU
SS_KUNAIKAITEN SS_KUNAIKUSSETSU SS_KUNAIWAIKYOKU SU_PICKYPECK TR_RHYTHMSHOOTING TR_ROSEBLOSSOM
WH_CRESCIVE_BOLT WH_GALESTORM WH_HAWKRUSH`

func init() {
	for _, code := range strings.Fields(rangedSkills) {
		for _, m := range latamByJob {
			if f := m[code]; f != nil {
				for _, v := range f.Variants() {
					if v.Melee == nil && !v.Magic {
						v.Melee = meleeNever
					}
				}
			}
		}
	}
	// Punho Labareda: the bonus is against Brute and Demon (client, bROWiki);
	// 2150 a level (2300 with it) + POW x 15.
	wikiFix("IQ_MASSIVE_F_BLASTER", func(f *SkillFormula) {
		f.Ratio = func(in FormulaInput) float64 {
			return (float64(in.Lv)*tern(in.MonIs("race", "brute", "demon"), 2300, 2150) + in.T(POW)*15) * float64(in.BaseLv) / 100
		}
	})
}
