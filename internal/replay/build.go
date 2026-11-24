package replay

import (
	"fmt"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// equipBits maps rAthena e_equip_pos bits to build slots (ammo comes from
// Snapshot.Ammo).
var equipBits = []struct {
	bit  uint32
	slot string
}{
	{0x1, calc.HeadLow}, {0x2, calc.Weapon}, {0x4, calc.Garment}, {0x8, calc.AccRight},
	{0x10, calc.Armor}, {0x20, calc.Shield}, {0x40, calc.Shoes}, {0x80, calc.AccLeft},
	{0x100, calc.HeadTop}, {0x200, calc.HeadMid},
	{0x400, calc.CosTop}, {0x800, calc.CosMid}, {0x1000, calc.CosLow}, {0x2000, calc.CosGarment},
	{0x10000, calc.ShArmor}, {0x20000, calc.ShWeapon}, {0x40000, calc.ShShield},
	{0x80000, calc.ShShoes}, {0x100000, calc.ShAccR}, {0x200000, calc.ShAccL},
}

// ToBuild turns a snapshot into a build plus notes for the user about what
// could not be imported. The build still goes through calc.Normalize.
func ToBuild(s *Snapshot, c *gamedata.Catalog) (calc.Build, []string) {
	b := calc.Build{
		Job: s.Job, BaseLv: s.BaseLv, JobLv: s.JobLv, Stats: s.Stats,
		Equip: map[string]calc.Equip{}, Skills: map[int]int{},
	}
	var notes []string
	if c.Jobs[s.Job] == nil {
		notes = append(notes, fmt.Sprintf("Classe %d desconhecida no catálogo.", s.Job))
	}
	for i := range b.Stats {
		b.Stats[i] = max(1, b.Stats[i])
	}
	if len(s.Traits) > 0 {
		for i, v := range s.Traits {
			if i >= 0 && i < len(b.Traits) {
				b.Traits[i] = v
			}
		}
	} else if j := c.Jobs[s.Job]; j != nil && j.Fourth {
		notes = append(notes, "A gravação não trouxe os traços (POW, STA...): ajuste-os na janela de atributos.")
	}
	unknown := 0
	for _, it := range s.Equipped {
		if c.Items[it.ItemID] == nil {
			unknown++
			continue
		}
		mask := it.Equipped
		if mask&0x2 != 0 && mask&0x20 != 0 {
			mask &^= 0x20 // two-handed weapon: both hand bits set, keep the weapon slot
		}
		if mask == 0x20 && c.Items[it.ItemID].Type == "Weapon" {
			notes = append(notes, c.Items[it.ItemID].Name.Get("pt")+" na mão esquerda: empunhadura dupla ainda não é simulada.")
			continue
		}
		for _, eb := range equipBits {
			if mask&eb.bit != 0 {
				b.Equip[eb.slot] = calc.Equip{Item: it.ItemID, Refine: it.Refine, Grade: it.Grade, Cards: it.Cards, Opts: it.Opts}
			}
		}
	}
	if it := c.Items[s.Ammo]; it != nil && it.Type == "Ammo" {
		b.Equip[calc.Ammo] = calc.Equip{Item: s.Ammo}
	}
	if unknown > 0 {
		notes = append(notes, fmt.Sprintf("%d item(ns) equipado(s) não existem no catálogo e foram ignorados.", unknown))
	}
	if len(s.Equipped) == 0 {
		notes = append(notes, "A gravação não trouxe o equipamento vestido.")
	}
	for id, lv := range s.Skills {
		b.Skills[id] = lv
	}
	if s.Pet != nil {
		found := false
		for egg, p := range c.Pets {
			if s.Pet.Mob != 0 && p.Mob == s.Pet.Mob {
				b.Pet, b.PetIntimacy, found = egg, s.Pet.Intimacy, true
				break
			}
		}
		if !found {
			notes = append(notes, "O pet da gravação não foi reconhecido: escolha-o na janela de Buffs.")
		}
	}
	n := applyStatuses(&b, s, c)
	if b.Atk.Element == "Poison" {
		notes = append(notes, "O ataque ficou com elemento Veneno, como estava no maior golpe da gravação: contra monstros de Veneno, troque o elemento.")
	}
	if n > 0 {
		notes = append(notes, fmt.Sprintf("%d buff(s) ativos no maior golpe da gravação foram ligados (níveis máximos quando o nível não vem na gravação). Efeitos no alvo, como a Profanação, não vêm na gravação.", n))
	}
	return b, notes
}
