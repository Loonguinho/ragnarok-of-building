package setup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestDownloadRetriesDroppedConnections(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			calls.Add(1)
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 { // the first try: the connection drops
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "f.yml")
	if err := download(context.Background(), srv.Client(), srv.URL+"/f", dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "ok" || calls.Load() != 2 {
		t.Errorf("got %q after %d calls", b, calls.Load())
	}
	// A missing file is not retried.
	calls.Store(0)
	if err := download(context.Background(), srv.Client(), srv.URL+"/missing", dst+"2"); err == nil || calls.Load() != 1 {
		t.Errorf("404: err %v after %d calls", err, calls.Load())
	}
}
