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

// inputHeight is the number of rows the input area occupies
// given the rune widths of its buffer; the chat area shrinks by
// the same count. Floors at 1. Walks runes with wrapRow so wide
// runes (CJK, emoji) and combining marks count correctly.
func inputHeight(m *Model) int {
	if m.TermCols <= 0 || len(m.Input.buf) == 0 {
		return 1
	}
	row, _ := wrapRow(m.Input.buf, len(m.Input.buf), m.TermCols)
	return row + 1
}

func inputRow(m *Model) int      { return m.TermRows - statusBarHeight - inputHeight(m) }
func statusBarRow(termRows int) int { return termRows - statusBarHeight }
func chatAreaRows(m *Model) int  { return m.TermRows - inputHeight(m) - statusBarHeight }