// Package tui render
package tui

import "droid/term"

// Render writes screen to the terminal. Rows where dirty[i] is
// false are skipped (assumed unchanged since the last frame) and
// their dirty bit is cleared on write. Pass dirty=nil for a
// full repaint (used by the initial frame and resize path).
//
// Each row is followed by ESC[K so stale bytes beyond the row's
// byte length are erased — needed because message rows can shrink
// (scroll to a shorter one) and ClearCurrentLine writes the
// escape after the cursor was advanced past the row by Write.
func Render(screen [][]byte, dirty []bool, statusRow int) {
	term.HideCursor()
	for i := range screen {
		if dirty != nil && i < len(dirty) && !dirty[i] {
			continue
		}
		term.Write(screen[i], i+1, 1)
		// Skip ESC[K for rows that already embed their own
		// clear-to-end-of-line inside ANSI escapes (status bar,
		// user message rows with background tint). Those rows
		// start with \033; plain text or blank rows don't.
		if len(screen[i]) == 0 || screen[i][0] != '\033' {
			term.ClearCurrentLine()
		}
		if dirty != nil && i < len(dirty) {
			dirty[i] = false
		}
	}
	term.ShowCursor()
}