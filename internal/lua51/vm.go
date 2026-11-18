package lua51

import (
	"fmt"
	"math"
	"sort"
)

// Value is nil, bool, float64, string, *Table or *Function.
type Value any

// Function is an opaque closure; data files may define helper functions we never call.
type Function struct{ Proto *Proto }

// GoFunc is a native function a caller puts in the globals (or in a table)
// so chunks that build data through calls (obj:Method(...)) can run.
type GoFunc func(args []Value) []Value

// NewTable returns an empty table (for callers building objects for GoFuncs).
func NewTable() *Table { return newTable() }

// Set stores a value (nil deletes).
func (t *Table) Set(k, v Value) { t.set(k, v) }

// Table is a Lua table.
type Table struct {
	Hash map[Value]Value
	// Missing, when set, supplies values for absent keys (like __index);
	// objects backing GoFuncs use it to accept unknown methods.
	Missing func(k Value) Value
}

func newTable() *Table { return &Table{Hash: map[Value]Value{}} }

// Get returns t[k].
func (t *Table) Get(k Value) Value {
	v := t.Hash[normKey(k)]
	if v == nil && t.Missing != nil {
		return t.Missing(k)
	}
	return v
}

func (t *Table) set(k, v Value) {
	k = normKey(k)
	if v == nil {
		delete(t.Hash, k)
		return
	}
	t.Hash[k] = v
}

func normKey(k Value) Value {
	if f, ok := k.(float64); ok && f == math.Trunc(f) {
		return f // keep float64; ints are always stored as float64
	}
	return k
}

