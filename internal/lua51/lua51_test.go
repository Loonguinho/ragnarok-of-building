package lua51

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"ragnarok-of-building/internal/config"
	"testing"
)

// chunk assembles a minimal Lua 5.1 main function from code and constants.
func chunk(code []uint32, consts []Value) []byte {
	b := append([]byte{}, header...)
	u32 := func(v uint32) { b = binary.LittleEndian.AppendUint32(b, v) }
	str := func(s string) { u32(uint32(len(s) + 1)); b = append(b, s...); b = append(b, 0) }
	str("@test")
	u32(0)
	u32(0)
	b = append(b, 0, 0, 2, 10) // nups, numparams, vararg, maxstack
	u32(uint32(len(code)))
	for _, c := range code {
		u32(c)
	}
	u32(uint32(len(consts)))
	for _, k := range consts {
		switch v := k.(type) {
		case string:
			b = append(b, 4)
			str(v)
		case float64:
			b = append(b, 3)
			b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v))
		}
	}
	u32(0) // protos
	u32(0) // lineinfo
	u32(0) // locvars
	u32(0) // upvalues
	return b
}

func iABC(op, a, b, c uint32) uint32 { return op | a<<6 | c<<14 | b<<23 }
func iABx(op, a, bx uint32) uint32   { return op | a<<6 | bx<<14 }

// tbl = { [4001] = { identifiedDisplayName = "Carta Poring", desc = {"a","b"} } }
func TestLoadTable(t *testing.T) {
	consts := []Value{"tbl", 4001.0, "identifiedDisplayName", "Carta Poring", "desc", "a", "b"}
	code := []uint32{
		iABC(opNEWTABLE, 0, 0, 0),
		iABC(opNEWTABLE, 1, 0, 0),
		iABC(opSETTABLE, 1, 256+2, 256+3), // r1.identifiedDisplayName = "Carta Poring"
		iABC(opNEWTABLE, 2, 0, 0),
		iABx(opLOADK, 3, 5),
		iABx(opLOADK, 4, 6),
		iABC(opSETLIST, 2, 2, 1),      // r2 = {"a","b"}
		iABC(opSETTABLE, 1, 256+4, 2), // r1.desc = r2
		iABC(opSETTABLE, 0, 256+1, 1), // r0[4001] = r1
		iABx(opSETGLOBAL, 0, 0),       // tbl = r0
		iABC(opRETURN, 0, 1, 0),
	}
	g, err := Load(chunk(code, consts))
	if err != nil {
		t.Fatal(err)
	}
	item := g["tbl"].(*Table).Get(4001.0).(*Table)
	if got := item.Get("identifiedDisplayName"); got != "Carta Poring" {
		t.Errorf("name = %v", got)
	}
	if desc := item.Get("desc").(*Table).Array(); len(desc) != 2 || desc[1] != "b" {
		t.Errorf("desc = %v", desc)
	}
}

func TestMalformedNeverPanics(t *testing.T) {
	good := chunk([]uint32{iABx(opSETGLOBAL, 0, 0), iABC(opRETURN, 0, 1, 0)}, []Value{"x"})
	for i := range good {
		Load(good[:i]) // truncated input may fail; it just must not panic
	}
	bad := chunk([]uint32{iABC(opMOVE, 0, 511, 0)}, nil)
	if _, err := Load(bad); err == nil {
		t.Error("expected error for out-of-range register")
	}
	if _, err := Load([]byte("not lua")); err == nil {
		t.Error("expected error for non-Lua input")
	}
}

// Runs against the real client when present (skipped in CI).
func TestRealIteminfo(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(config.ClientDir(), "System", "iteminfo_new.lub"))
	if err != nil {
		t.Skip("RO LATAM client not installed")
	}
	g, err := Load(b)
	if err != nil {
		t.Fatal(err)
	}
	tbl := g["tbl"].(*Table)
	if len(tbl.Hash) < 10000 {
		t.Errorf("only %d items", len(tbl.Hash))
	}
	if name := tbl.Get(4001.0).(*Table).Get("identifiedDisplayName"); name != "Carta Poring" {
		t.Errorf("4001 = %v", name)
	}
}
