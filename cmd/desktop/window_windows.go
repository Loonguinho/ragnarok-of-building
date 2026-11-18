package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

// browsers that support app mode (a window without tabs or address bar).
func browserCandidates() []string {
	var out []string
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"} {
		base := os.Getenv(env)
		if base == "" {
			continue
		}
		out = append(out,
			filepath.Join(base, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(base, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(base, `BraveSoftware\Brave-Browser\Application\brave.exe`),
		)
	}
	return out
}

// openWindow opens url in an app-mode window and, unless detach, waits until
// the user closes it. A private profile under the app's folder makes the
// browser start its own process, so waiting on it works; it also keeps the
// app's window size and zoom apart from the user's browsing.
func openWindow(dir, url string, detach bool) error {
	if nativeWindow(dir, url) {
		return nil
	}
	for _, b := range browserCandidates() {
		if _, err := os.Stat(b); err != nil {
			continue
		}
		cmd := exec.Command(b,
			"--app="+url,
			"--user-data-dir="+filepath.Join(dir, "window"),
			"--no-first-run", "--no-default-browser-check",
			"--disable-features=Translate",
			"--window-size=1440,900",
		)
		if err := cmd.Start(); err != nil {
			continue
		}
		if detach {
			return nil
		}
		return cmd.Wait()
	}
	// No app-mode browser: open the default browser and keep serving until
	// the user answers the dialog.
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	if detach {
		return nil
	}
	messageBox("Ragnarok of Building",
		"O simulador abriu no seu navegador.\n\nDeixe esta mensagem aberta enquanto estiver usando. Clique em OK para fechar o simulador.", 0x40)
	return nil
}

func messageBox(title, text string, flags uintptr) {
	user32 := syscall.NewLazyDLL("user32.dll")
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags)
}

func fatal(err error) {
	messageBox("Ragnarok of Building", "Não foi possível iniciar o simulador:\n\n"+err.Error(), 0x10)
	os.Exit(1)
}

func errorf(format string, a ...any) error { return fmt.Errorf(format, a...) }
