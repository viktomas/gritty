package main

import "github.com/viktomas/gritty/controller"

func main() {
	shell := "/bin/sh"
	controller := &controller.Controller{}
	StartGui(shell, controller)
}
