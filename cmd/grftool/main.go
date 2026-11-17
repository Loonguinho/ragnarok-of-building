// grftool lists and extracts files from an RO client GRF archive.
//
//	go run ./cmd/grftool C:/Gravity/Ragnarok/data.grf find skilltree
//	go run ./cmd/grftool C:/Gravity/Ragnarok/data.grf cat "data\luafiles514\lua files\skillinfoz\skilltreeview.lub" > out.lub
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"

	"ragnarok-of-building/internal/grf"
	"ragnarok-of-building/internal/lua51"
)

func toJSON(v lua51.Value, depth int) any {
	switch t := v.(type) {
	case *lua51.Table:
		if depth > 4 {
			return fmt.Sprintf("<table %d>", len(t.Hash))
		}
		if arr := t.Array(); len(arr) == len(t.Hash) && len(arr) > 0 {
			out := make([]any, len(arr))
			for i, e := range arr {
				out[i] = toJSON(e, depth+1)
			}
			return out
		}
		out := map[string]any{}
		for k, e := range t.Hash {
			out[fmt.Sprint(k)] = toJSON(e, depth+1)
		}
		return out
	case *lua51.Function:
		return "<function>"
	default:
		return t
	}
}

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: grftool GRF find SUBSTR | GRF cat PATH")
		os.Exit(2)
	}
	a, err := grf.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer a.Close()

	switch os.Args[2] {
	case "find":
		for _, e := range a.Find(os.Args[3]) {
			enc := ""
			if e.Encrypted() {
				enc = " [encrypted]"
			}
			fmt.Printf("%10d  %s%s\n", e.Size, e.Name, enc)
		}
	case "cat":
		b, err := a.Read(os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Stdout.Write(b)
	case "lua":
		// grftool GRF lua FILE... [-- GLOBAL [KEY]]
		files, query := os.Args[3:], []string(nil)
		for i, s := range files {
			if s == "--" {
				files, query = files[:i], files[i+1:]
				break
			}
		}
		env := map[string]lua51.Value{}
		for _, p := range files {
			b, err := a.Read(p)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if _, err := lua51.LoadEnv(b, env); err != nil {
				var pe *lua51.PartialError
				if errors.As(err, &pe) {
					fmt.Fprintf(os.Stderr, "warning: %s: %v (kept globals so far)\n", p, err)
					continue
				}
				fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
				os.Exit(1)
			}
		}
		if len(query) == 0 {
			names := make([]string, 0, len(env))
			for k := range env {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, k := range names {
				if t, ok := env[k].(*lua51.Table); ok {
					fmt.Printf("%-36s table(%d)\n", k, len(t.Hash))
				} else {
					fmt.Printf("%-36s %T\n", k, env[k])
				}
			}
			return
		}
		v := env[query[0]]
		if len(query) > 1 {
			t, _ := v.(*lua51.Table)
			if t == nil {
				fmt.Fprintln(os.Stderr, "not a table")
				os.Exit(1)
			}
			var k lua51.Value = query[1]
			if f, err := strconv.ParseFloat(query[1], 64); err == nil {
				k = f
			}
			v = t.Get(k)
		}
		out, _ := json.MarshalIndent(toJSON(v, 0), "", "  ")
		fmt.Println(string(out))
	default:
		fmt.Fprintln(os.Stderr, "unknown command", os.Args[2])
		os.Exit(2)
	}
}
