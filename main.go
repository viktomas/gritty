package main

import (
	"fmt"
	"os"
)

func logDebug(f string, vars ...any) {
	if os.Getenv("gritty_debug") != "" {
		fmt.Printf(f, vars...)
	}
}

func main() {
	shell := "/bin/sh"
	controller := &Controller{}
	StartGui(shell, controller)
}
