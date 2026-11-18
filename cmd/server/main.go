package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ragnarok-of-building/internal/config"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_ = config.LoadDotEnv(".env")

	// Hosts (Render, Koyeb, Fly...) give the port in PORT.
	addr := env("ADDR", ":"+env("PORT", "8080"))
	catalogPath := env("CATALOG", "data/gen/catalog.json.gz")
	dev := os.Getenv("DEV") == "1"

	cat, err := gamedata.Load(catalogPath)
	if err != nil {
		log.Warn("no game catalog; run `go run ./cmd/import`", "path", catalogPath, "err", err)
		cat = nil
	} else {
		log.Info("catalog loaded", "items", len(cat.Items), "skills", len(cat.Skills), "jobs", len(cat.Jobs))
	}

	sounds := env("SOUNDS", filepath.Join(filepath.Dir(catalogPath), "sounds"))
	srv, err := web.New(web.Options{Log: log, Catalog: cat, Dev: dev, SoundDir: sounds})
	if err != nil {
		log.Error("init", "err", err)
		os.Exit(1)
	}
	hs := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second, // replay uploads (up to 16 MB)
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		log.Info("listening", "addr", addr, "dev", dev)
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server", "err", err)
			os.Exit(1)
		}
	}()
	// Hosts stop the app with SIGTERM: finish the requests in flight.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	hs.Shutdown(ctx)
	log.Info("stopped")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
