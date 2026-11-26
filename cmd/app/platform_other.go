//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Outside Windows: no single-instance check, no native folder picker, and
// the file dialog is zenity or kdialog when one is installed.

func fatal(msg string) { fmt.Fprintln(os.Stderr, "ERRO:", msg) }

func singleInstance() bool { return true }

func focusExisting() {}

// pickFolder: no native picker here.
var pickFolder func() (string, error)

// darkTitleBar: the title bar follows the desktop's theme.
func darkTitleBar(bool) {}

// openFileDialog asks for a file to open with zenity or kdialog; filter is
// pairs of name and pattern separated by "|" ("Replays (*.rrf)|*.rrf").
func openFileDialog(title, filter string) (string, error) {
	parts := strings.Split(filter, "|")
	var cmd *exec.Cmd
	if p, err := exec.LookPath("zenity"); err == nil {
		args := []string{"--file-selection", "--title=" + title}
		for i := 0; i+1 < len(parts); i += 2 {
			pats := strings.ReplaceAll(parts[i+1], ";", " ")
			args = append(args, "--file-filter="+parts[i]+" | "+pats)
		}
		cmd = exec.Command(p, args...)
	} else if p, err := exec.LookPath("kdialog"); err == nil {
		var f []string
		for i := 0; i+1 < len(parts); i += 2 {
			f = append(f, strings.ReplaceAll(parts[i+1], ";", " ")+"|"+parts[i])
		}
		cmd = exec.Command(p, "--title", title, "--getopenfilename", ".", strings.Join(f, "\n"))
	} else {
		return "", errors.New("instale o zenity ou o kdialog para escolher arquivos")
	}
	out, err := cmd.Output()
	path := strings.TrimSpace(string(out))
	if err != nil || path == "" {
		return "", errors.New("nenhum arquivo escolhido")
	}
	return path, nil
}
