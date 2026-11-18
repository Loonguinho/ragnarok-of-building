// fetchrathena downloads the rAthena (GPL-3.0) renewal database files the
// importer uses into data/rathena/. They are not committed to this repo.
//
//	go run ./cmd/fetchrathena
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"ragnarok-of-building/internal/setup"
)

func main() {
	out := flag.String("out", "data/rathena", "output folder")
	flag.Parse()
	os.Remove(*out + "/COMMIT") // force a fresh download
	if err := setup.FetchRathena(context.Background(), *out, func(m string) { fmt.Println(m) }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
