package web

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	l := newRateLimiter(1, 3)
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !l.allow("a", now) {
			t.Fatalf("burst request %d refused", i)
		}
	}
	if l.allow("a", now) {
		t.Error("4th request in the same instant allowed")
	}
	if !l.allow("b", now) {
		t.Error("another client blocked")
	}
	if !l.allow("a", now.Add(1100*time.Millisecond)) {
		t.Error("token not refilled after a second")
	}
}
