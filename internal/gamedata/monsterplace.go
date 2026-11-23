package gamedata

// Where monsters live, for the search: the Illusion dungeons, whose monsters
// are named after their cards and so are hard to find. Names are the LATAM
// client's (mapInfo); the IDs are rAthena's ILL_* ranges.

type monsterPlace struct {
	name   Text
	ranges [][2]int
}

var monsterPlaces = []monsterPlace{
	{Text{"pt": "Ilusão do Vampiro", "es": "Ilusión vampírica", "en": "Illusion of Vampire"}, [][2]int{{3750, 3757}, {25041, 25046}}},
	{Text{"pt": "Ilusão da Lua", "es": "Ilusión lunar", "en": "Illusion of Moonlight"}, [][2]int{{3758, 3765}, {25061, 25066}}},
	{Text{"pt": "Ilusão do Gelo", "es": "Ilusión de hielo", "en": "Illusion of Frozen"}, [][2]int{{3792, 3796}}},
	{Text{"pt": "Ilusão da Tartaruga", "es": "Ilusión de tortuga", "en": "Illusion of Turtle"}, [][2]int{{3799, 3804}, {3896, 3896}}},
	{Text{"pt": "Ilusão do Ursinho", "es": "Ilusión de osito de peluche", "en": "Illusion of Teddy Bear"}, [][2]int{{20255, 20268}}},
	{Text{"pt": "Ilusão de Luanda", "es": "Ilusión de Luanda", "en": "Illusion of Luanda"}, [][2]int{{20270, 20280}}},
	{Text{"pt": "Ilusão do Labirinto", "es": "Ilusión del laberinto", "en": "Illusion of Labyrinth"}, [][2]int{{20520, 20533}}},
	{Text{"pt": "Ilusão do Mar", "es": "Ilusión submarina", "en": "Illusion of Underwater"}, [][2]int{{20801, 20811}, {20843, 20843}, {20877, 20885}}},
	{Text{"pt": "Ilusão dos Gêmeos", "es": "Ilusión de los gemelos", "en": "Illusion of Twins"}, [][2]int{{21386, 21395}, {21431, 21435}}},
}

// MonsterPlace is the place a monster is listed under; nil when unknown.
func MonsterPlace(id int) Text {
	for _, p := range monsterPlaces {
		for _, r := range p.ranges {
			if id >= r[0] && id <= r[1] {
				return p.name
			}
		}
	}
	return nil
}

// placeMatches reports a search that asks for the monster's place.
func placeMatches(id int, words []string) bool {
	for _, n := range MonsterPlace(id) {
		if containsAll(Normalize(n), words) {
			return true
		}
	}
	return false
}
