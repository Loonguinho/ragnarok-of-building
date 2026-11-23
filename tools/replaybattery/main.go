// Command replaybattery imports recordings and compares every skill's
// recorded hits with the calculation, monster by monster:
//
//	go run ./tools/replaybattery [-all] file-or-dir...
//
// The ratio is recorded / calculated at the biggest packet (the moment the
// imported statuses were read) and at the smallest. Recordings without
// traits or worn gear, or older than the 28/10/2026 client, are marked: they
// do not measure the current formulas.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/replay"
	"ragnarok-of-building/internal/view"
)

// rebalance is when LATAM's current formulas went live.
var rebalance = time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)

func main() {
	all := flag.Bool("all", false, "also the recordings that cannot measure the current formulas")
	cat := flag.String("catalog", "data/gen/catalog.json.gz", "catalog")
	minHits := flag.Int("min", 3, "packets a skill needs on a monster")
	on := flag.String("on", "", "Build.Buffs keys to turn on, comma separated (proc:ev31975)")
	flag.Parse()
	c, err := gamedata.Load(*cat)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	v, err := view.New(view.Options{Catalog: c})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var files []string
	for _, a := range flag.Args() {
		if st, err := os.Stat(a); err == nil && st.IsDir() {
			m, _ := filepath.Glob(filepath.Join(a, "*.rrf"))
			files = append(files, m...)
		} else {
			files = append(files, a)
		}
	}
	sort.Strings(files)
	var good, bad int
	for _, f := range files {
		buf, err := os.ReadFile(f)
		if err != nil {
			fmt.Println("##", f, err)
			continue
		}
		s, err := replay.Decode(buf)
		if err != nil {
			fmt.Printf("## %s: %v\n", filepath.Base(f), err)
			continue
		}
		b, notes, _, err := v.ImportReplay(buf, "pt")
		if err != nil {
			fmt.Printf("## %s: %v\n", filepath.Base(f), err)
			continue
		}
		for _, k := range strings.Split(*on, ",") {
			if k != "" {
				if b.Buffs == nil {
					b.Buffs = map[string]int{}
				}
				b.Buffs[k] = 1
			}
		}
		why := ""
		if s.RecordedAt.Before(rebalance) {
			why += " ANTES-DO-REBALANCE"
		}
		if j := c.Jobs[b.Job]; j != nil && j.Fourth && len(s.Traits) == 0 {
			why += " SEM-TALENTOS"
		}
		if len(b.Equip) == 0 {
			why += " SEM-EQUIP"
		}
		name := "?"
		if j := c.Jobs[b.Job]; j != nil {
			name = j.Name.Get("pt")
		}
		fmt.Printf("\n## %s · %s %d/%d · %s%s\n", filepath.Base(f), name, b.BaseLv, b.JobLv, s.RecordedAt.Format("2006-01-02"), why)
		if why != "" && !*all {
			continue
		}
		fmt.Println("   notas:", notes, "estados:", b.Atk.States, "buffs:", b.Buffs)
		fmt.Println("   status:", s.Statuses)
		for _, d := range s.Damage {
			if !d.Self {
				continue
			}
			for _, m := range d.Monsters {
				mm := c.Monster(m.Mob)
				if mm == nil {
					continue
				}
				for _, sk := range m.Skill {
					if sk.Hits < *minHits {
						continue
					}
					sn := "Ataque básico"
					if x := c.Skills[sk.Skill]; x != nil {
						sn = x.Name.Get("pt")
					}
					x := b.Clone()
					x.Target = m.Mob
					x.Atk.Skill, x.Atk.Level, x.Atk.Ratio, x.Atk.Variant = sk.Skill, b.Skills[sk.Skill], 0, 0
					if sk.Skill != 0 && x.Atk.Level == 0 {
						fmt.Printf("   %-30s %-24s %3d golpes: não aprendida (autoconjurada?)\n", mm.Name, sn, sk.Hits)
						continue
					}
					dv := v.Build(x, "pt").D
					// A packet is the whole use or one hit of it.
					h := int64(max(1, dv.Dmg.Hits))
					lo, hi, crit := dv.Dmg.Min, dv.Dmg.Max, dv.Dmg.Crit
					avg := float64(sk.Total) / float64(sk.Hits)
					if h > 1 && math.Abs(math.Log(avg/float64(max(1, lo/h)))) < math.Abs(math.Log(avg/float64(max(1, lo)))) {
						lo, hi, crit = lo/h, hi/h, crit/h
					}
					rmin, rmax := float64(sk.Min)/float64(max(1, lo)), float64(sk.Max)/float64(max(1, hi))
					mark := "ok "
					rcrit := ""
					if crit > 0 {
						k := float64(sk.Max) / float64(crit)
						rcrit = fmt.Sprintf(" crít ×%.3f", k)
						if math.Abs(k-1) < math.Abs(rmax-1) {
							rmax = k
						}
					}
					if math.Abs(rmax-1) > 0.03 {
						mark = "!! "
						bad++
					} else {
						good++
					}
					fmt.Printf(" %s%-30s %-24s %4d golpes  gravado %d~%d  calc %d~%d  min ×%.3f max ×%.3f%s [%s]\n", mark, mm.Name, sn, sk.Hits, sk.Min, sk.Max, lo, hi, rmin, rmax, rcrit, dv.RatioSource)
				}
			}
		}
	}
	fmt.Printf("\n%d dentro de 3%%, %d fora\n", good, bad)
}
