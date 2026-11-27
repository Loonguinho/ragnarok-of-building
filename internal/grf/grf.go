// Package grf reads Ragnarok Online GRF archives (versions 0x200 and 0x300).
// Encrypted (DES) entries are not supported; data files the simulator needs
// are stored as plain zlib in current LATAM clients.
package grf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/text/encoding/korean"
)

const headerSize = 46

const (
	flagFile     = 0x01
	flagMixCrypt = 0x02
	flagDES      = 0x04
)

// Entry is one file inside the archive.
type Entry struct {
	Name     string // UTF-8 path, backslash separated, as stored
	CompSize uint32
	Aligned  uint32
	Size     uint32
	Flags    byte
	Offset   uint64
}

// Encrypted reports whether the entry uses GRF DES encryption.
func (e Entry) Encrypted() bool { return e.Flags&(flagMixCrypt|flagDES) != 0 }

// Archive is an open GRF file. Read is safe for concurrent use.
type Archive struct {
	f       *os.File
	Version uint32
	entries map[string]Entry // key: normalized path
}

// Open reads the header and file table.
func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	a, err := parse(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return a, nil
}

func parse(f *os.File) (*Archive, error) {
	h := make([]byte, headerSize)
	if _, err := io.ReadFull(f, h); err != nil {
		return nil, fmt.Errorf("grf: header: %w", err)
	}
	if !bytes.HasPrefix(h, []byte("Master of Magic")) && !bytes.HasPrefix(h, []byte("Event Horizon")) {
		return nil, errors.New("grf: not a GRF file")
	}
	version := binary.LittleEndian.Uint32(h[42:])
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}

	var tableOff int64
	var count int
	var offSize int
	switch version {
	case 0x200:
		tableOff = int64(binary.LittleEndian.Uint32(h[30:]))
		seed := binary.LittleEndian.Uint32(h[34:])
		count = int(binary.LittleEndian.Uint32(h[38:])) - int(seed) - 7
		offSize = 4
	case 0x300:
		tableOff = int64(binary.LittleEndian.Uint64(h[30:])) + 4 // 4 unknown bytes precede the table
		count = int(binary.LittleEndian.Uint32(h[38:]))
		offSize = 8
	default:
		return nil, fmt.Errorf("grf: unsupported version 0x%x", version)
	}

	pos := headerSize + tableOff
	if pos < headerSize || pos+8 > st.Size() || count < 0 {
		return nil, errors.New("grf: table offset out of range")
	}
	var th [8]byte
	if _, err := f.ReadAt(th[:], pos); err != nil {
		return nil, fmt.Errorf("grf: table header: %w", err)
	}
	comp := int64(binary.LittleEndian.Uint32(th[:]))
	size := int64(binary.LittleEndian.Uint32(th[4:]))
	if pos+8+comp > st.Size() || size > 512<<20 {
		return nil, errors.New("grf: table size out of range")
	}
	zr, err := zlib.NewReader(io.NewSectionReader(f, pos+8, comp))
	if err != nil {
		return nil, fmt.Errorf("grf: table: %w", err)
	}
	table := make([]byte, size)
	if _, err := io.ReadFull(zr, table); err != nil {
		return nil, fmt.Errorf("grf: table: %w", err)
	}

	dec := korean.EUCKR.NewDecoder()
	a := &Archive{f: f, Version: version, entries: make(map[string]Entry, count)}
	fixed := 13 + offSize
	for p := 0; p < len(table) && len(a.entries) < count; {
		end := bytes.IndexByte(table[p:], 0)
		if end < 0 || p+end+1+fixed > len(table) {
			return nil, fmt.Errorf("grf: truncated table entry at %d", p)
		}
		rawName := table[p : p+end]
		m := table[p+end+1:]
		e := Entry{
			CompSize: binary.LittleEndian.Uint32(m[0:]),
			Aligned:  binary.LittleEndian.Uint32(m[4:]),
			Size:     binary.LittleEndian.Uint32(m[8:]),
			Flags:    m[12],
		}
		if offSize == 8 {
			e.Offset = binary.LittleEndian.Uint64(m[13:])
		} else {
			e.Offset = uint64(binary.LittleEndian.Uint32(m[13:]))
		}
		if name, err := dec.Bytes(rawName); err == nil {
			e.Name = string(name)
		} else {
			e.Name = string(rawName)
		}
		p += end + 1 + fixed
		if e.Flags&flagFile == 0 {
			continue // directory entry
		}
		a.entries[normalize(e.Name)] = e
	}
	return a, nil
}

func normalize(p string) string {
	return strings.ToLower(strings.ReplaceAll(p, "/", `\`))
}

// Close closes the underlying file.
func (a *Archive) Close() error { return a.f.Close() }

// Len returns the number of files.
func (a *Archive) Len() int { return len(a.entries) }

// Lookup finds an entry by path (case-insensitive, / or \).
func (a *Archive) Lookup(path string) (Entry, bool) {
	e, ok := a.entries[normalize(path)]
	return e, ok
}

// Find returns entries whose path contains substr (case-insensitive), sorted.
func (a *Archive) Find(substr string) []Entry {
	s := normalize(substr)
	var out []Entry
	for k, e := range a.entries {
		if strings.Contains(k, s) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Read returns the decompressed contents of a file.
func (a *Archive) Read(path string) ([]byte, error) {
	e, ok := a.Lookup(path)
	if !ok {
		return nil, fmt.Errorf("grf: %s: not found", path)
	}
	return a.ReadEntry(e)
}

// ReadEntry returns the decompressed contents of an entry.
func (a *Archive) ReadEntry(e Entry) ([]byte, error) {
	if e.Size > 256<<20 || e.Aligned > 256<<20 {
		return nil, fmt.Errorf("grf: %s: implausible size", e.Name)
	}
	var r io.Reader = io.NewSectionReader(a.f, headerSize+int64(e.Offset), int64(e.Aligned))
	if e.Encrypted() {
		raw := make([]byte, e.Aligned)
		if _, err := io.ReadFull(r, raw); err != nil {
			return nil, fmt.Errorf("grf: %s: %w", e.Name, err)
		}
		unscramble(raw, e.Flags, e.CompSize)
		r = bytes.NewReader(raw)
	} else if e.CompSize == e.Size && e.Size > 0 {
		// Some tools store files uncompressed.
		b := make([]byte, e.Size)
		if _, err := io.ReadFull(r, b); err == nil && !bytes.HasPrefix(b, []byte{0x78}) {
			return b, nil
		}
		r = io.NewSectionReader(a.f, headerSize+int64(e.Offset), int64(e.Aligned))
	}
	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("grf: %s: %w", e.Name, err)
	}
	defer zr.Close()
	b := make([]byte, e.Size)
	if _, err := io.ReadFull(zr, b); err != nil {
		return nil, fmt.Errorf("grf: %s: %w", e.Name, err)
	}
	return b, nil
}
