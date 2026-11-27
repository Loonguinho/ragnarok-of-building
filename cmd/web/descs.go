//go:build js

package main

import (
	"bytes"
	"errors"
	"log/slog"
	"syscall/js"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/view"
)

// itemDescs is one language's item descriptions, read and waiting for the
// frame goroutine to put them in the catalog.
type itemDescs struct {
	lang  string
	descs gamedata.ItemDescs
}

// loadItemDescs downloads and reads lang's item descriptions (written
// apart from the catalog by tools/websplit), pausing so the page keeps
// drawing, and hands them over. Until then the tooltips have no text; if
// they never come, they stay so.
func loadItemDescs(lang string, out chan<- itemDescs, wake func(), log *slog.Logger) {
	b, err := fetch(pageURL("desc-"+lang+".json.gz?v="+version), func(int64) {})
	if err != nil {
		log.Warn("item descriptions", "lang", lang, "err", err)
		return
	}
	timing("item descriptions downloaded (" + lang + ")")
	d, err := gamedata.ReadItemDescs(bytes.NewReader(b), view.Yield)
	if err != nil {
		log.Warn("item descriptions", "lang", lang, "err", err)
		return
	}
	out <- itemDescs{lang, d}
	wake()
}

// errNoPromise: the page started no download (an older index.html).
var errNoPromise = errors.New("no download started by the page")

// awaitBytes waits for a JS promise of an ArrayBuffer and copies it.
func awaitBytes(p js.Value) ([]byte, error) {
	if p.Type() != js.TypeObject || p.Get("then").Type() != js.TypeFunction {
		return nil, errNoPromise
	}
	type result struct {
		b   []byte
		err error
	}
	ch := make(chan result, 1)
	ok := js.FuncOf(func(_ js.Value, args []js.Value) any {
		buf := js.Global().Get("Uint8Array").New(args[0])
		b := make([]byte, buf.Get("length").Int())
		js.CopyBytesToGo(b, buf)
		ch <- result{b: b}
		return nil
	})
	fail := js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "download failed"
		if len(args) > 0 {
			msg = args[0].Call("toString").String()
		}
		ch <- result{err: errors.New(msg)}
		return nil
	})
	defer ok.Release()
	defer fail.Release()
	p.Call("then", ok, fail)
	r := <-ch
	return r.b, r.err
}