// Array returns t[1..n] for the longest run of non-nil integer keys.
func (t *Table) Array() []Value {
	var out []Value
	for i := 1; ; i++ {
		v, ok := t.Hash[float64(i)]
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

// IntKeys returns the integer keys in ascending order.
func (t *Table) IntKeys() []int {
	var ks []int
	for k := range t.Hash {
		if f, ok := k.(float64); ok && f == math.Trunc(f) {
			ks = append(ks, int(f))
		}
	}
	sort.Ints(ks)
	return ks
}

// Lua 5.1 opcodes used by data chunks.
const (
	opMOVE      = 0
	opLOADK     = 1
	opLOADBOOL  = 2
	opLOADNIL   = 3
	opGETUPVAL  = 4
	opGETGLOBAL = 5
	opGETTABLE  = 6
	opSETGLOBAL = 7
	opSETTABLE  = 9
	opNEWTABLE  = 10
	opSELF      = 11
	opUNM       = 18
	opJMP       = 22
	opCALL      = 28
	opRETURN    = 30
	opSETLIST   = 34
	opCLOSURE   = 36
)

const fieldsPerFlush = 50

// PartialError means execution stopped at an opcode the data VM does not
// support (e.g. a function call after the data tables). The globals returned
// alongside it hold everything assigned before that point; callers decide
// whether that is enough (check the globals they need).
type PartialError struct {
	PC     int
	Op     uint32
	Global string // set when stopped on an undefined global (e.g. setmetatable)
}

func (e *PartialError) Error() string {
	if e.Global != "" {
		return fmt.Sprintf("lua51: stopped at pc %d: undefined global %q", e.PC, e.Global)
	}
	return fmt.Sprintf("lua51: stopped at pc %d: unsupported opcode %d", e.PC, e.Op)
}

// Run executes the main chunk and returns the resulting globals.
// Only table-building opcodes are supported; anything else is an error,
// so a changed file format fails loudly instead of producing bad data.
func Run(p *Proto) (map[string]Value, error) {
	return RunEnv(p, map[string]Value{})
}

// RunEnv executes the chunk against an existing globals map, so files that
// reference tables defined by earlier files (e.g. SKID, JOBID) resolve.
func RunEnv(p *Proto, globals map[string]Value) (map[string]Value, error) {
	reg := make([]Value, 256)
	rk := func(x uint32) Value {
		if x >= 256 {
			return p.Consts[x-256]
		}
		return reg[x]
	}
	code := p.Code
	for pc := 0; pc < len(code); pc++ {
		ins := code[pc]
		op := ins & 0x3f
		a := (ins >> 6) & 0xff
		c := (ins >> 14) & 0x1ff
		b := ins >> 23
		bx := ins >> 14
		switch op {
		case opMOVE:
			reg[a] = reg[b]
		case opLOADK:
			if int(bx) >= len(p.Consts) {
				return nil, fmt.Errorf("lua51: constant %d out of range", bx)
			}
			reg[a] = p.Consts[bx]
		case opLOADBOOL:
			reg[a] = b != 0
			if c != 0 {
				pc++
			}
		case opLOADNIL:
			for i := a; i <= b; i++ {
				reg[i] = nil
			}
		case opGETGLOBAL:
			name := p.Consts[bx].(string)
			v, ok := globals[name]
			if !ok {
				return globals, &PartialError{PC: pc, Op: op, Global: name}
			}
			reg[a] = v
		case opSETGLOBAL:
			globals[p.Consts[bx].(string)] = reg[a]
		case opGETTABLE:
			t, ok := reg[b].(*Table)
			if !ok {
				return nil, fmt.Errorf("lua51: pc %d: index of non-table", pc)
			}
			reg[a] = t.Get(rk(c))
		case opSETTABLE:
			t, ok := reg[a].(*Table)
			if !ok {
				return nil, fmt.Errorf("lua51: pc %d: index of non-table", pc)
			}
			k := rk(b)
			if k == nil {
				return nil, fmt.Errorf("lua51: pc %d: nil table key", pc)
			}
			t.set(k, rk(c))
		case opNEWTABLE:
			reg[a] = newTable()
		case opSELF:
			t, ok := reg[b].(*Table)
			if !ok {
				return nil, fmt.Errorf("lua51: pc %d: method of non-table", pc)
			}
			reg[a+1] = t
			reg[a] = t.Get(rk(c))
		case opCALL:
			f, ok := reg[a].(GoFunc)
			if !ok {
				return globals, &PartialError{PC: pc, Op: op}
			}
			if b == 0 || c == 0 {
				return nil, fmt.Errorf("lua51: pc %d: variable-argument call", pc)
			}
			args := make([]Value, b-1)
			copy(args, reg[a+1:a+b])
			out := f(args)
			for i := uint32(0); i+1 < c; i++ {
				var v Value
				if int(i) < len(out) {
					v = out[i]
				}
				reg[a+i] = v
			}
		case opUNM:
			f, ok := rk(b).(float64)
			if !ok {
				return nil, fmt.Errorf("lua51: pc %d: unary minus on non-number", pc)
			}
			reg[a] = -f
		case opSETLIST:
			t, ok := reg[a].(*Table)
			if !ok || b == 0 {
				return nil, fmt.Errorf("lua51: pc %d: unsupported SETLIST", pc)
			}
			if c == 0 {
				pc++
				if pc >= len(code) {
					return nil, fmt.Errorf("lua51: truncated SETLIST")
				}
				c = code[pc]
			}
			base := (int(c) - 1) * fieldsPerFlush
			for i := 1; i <= int(b); i++ {
				t.set(float64(base+i), reg[int(a)+i])
			}
		case opCLOSURE:
			if int(bx) >= len(p.Protos) {
				return nil, fmt.Errorf("lua51: proto %d out of range", bx)
			}
			sub := p.Protos[bx]
			reg[a] = &Function{Proto: sub}
			pc += sub.NumUpvals // skip upvalue pseudo-instructions
		case opJMP:
			pc += int(bx) - 131071
		case opRETURN:
			return globals, nil
		default:
			return globals, &PartialError{PC: pc, Op: op}
		}
		if int(a) >= len(reg)-1 {
			return nil, fmt.Errorf("lua51: register %d out of range", a)
		}
	}
	return globals, nil
}

// Load undumps and runs a chunk, returning its globals. Malformed input
// (bad register or constant indexes, wrong types) returns an error, never panics.
func Load(b []byte) (g map[string]Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			g, err = nil, fmt.Errorf("lua51: malformed chunk: %v", r)
		}
	}()
	return LoadEnv(b, map[string]Value{})
}

// LoadEnv is Load with a shared globals map (see RunEnv).
func LoadEnv(b []byte, globals map[string]Value) (g map[string]Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			g, err = nil, fmt.Errorf("lua51: malformed chunk: %v", r)
		}
	}()
	p, err := Undump(b)
	if err != nil {
		return nil, err
	}
	return RunEnv(p, globals)
}
