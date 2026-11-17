// lubdump loads a compiled .lub file from the RO client and prints its globals
// (or one entry of a global table) as JSON.
//
//	go run ./cmd/lubdump C:/Gravity/Ragnarok/System/iteminfo_new.lub            # summary
//	go run ./cmd/lubdump C:/Gravity/Ragnarok/System/iteminfo_new.lub tbl 4001   # one entry
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"ragnarok-of-building/internal/lua51"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: lubdump FILE [GLOBAL [KEY]]")
		os.Exit(2)
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	g, err := lua51.Load(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(os.Args) == 2 {
		for name, v := range g {
			switch t := v.(type) {
			case *lua51.Table:
				fmt.Printf("%-30s table, %d entries\n", name, len(t.Hash))
			default:
				fmt.Printf("%-30s %T\n", name, v)
			}
		}
		return
	}
	var v lua51.Value = g[os.Args[2]]
	if len(os.Args) > 3 {
		t, ok := v.(*lua51.Table)
		if !ok {
			fmt.Fprintln(os.Stderr, "not a table")
			os.Exit(1)
		}
		var k lua51.Value = os.Args[3]
		if f, err := strconv.ParseFloat(os.Args[3], 64); err == nil {
			k = f
		}
		v = t.Get(k)
	}
	out, _ := json.MarshalIndent(toJSON(v, 0), "", "  ")
	fmt.Println(string(out))
}

func toJSON(v lua51.Value, depth int) any {
	switch t := v.(type) {
	case *lua51.Table:
		if depth > 3 {
			return fmt.Sprintf("<table %d entries>", len(t.Hash))
		}
		if arr := t.Array(); len(arr) == len(t.Hash) && len(arr) > 0 {
			out := make([]any, len(arr))
			for i, e := range arr {
				out[i] = toJSON(e, depth+1)
			}
			return out
		}
		out := map[string]any{}
		n := 0
		for k, e := range t.Hash {
			if n++; n > 40 {
				out["..."] = fmt.Sprintf("%d more", len(t.Hash)-40)
				break
			}
			out[fmt.Sprint(k)] = toJSON(e, depth+1)
		}
		return out
	case *lua51.Function:
		return "<function>"
	default:
		return t
	}
}
