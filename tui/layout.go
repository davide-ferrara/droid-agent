package tui

// Screen layout (0-indexed rows):
//
//	0 .. chatAreaRows-1:               content area (messages)
//	chatAreaRows .. inputRow-1:        reserved for future input growth
//	inputRow:                          input line
//	statusBarRow:                      status bar
//
// All values are derived from termRows so that a single resize
// only needs to recompute one place. When the input line grows
// (multi-line editing), inputHeight returns >1 and the chat
// area shrinks automatically.

// inputHeight is the number of rows the input line occupies.
// For now 1; will grow with multi-line editing.
const inputHeight = 1

// statusBarHeight is the number of rows the status bar occupies.
const statusBarHeight = 1

func inputRow(termRows int) int { return termRows - statusBarHeight - inputHeight }
func statusBarRow(termRows int) int { return termRows - statusBarHeight }
func chatAreaRows(termRows int) int { return termRows - inputHeight - statusBarHeight }