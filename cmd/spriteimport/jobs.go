package main

// jobSprites names each job's body sprite (data\sprite\<race>\몸통\<sex>\
// <name>_<sex>.spr). The client keeps this table in its executable for
// the older jobs; the names are the files in the LATAM data.grf. A
// transcendent third job wears the same sprite as the third job.
var jobSprites = map[string]string{
	"JT_NOVICE": "초보자", "JT_SWORDMAN": "검사", "JT_MAGICIAN": "마법사", "JT_ARCHER": "궁수",
	"JT_ACOLYTE": "성직자", "JT_MERCHANT": "상인", "JT_THIEF": "도둑",
	"JT_KNIGHT": "기사", "JT_PRIEST": "프리스트", "JT_WIZARD": "위저드", "JT_BLACKSMITH": "제철공",
	"JT_HUNTER": "헌터", "JT_ASSASSIN": "어세신", "JT_CRUSADER": "크루세이더", "JT_MONK": "몽크",
	"JT_SAGE": "세이지", "JT_ROGUE": "로그", "JT_ALCHEMIST": "연금술사", "JT_BARD": "바드",
	"JT_DANCER": "무희", "JT_SUPERNOVICE": "슈퍼노비스", "JT_GUNSLINGER": "건너", "JT_NINJA": "닌자",

	"JT_KNIGHT_H": "로드나이트", "JT_PRIEST_H": "하이프리", "JT_WIZARD_H": "하이위저드",
	"JT_BLACKSMITH_H": "화이트스미스", "JT_HUNTER_H": "스나이퍼", "JT_ASSASSIN_H": "어쌔신크로스",
	"JT_CRUSADER_H": "팔라딘", "JT_MONK_H": "챔피온", "JT_SAGE_H": "프로페서", "JT_ROGUE_H": "스토커",
	"JT_ALCHEMIST_H": "크리에이터", "JT_BARD_H": "클라운", "JT_DANCER_H": "집시",

	"JT_TAEKWON": "태권소년", "JT_STAR": "권성", "JT_LINKER": "소울링커",

	"JT_RUNE_KNIGHT": "룬나이트", "JT_WARLOCK": "워록", "JT_RANGER": "레인져", "JT_ARCHBISHOP": "아크비숍",
	"JT_MECHANIC": "미케닉", "JT_GUILLOTINE_CROSS": "길로틴크로스", "JT_ROYAL_GUARD": "가드",
	"JT_SORCERER": "소서러", "JT_MINSTREL": "민스트럴", "JT_WANDERER": "원더러", "JT_SURA": "슈라",
	"JT_GENETIC": "제네릭", "JT_SHADOW_CHASER": "쉐도우체이서",
	"JT_RUNE_KNIGHT_H": "룬나이트", "JT_WARLOCK_H": "워록", "JT_RANGER_H": "레인져", "JT_ARCHBISHOP_H": "아크비숍",
	"JT_MECHANIC_H": "미케닉", "JT_GUILLOTINE_CROSS_H": "길로틴크로스", "JT_ROYAL_GUARD_H": "가드",
	"JT_SORCERER_H": "소서러", "JT_MINSTREL_H": "민스트럴", "JT_WANDERER_H": "원더러", "JT_SURA_H": "슈라",
	"JT_GENETIC_H": "제네릭", "JT_SHADOW_CHASER_H": "쉐도우체이서",

	"JT_SUPERNOVICE2": "슈퍼노비스", "JT_KAGEROU": "kagerou", "JT_OBORO": "oboro", "JT_REBELLION": "rebellion",
	"JT_STAR_EMPEROR": "성제", "JT_SOUL_REAPER": "소울리퍼", "JT_DO_SUMMONER": "summoner",

	"JT_DRAGON_KNIGHT": "dragon_knight", "JT_MEISTER": "meister", "JT_SHADOW_CROSS": "shadow_cross",
	"JT_ARCH_MAGE": "arch_mage", "JT_CARDINAL": "cardinal", "JT_WINDHAWK": "windhawk",
	"JT_IMPERIAL_GUARD": "imperial_guard", "JT_BIOLO": "biolo", "JT_ABYSS_CHASER": "abyss_chaser",
	"JT_ELEMENTAL_MASTER": "elemetal_master", "JT_INQUISITOR": "inquisitor", "JT_TROUBADOUR": "troubadour",
	"JT_TROUVERE": "trouvere", "JT_SKY_EMPEROR": "sky_emperor", "JT_SOUL_ASCETIC": "soul_ascetic",
	"JT_SHINKIRO": "shinkiro", "JT_SHIRANUI": "shiranui", "JT_NIGHT_WATCH": "night_watch",
	"JT_HYPER_NOVICE": "hyper_novice", "JT_SPIRIT_HANDLER": "spirit_handler",
}

// doramJobs are drawn from data\sprite\도람족 (their own bodies and heads).
var doramJobs = []string{"JT_DO_SUMMONER", "JT_SPIRIT_HANDLER"}
