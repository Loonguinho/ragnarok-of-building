//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Outside Windows the program keeps a terminal: messages go there and it
// runs until Ctrl+C.

func fatal(msg string) { fmt.Fprintln(os.Stderr, "ERRO:", msg) }

func singleInstance() bool { return true }

func focusExisting() {}

// pickFolder: no native picker here (the page asks for the path).
var pickFolder func() (string, error)

func waitBrowserMode(url string) {
	fmt.Println("Pronto! O simulador está aberto em:", url)
	fmt.Println("Para sair, aperte Ctrl+C.")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}

func releaseInstance() {}
