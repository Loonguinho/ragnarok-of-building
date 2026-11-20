//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// Without WebView2 the program opens in a Chromium browser's app mode
// (Chrome, Edge or Brave with --app): a window of its own, no tabs or
// address bar. A profile of its own (data/browser) keeps that browser
// process tied to the window, so the program ends when the window closes,
// with no message box to keep open.

// appBrowsers are the executables tried, in order.
var appBrowsers = []struct{ exe, dir string }{
	{"chrome.exe", `Google\Chrome\Application`},
	{"msedge.exe", `Microsoft\Edge\Application`},
	{"brave.exe", `BraveSoftware\Brave-Browser\Application`},
}

// findAppBrowser is the path of an installed Chromium browser ("" = none).
func findAppBrowser() string {
	roots := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")}
	for _, b := range appBrowsers {
		for _, r := range roots {
			if r == "" {
				continue
			}
			if p := filepath.Join(r, b.dir, b.exe); fileExists(p) {
				return p
			}
		}
		// Installed elsewhere: the App Paths registration.
		for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
			k, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`+b.exe, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			p, _, err := k.GetStringValue("")
			k.Close()
			if err == nil && fileExists(p) {
				return p
			}
		}
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// openAppBrowser shows url in a browser's app window and returns when it is
// closed; errNoWindow when there is no Chromium browser.
func openAppBrowser(url string) error {
	exe := findAppBrowser()
	if exe == "" {
		return errNoWindow
	}
	profile, _ := filepath.Abs(filepath.Join("data", "browser"))
	cmd := exec.Command(exe,
		"--app="+url,
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--window-size=1440,900",
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}
