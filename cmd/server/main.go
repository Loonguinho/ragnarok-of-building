package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"ragnarok-of-building/internal/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv, err := web.New(log)
	if err != nil {
		log.Error("init", "err", err)
		os.Exit(1)
	}
	hs := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Info("listening", "addr", addr)
	if err := hs.ListenAndServe(); err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}
