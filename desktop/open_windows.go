package main

import "os/exec"

func open(path string) error {
	return exec.Command("cmd", "/c", "start", "", path).Start()
}
