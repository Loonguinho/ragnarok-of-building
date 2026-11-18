// Ragnarok of Building as a desktop app: one .exe that keeps its data in
// %LOCALAPPDATA%\RagnarokOfBuilding, prepares the game data on first run
// (finds the RO client, downloads the rAthena tables, builds the catalog)
// and opens the UI in its own window (Edge/Chrome app mode). Closing the
// window quits the app.
//
//	go build -ldflags "-H=windowsgui -s -w" -o dist/RagnarokOfBuilding.exe ./cmd/desktop
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/store"
	"ragnarok-of-building/internal/web"
)

const appName = "RagnarokOfBuilding"

// handlerBox lets atomic.Pointer swap handlers of different concrete types.
type handlerBox struct{ h http.Handler }

type app struct {
	dir     string // %LOCALAPPDATA%\RagnarokOfBuilding
	log     *slog.Logger
	store   *store.Store
	handler atomic.Pointer[handlerBox] // the setup screen, then the full app
	setup   *setupState
	addr    string
}

func main() {
	if err := run(); err != nil {
		fatal(err)
	}
}

func run() error {
	base, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return err
	}
	a := &app{dir: filepath.Join(base, appName)}
	if d := os.Getenv("ROB_DATA"); d != "" { // tests and portable use
		a.dir = d
	}
	if err := os.MkdirAll(a.dir, 0o755); err != nil {
		return err
	}
	logf, _ := os.OpenFile(filepath.Join(a.dir, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	var logw io.Writer = io.Discard
	if logf != nil {
		logw = logf
		defer logf.Close()
	}
	a.log = slog.New(slog.NewTextHandler(logw, nil))

	// One instance: if the app is already running, just open another window.
	if addr := a.running(); addr != "" {
		return openWindow(a.dir, "http://"+addr+"/", true)
	}

	if a.store, err = store.Open(filepath.Join(a.dir, "builds.db")); err != nil {
		return fmt.Errorf("não foi possível abrir os builds salvos: %w", err)
	}
	defer a.store.Close()

	a.setup = newSetupState(a)
	if cat, err := gamedata.Load(a.catalogPath()); err == nil && len(cat.StatPoints) > 0 {
		if err := a.useCatalog(cat); err != nil {
			return err
		}
	} else {
		a.handler.Store(&handlerBox{a.setup.routes()})
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	a.addr = ln.Addr().String()
	os.WriteFile(filepath.Join(a.dir, "port"), []byte(a.addr), 0o644)
	defer os.Remove(filepath.Join(a.dir, "port"))
	srv := &http.Server{Handler: a, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(ln)
	a.log.Info("listening", "addr", a.addr)

	// The window decides the app's lifetime.
	if os.Getenv("ROB_HEADLESS") == "1" { // tests: no window, run until killed
		select {}
	}
	err = openWindow(a.dir, "http://"+a.addr+"/", false)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	return err
}

func (a *app) catalogPath() string { return filepath.Join(a.dir, "catalog.json.gz") }
func (a *app) rathenaDir() string  { return filepath.Join(a.dir, "rathena") }

// useCatalog switches from the setup screen to the full app.
func (a *app) useCatalog(cat *gamedata.Catalog) error {
	srv, err := web.New(web.Options{Log: a.log, Catalog: cat, Store: a.store, Desktop: true})
	if err != nil {
		return err
	}
	a.handler.Store(&handlerBox{srv.Routes()})
	return nil
}

// ServeHTTP only answers requests for the app's own address (no other site
// can reach it through DNS tricks), keeps /setup available for refreshing
// the game data, and hands everything else to the current handler.
func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, port, err := net.SplitHostPort(r.Host); err == nil && (h == "localhost" || h == "127.0.0.1") {
		host = "127.0.0.1:" + port
	}
	if host != a.addr {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.URL.Path == "/healthz" {
		w.Write([]byte("ok"))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/setup") {
		a.setup.routes().ServeHTTP(w, r)
		return
	}
	a.handler.Load().h.ServeHTTP(w, r)
}

// running returns the address of an instance that is already up.
func (a *app) running() string {
	b, err := os.ReadFile(filepath.Join(a.dir, "port"))
	if err != nil {
		return ""
	}
	addr := strings.TrimSpace(string(b))
	c := http.Client{Timeout: 700 * time.Millisecond}
	resp, err := c.Get("http://" + addr + "/healthz")
	if err != nil {
		return ""
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	return addr
}
