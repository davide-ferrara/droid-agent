// Package tui render
package tui

import "droid/term"

func Render(screen [][]byte) {
	term.HideCursor()
	for i := range len(screen) {
		term.Write(screen[i], i+1, 1)
	}
	term.ShowCursor()
}
