package tui

// Screen layout (0-indexed rows):
//
//	0 .. TermRows-3:  content area
//	TermRows-2:       input line
//	TermRows-1:       status bar

func inputRow(termRows int) int { return termRows - 2 }
func statusBarRow(termRows int) int { return termRows - 1 }
