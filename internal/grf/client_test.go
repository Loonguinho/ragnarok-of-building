package grf

import (
	"bytes"
	"path/filepath"
	"testing"

	"ragnarok-of-building/internal/config"
)

// TestClientScrambled reads every scrambled entry of the client's
// interface pictures (only where a client is set up: RO_CLIENT in .env).
func TestClientScrambled(t *testing.T) {
	dir := config.ClientDir()
	a, err := Open(filepath.Join(dir, "data.grf"))
	if err != nil {
		t.Skip("no client:", err)
	}
	defer a.Close()
	n, bad := 0, 0
	for _, e := range a.Find(`data\texture\`) {
		if !e.Encrypted() || filepath.Ext(e.Name) != ".bmp" {
			continue
		}
		n++
		b, err := a.ReadEntry(e)
		if err != nil || !bytes.HasPrefix(b, []byte("BM")) {
			if bad++; bad <= 5 {
				t.Errorf("%s: %v", e.Name, err)
			}
		}
	}
	if n == 0 {
		t.Skip("no scrambled pictures in", dir)
	}
	t.Logf("%d scrambled pictures, %d unreadable", n, bad)
}

func TestSubstituteUndoesItself(t *testing.T) {
	for c := range 256 {
		if got := substitute(substitute(byte(c))); got != byte(c) {
			t.Errorf("%#x -> %#x", c, got)
		}
	}
}

