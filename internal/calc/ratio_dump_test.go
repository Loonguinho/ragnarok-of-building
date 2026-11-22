package calc

import (
	"encoding/json"
	"os"
	"sort"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestDumpRatios writes every generated formula's ratio at base level 100
// with no stats, traits, masteries or states (the conditions of bROWiki's
// per-level tables) to $DUMP_RATIOS, for tools/browiki/compare.py.
func TestDumpRatios(t *testing.T) {
	path := os.Getenv("DUMP_RATIOS")
	if path == "" {
		t.Skip("set DUMP_RATIOS=<file>")
	}
	type entry struct {
		Jobs     []string             `json:"jobs"`
		Magic    bool                 `json:"magic"`
		Hits     []int                `json:"hits"`
		Ratios   []float64            `json:"ratios"`
		Variants map[string][]float64 `json:"variants,omitempty"`
		Races    map[string][]float64 `json:"races"` // ratios vs other races (race bonuses)
		// Coefs: % each point of a stat or trait adds at Nv 10, base level
		// 100 (STR..LUK as FOR, AGI, VIT, INT, DES, SOR; traits as POW..CRT),
		// per variant ("" = the first).
		Coefs map[string]map[string]float64 `json:"coefs,omitempty"`
	}
	statNames := []string{"FOR", "AGI", "VIT", "INT", "DES", "SOR"}
	traitNames := []string{"POD", "VIG", "SAB", "FEI", "CON", "PRE"}
	coefs := func(f *SkillFormula) map[string]float64 {
		mk := func() FormulaInput {
			return FormulaInput{Lv: 10, BaseLv: 100, States: map[string]int{}, Monster: &gamedata.Monster{Race: "Formless", Element: "Neutral", Size: "Medium"}}
		}
		base := f.Ratio(mk())
		out := map[string]float64{}
		for i := 0; i < 6; i++ {
			in := mk()
			in.Stats[i] = 100
			if d := (f.Ratio(in) - base) / 100; d != 0 {
				out[statNames[i]] = d
			}
			in = mk()
			in.Traits[i] = 100
			if d := (f.Ratio(in) - base) / 100; d != 0 {
				out[traitNames[i]] = d
			}
		}
		return out
	}
	ratios := func(f *SkillFormula, race string) []float64 {
		var out []float64
		for lv := 1; lv <= 10; lv++ {
			in := FormulaInput{Lv: lv, BaseLv: 100, States: map[string]int{}, Monster: &gamedata.Monster{Race: race, Element: "Neutral", Size: "Medium"}}
			out = append(out, f.Ratio(in))
		}
		return out
	}
	jobs := make([]string, 0, len(latamByJob))
	for j := range latamByJob {
		jobs = append(jobs, j)
	}
	sort.Strings(jobs)
	out := map[string]*entry{}
	for _, job := range jobs {
		for code, f := range latamByJob[job] {
			key := code
			if e, ok := out[key]; ok {
				e.Jobs = append(e.Jobs, job)
				continue
			}
			e := &entry{Jobs: []string{job}, Magic: f.Magic}
			for lv := 1; lv <= 10; lv++ {
				in := FormulaInput{Lv: lv, BaseLv: 100, States: map[string]int{}, Monster: &gamedata.Monster{Race: "Formless", Element: "Neutral", Size: "Medium"}}
				e.Ratios = append(e.Ratios, f.Ratio(in))
				h := 1
				if f.Hits != nil {
					h = f.Hits(in)
				}
				e.Hits = append(e.Hits, h)
			}
			e.Races = map[string][]float64{}
			for _, race := range []string{"Brute", "Demihuman", "Demon", "Undead", "Angel", "Fish", "Plant", "Insect", "Dragon"} {
				e.Races[race] = ratios(f, race)
			}
			e.Coefs = map[string]map[string]float64{"": coefs(f)}
			for _, alt := range f.Alts {
				e.Coefs[alt.Variant] = coefs(alt)
				if e.Variants == nil {
					e.Variants = map[string][]float64{}
				}
				e.Variants[alt.Variant] = ratios(alt, "Formless")
			}
			out[key] = e
		}
	}
	b, _ := json.MarshalIndent(out, "", " ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
