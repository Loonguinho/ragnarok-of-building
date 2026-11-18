// import builds data/gen/catalog.json.gz from an installed RO LATAM client.
//
//	go run ./cmd/import                         # default C:\Gravity\Ragnarok
//	go run ./cmd/import -client D:\RO -out data/gen/catalog.json.gz
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"ragnarok-of-building/internal/clientimport"
	"ragnarok-of-building/internal/rathena"
)

func main() {
	client := flag.String("client", `C:\Gravity\Ragnarok`, "RO LATAM client folder")
	out := flag.String("out", "data/gen/catalog.json.gz", "output catalog file")
	ra := flag.String("rathena", "data/rathena", "rAthena data folder (go run ./cmd/fetchrathena)")
	flag.Parse()

	start := time.Now()
	c, err := clientimport.Import(*client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import:", err)
		os.Exit(1)
	}
	if db, err := rathena.Load(*ra); err != nil {
		fmt.Fprintln(os.Stderr, "warning: no rAthena data, stats/equipment will be missing:", err)
	} else {
		jobs, items := clientimport.MergeRathena(c, db)
		fmt.Printf("rAthena: %d jobs with stat tables, %d items with equipment data\n", jobs, items)
	}
	scripts, combos := clientimport.DescBonuses(c)
	fmt.Printf("descriptions: %d items without rAthena script read from the client text, %d combos\n", scripts, combos)
	if err := c.Save(*out); err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}
	st, _ := os.Stat(*out)
	fmt.Printf("items %d, skills %d, jobs %d -> %s (%d KB) in %s\n",
		len(c.Items), len(c.Skills), len(c.Jobs), *out, st.Size()/1024, time.Since(start).Round(time.Millisecond))
}
