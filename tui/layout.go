package tui

// Screen layout (0-indexed rows):
//
//	0 .. inputRow-1:        content area (messages)
//	inputRow .. (s-1):      multi-line input (inputHeight rows; grows on wrap)
//	statusBarRow:          status bar
//
// inputHeight grows with the wrapped input buffer so the chat
// area shrinks automatically as the user types past the right
// edge. statusBarRow stays pinned to the bottom regardless.

// statusBarHeight is the number of rows the status bar occupies.
const statusBarHeight = 1

// inputHeight is the number of rows the input area occupies,
// derived from the byte length of the input buffer so the chat
// area shrinks exactly as the buffer wraps. Floors at 1.
// TODO: byte-based math is ASCII-only; switch to rune-width-aware
// truncation when HandleLine accepts runes.
func inputHeight(m *Model) int {
	cols := m.TermCols
	if cols <= 0 || len(m.Input.buf) == 0 {
		return 1
	}
	return (len(m.Input.buf)-1)/cols + 1
}

func inputRow(m *Model) int      { return m.TermRows - statusBarHeight - inputHeight(m) }
func statusBarRow(termRows int) int { return termRows - statusBarHeight }
func chatAreaRows(m *Model) int  { return m.TermRows - inputHeight(m) - statusBarHeight }