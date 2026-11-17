// Package lua51 loads compiled Lua 5.1 chunks (.lub files from the RO client)
// and evaluates the subset of bytecode used by data files: building tables
// and assigning globals. It is not a general Lua interpreter.
package lua51

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Proto is a decoded function prototype.
type Proto struct {
	Source    string
	NumUpvals int
	Code      []uint32
	Consts    []Value
	Protos    []*Proto
}

var header = []byte{0x1b, 'L', 'u', 'a', 0x51, 0x00, 0x01, 4, 4, 4, 8, 0}

type reader struct {
	b   []byte
	off int
	err error
}

func (r *reader) need(n int) bool {
	if r.err != nil {
		return false
	}
	if n < 0 || r.off+n > len(r.b) {
		r.err = fmt.Errorf("lua51: truncated chunk at offset %d", r.off)
		return false
	}
	return true
}

func (r *reader) byte() byte {
	if !r.need(1) {
		return 0
	}
	v := r.b[r.off]
	r.off++
	return v
}

func (r *reader) u32() uint32 {
	if !r.need(4) {
		return 0
	}
	v := binary.LittleEndian.Uint32(r.b[r.off:])
	r.off += 4
	return v
}

func (r *reader) count() int {
	n := r.u32()
	if int64(n) > int64(len(r.b)) { // a count can never exceed the file size
		r.err = fmt.Errorf("lua51: implausible count %d at offset %d", n, r.off-4)
		return 0
	}
	return int(n)
}

func (r *reader) str() (string, bool) {
	n := r.count()
	if n == 0 || !r.need(n) {
		return "", false
	}
	s := string(r.b[r.off : r.off+n-1]) // drop trailing NUL
	r.off += n
	return s, true
}

// Undump decodes a compiled Lua 5.1 chunk.
func Undump(b []byte) (*Proto, error) {
	if !bytes.HasPrefix(b, header) {
		return nil, errors.New("lua51: not a little-endian Lua 5.1 chunk")
	}
	r := &reader{b: b, off: len(header)}
	p := r.function(0)
	if r.err != nil {
		return nil, r.err
	}
	return p, nil
}

func (r *reader) function(depth int) *Proto {
	if depth > 200 {
		r.err = errors.New("lua51: prototypes nested too deeply")
		return nil
	}
	p := &Proto{}
	p.Source, _ = r.str()
	r.u32() // linedefined
	r.u32() // lastlinedefined
	p.NumUpvals = int(r.byte())
	r.byte() // numparams
	r.byte() // is_vararg
	r.byte() // maxstacksize

	n := r.count()
	if r.need(n * 4) {
		p.Code = make([]uint32, n)
		for i := range p.Code {
			p.Code[i] = r.u32()
		}
	}

	n = r.count()
	p.Consts = make([]Value, 0, n)
	for i := 0; i < n && r.err == nil; i++ {
		switch t := r.byte(); t {
		case 0:
			p.Consts = append(p.Consts, nil)
		case 1:
			p.Consts = append(p.Consts, r.byte() != 0)
		case 3:
			p.Consts = append(p.Consts, math.Float64frombits(uint64(r.u32())|uint64(r.u32())<<32))
		case 4:
			s, _ := r.str()
			p.Consts = append(p.Consts, s)
		default:
			r.err = fmt.Errorf("lua51: unknown constant type %d", t)
		}
	}

	n = r.count()
	for i := 0; i < n && r.err == nil; i++ {
		p.Protos = append(p.Protos, r.function(depth+1))
	}

	// Debug info: skipped.
	n = r.count()
	r.need(n * 4)
	r.off += n * 4
	n = r.count()
	for i := 0; i < n && r.err == nil; i++ {
		r.str()
		r.u32()
		r.u32()
	}
	n = r.count()
	for i := 0; i < n && r.err == nil; i++ {
		r.str()
	}
	return p
}
