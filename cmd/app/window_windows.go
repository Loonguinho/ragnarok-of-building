//go:build windows

package main

import (
	"path/filepath"
	"runtime"
	"strings"

	webview2 "github.com/jchv/go-webview2"
)

// The program's own window: the simulator in WebView2 (Edge's engine, part
// of Windows 10/11), like a desktop app, instead of the browser.

func init() {
	// The window's message loop must run on the process's main thread: keep
	// the main goroutine there.
	runtime.LockOSThread()
	showWindow = openWindow
}

// externalLinks sends links and window.open calls to other sites (paste.rs,
// Divine Pride, GitHub) to the browser; the simulator's own pages stay here.
const externalLinks = `(() => {
  const ext = (href) => {
    try { const u = new URL(href, location.href); return u.origin !== location.origin && /^https?:$/.test(u.protocol) ? u.href : ""; }
    catch { return ""; }
  };
  document.addEventListener("click", (e) => {
    const a = e.target && e.target.closest ? e.target.closest("a[href]") : null;
    const u = a && ext(a.getAttribute("href"));
    if (u) { e.preventDefault(); window.roOpenExternal(u); }
  }, true);
  const open = window.open;
  window.open = function (href, ...rest) {
    const u = ext(href);
    if (u) { window.roOpenExternal(u); return null; }
    return open.call(window, href, ...rest);
  };
})();`

// openWindow shows url in the program's window and returns when it is
// closed. Without WebView2 it uses a browser's app window
// (appbrowser_windows.go); errNoWindow when neither is available.
func openWindow(url string, debug bool) error {
	data, _ := filepath.Abs(filepath.Join("data", "webview"))
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     debug,
		AutoFocus: true,
		DataPath:  data, // the engine's storage, next to the program
		WindowOptions: webview2.WindowOptions{
			Title: appTitle, Width: 1440, Height: 900, Center: true,
			IconId: 1, // the icon embedded by tools/buildexe
		},
	})
	if w == nil {
		return openAppBrowser(url) // no WebView2: a browser's app window
	}
	w.Bind("roOpenExternal", func(u string) {
		if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
			openBrowser(u)
		}
	})
	w.Init(externalLinks)
	w.Navigate(url)
	w.Run()
	return nil
}
