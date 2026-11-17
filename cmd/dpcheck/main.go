// dpcheck fetches a few sample entities from Divine Pride (LATAM) and prints
// them, so we can see the real shape of fields like scripts and prerequisites.
//
//	go run ./cmd/dpcheck                 # default samples
//	go run ./cmd/dpcheck Item 4001 pt    # one entity
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"ragnarok-of-building/internal/config"
	"ragnarok-of-building/internal/divinepride"
)

func main() {
	_ = config.LoadDotEnv(".env")
	key := os.Getenv("DP_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "DP_API_KEY is not set (put it in .env)")
		os.Exit(1)
	}
	c := divinepride.New(key, "LATAM", "data/cache/dp")

	type sample struct {
		kind divinepride.Kind
		id   int
		lang string
	}
	samples := []sample{
		{divinepride.Item, 4001, "pt"}, // Poring Card: simple bonus
		{divinepride.Item, 1201, "pt"}, // Knife: weapon
		{divinepride.Skill, 62, "pt"},  // Bowling Bash: has prerequisites
		{divinepride.Monster, 1002, "pt"},
	}
	if len(os.Args) == 4 {
		id, _ := strconv.Atoi(os.Args[2])
		samples = []sample{{divinepride.Kind(os.Args[1]), id, os.Args[3]}}
	}

	for _, s := range samples {
		fmt.Printf("===== %s %d [%s] =====\n", s.kind, s.id, s.lang)
		b, err := c.Get(context.Background(), s.kind, s.id, s.lang)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		var out bytes.Buffer
		if json.Indent(&out, b, "", "  ") != nil {
			out.Write(b)
		}
		s := out.String()
		if len(s) > 4000 {
			s = s[:4000] + "\n... (truncated)"
		}
		fmt.Println(s)
	}
}
