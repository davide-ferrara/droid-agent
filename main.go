package main

import (
	"log"
	"os"

	"droid/term"
	"droid/tui"
)

func initLog() {
	fp, err := os.OpenFile("/tmp/droid.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		os.Exit(1)
	}
	log.SetOutput(fp)
	log.SetFlags(log.Lshortfile)
}

func must(err error) {
	_, _ = os.Stdout.Write([]byte(err.Error()))
	os.Exit(1)
}

func main() {
	initLog()

	rawModeOff, err := term.Init()
	if err != nil {
		must(err)
	}
	defer rawModeOff()

	tui.Run()
}
