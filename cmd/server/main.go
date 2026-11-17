package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"ragnarok-of-building/internal/config"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_ = config.LoadDotEnv(".env")

	addr := env("ADDR", ":8080")
	catalogPath := env("CATALOG", "data/gen/catalog.json.gz")
	dev := os.Getenv("DEV") == "1"

	cat, err := gamedata.Load(catalogPath)
	if err != nil {
		log.Warn("no game catalog; run `go run ./cmd/import`", "path", catalogPath, "err", err)
		cat = nil
	} else {
		log.Info("catalog loaded", "items", len(cat.Items), "skills", len(cat.Skills), "jobs", len(cat.Jobs))
	}

	srv, err := web.New(web.Options{Log: log, Catalog: cat, Dev: dev})
	if err != nil {
		log.Error("init", "err", err)
		os.Exit(1)
	}
	hs := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Info("listening", "addr", addr, "dev", dev)
	if err := hs.ListenAndServe(); err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
