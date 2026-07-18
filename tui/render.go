// package tui render
package tui

import "droid/term"

func Render(buf [][]byte) {
	for i := range len(buf) {
		term.Write(buf[i], i+1, 1)
	}
}
