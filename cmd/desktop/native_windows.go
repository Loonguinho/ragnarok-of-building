package main

import (
	"path/filepath"
	"runtime"

	webview2 "github.com/jchv/go-webview2"
)

// The native window must run on the main OS thread.
func init() { runtime.LockOSThread() }

// nativeWindow shows the app in its own Windows window (WebView2, the
// component Windows apps use to draw web UI; no browser involved). It blocks
// until the window closes and returns false when WebView2 is unavailable.
func nativeWindow(dir, url string) bool {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(dir, "webview2"),
		WindowOptions: webview2.WindowOptions{
			Title: "Ragnarok of Building", Width: 1440, Height: 900, Center: true,
		},
	})
	if w == nil {
		return false
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
	return true
}
