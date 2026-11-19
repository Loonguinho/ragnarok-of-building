package web

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
)

func TestShareCodes(t *testing.T) {
	b := calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Stats: [6]int{120, 1, 90, 1, 1, 1}}
	code := encodeBuild(b)
	if !strings.HasPrefix(code, "z") {
		t.Fatalf("code %q is not compressed", code)
	}
	got, ok := decodeBuild(code)
	if !ok || got.Job != b.Job || got.BaseLv != 250 || got.Stats != b.Stats {
		t.Fatalf("round trip: %+v %v", got, ok)
	}
	// Old links (plain JSON) still open.
	raw, _ := json.Marshal(b)
	old := base64.RawURLEncoding.EncodeToString(raw)
	if got, ok := decodeBuild(old); !ok || got.BaseLv != 250 {
		t.Fatalf("old code: %+v %v", got, ok)
	}
	if len(code) >= len(old) {
		t.Errorf("compressed code is not shorter: %d vs %d", len(code), len(old))
	}
	for _, bad := range []string{"", "z", "z!!", "zAAAA", "not a code"} {
		if _, ok := decodeBuild(bad); ok {
			t.Errorf("%q decoded", bad)
		}
	}
}
