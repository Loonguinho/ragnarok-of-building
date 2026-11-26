//go:build js

package view

import (
	"sync/atomic"
	"syscall/js"
	"time"
)

// In the browser Go runs on one thread without preemption: a search that
// never blocks keeps the page from drawing until it ends. yield, called
// once per candidate, gives the browser one animation frame (or 50 ms when
// the tab is hidden and frames stop) after each ~30 ms of work.

const (
	yieldEvery = 30 * time.Millisecond
	yieldMax   = 50 * time.Millisecond
)

var (
	lastYield atomic.Int64 // unix nanos of the last yield
	frameCh   = make(chan struct{}, 1)
	// onFrame is never released: a frame asked for may come after the
	// timeout.
	onFrame = js.FuncOf(func(js.Value, []js.Value) any {
		select {
		case frameCh <- struct{}{}:
		default:
		}
		return nil
	})
)

func yield() {
	now := time.Now().UnixNano()
	last := lastYield.Load()
	if now-last < int64(yieldEvery) || !lastYield.CompareAndSwap(last, now) {
		return
	}
	select {
	case <-frameCh: // a stale frame from an earlier timeout
	default:
	}
	js.Global().Call("requestAnimationFrame", onFrame)
	select {
	case <-frameCh:
	case <-time.After(yieldMax):
	}
	lastYield.Store(time.Now().UnixNano())
}
