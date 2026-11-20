//go:build !windows

package main

import (
	"fmt"
	"os"
)

// Outside Windows there is no program window (it is built for Windows).

func fatal(msg string) { fmt.Fprintln(os.Stderr, "ERRO:", msg) }

func singleInstance() bool { return true }

func focusExisting() {}

// pickFolder: no native picker here.
var pickFolder func() (string, error)
