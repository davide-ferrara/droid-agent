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
		// Skip the trailing ESC[K for the status row: it already
		// embeds ClearLine inside its styled run so a second
		// clear would run after \033[0m and erase the cyan bg
		// fill that the embedded ESC[K just laid down.
		if i != statusRow {
			term.ClearCurrentLine()
		}
		if dirty != nil && i < len(dirty) {
			dirty[i] = false
		}
	}
	term.ShowCursor()
}