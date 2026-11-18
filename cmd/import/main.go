// import builds data/gen/catalog.json.gz from an installed RO LATAM client.
//
//	go run ./cmd/import                         # RO_CLIENT from .env, else C:\Gravity\Ragnarok
//	go run ./cmd/import -client D:\RO -out data/gen/catalog.json.gz
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"ragnarok-of-building/internal/config"
	"ragnarok-of-building/internal/setup"
)

func main() {
	client := flag.String("client", config.ClientDir(), "RO LATAM client folder (default: RO_CLIENT in .env)")
	out := flag.String("out", "data/gen/catalog.json.gz", "output catalog file")
	ra := flag.String("rathena", "data/rathena", "rAthena data folder (go run ./cmd/fetchrathena)")
	flag.Parse()

	start := time.Now()
	c, err := setup.BuildCatalog(*client, *ra, *out, func(m string) { fmt.Println(m) })
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	st, _ := os.Stat(*out)
	fmt.Printf("items %d, skills %d, jobs %d -> %s (%d KB) in %s\n",
		len(c.Items), len(c.Skills), len(c.Jobs), *out, st.Size()/1024, time.Since(start).Round(time.Millisecond))
}
