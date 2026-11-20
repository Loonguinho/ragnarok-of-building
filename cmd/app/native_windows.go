//go:build windows

package main

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"

	"ragnarok-of-building/internal/ui"
	"ragnarok-of-building/internal/web"
)

// The native window (Gio): the startup screen, then the simulator drawn by
// internal/ui. No browser engine is needed. The old web version stays
// available with -web, from the settings menu, and as the fallback when
// this window cannot open.

func init() { runNative = nativeMain }

// nativeMain never returns: Gio's app.Main keeps the main thread.
func nativeMain(st *startup, o options, log *slog.Logger) {
	th := ui.NewTheme()
	w := new(app.Window)
	w.Option(app.Title(appTitle), app.Size(unit.Dp(1440), unit.Dp(900)), app.MinSize(unit.Dp(640), unit.Dp(480)))

	var simulator atomic.Pointer[ui.App]
	start := &ui.Startup{Th: th, Pick: pickFolder, UseClient: st.useClient, Status: func() ui.StartupStatus {
		s := st.status()
		return ui.StartupStatus{Lines: s.Lines, Ask: s.Ask, AskNote: s.AskNote, Error: s.Error}
	}}
	st.onChange = w.Invalidate

	go func() {
		srv, err := prepare(st, o, log)
		if err != nil {
			st.fail(err)
			w.Invalidate()
			return
		}
		a := ui.NewApp(srv.Views(), th, filepath.Join("data", "ui.json"))
		a.Icons = ui.NewIcons(web.NewIconCache(filepath.Join("data", "cache", "img")).Get, w.Invalidate)
		a.Invalidate = w.Invalidate
		a.SavedFile = "builds.json"
		a.OpenFile = openFileDialog
		a.OpenWeb = func() { relaunch(log, "-web") }
		simulator.Store(a)
		w.Invalidate()
	}()

	go func() {
		var ops op.Ops
		frames := 0
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				if e.Err != nil {
					log.Error("native window", "err", e.Err)
					if frames == 0 {
						// The window could not open (no Direct3D...): the web version.
						relaunch(log, "-web")
					}
				}
				os.Exit(0)
			case app.FrameEvent:
				frames++
				gtx := app.NewContext(&ops, e)
				if a := simulator.Load(); a != nil {
					a.Frame(gtx)
				} else {
					start.Frame(gtx)
				}
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}

// relaunch starts this program again with args (the web version) and ends
// this one.
func relaunch(log *slog.Logger, args ...string) {
	exe, err := os.Executable()
	if err != nil {
		log.Error("relaunch", "err", err)
		return
	}
	releaseInstance()
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		log.Error("relaunch", "err", err)
		return
	}
	os.Exit(0)
}
