package tui

import (
	"github.com/mattn/go-runewidth"
)

// runeWidth returns the display column width of r:
//
//	0 = combining mark (does not advance the cursor)
//	1 = normal character (Latin, precomposed Italian accents)
//	2 = wide character (CJK, emoji)
//
// Delegates to go-runewidth so we inherit its full Unicode
// East-Asian-Width table, ZWJ / variation-selector handling,
// and CJK-terminal detection via LANG / TERM.
func runeWidth(r rune) int {
	return runewidth.RuneWidth(r)
}

// truncateToCols returns the byte offset into s that fits within
// the given display columns, walking runes instead of bytes so a
// multi-byte UTF-8 char is never cut mid-rune. Wide runes (CJK,
// emoji) count as 2 columns; combining marks as 0. A wide rune
// that does not fit in the remaining column budget stops the walk
// — we leave that column blank rather than splitting the rune.
func truncateToCols(s string, cols int) (endByte, usedCols int) {
	for i, r := range s {
		w := runeWidth(r)
		if usedCols+w > cols {
			return i, usedCols
		}
		usedCols += w
		endByte = i + len(string(r))
	}
	return endByte, usedCols
}

// wrapRow walks runes [0, idx) of buf accumulating rune display
// widths, wrapping to the next row when a rune would overflow
// cols. Returns the (row, col) of the screen cell where the rune
// at index idx sits (or, when idx == len(buf), where the cursor
// would land after the last rune).
//
// Rules:
//
//	- Combining marks (runeWidth==0) do not advance col.
//	- Wide runes that don't fit in the remaining cols advance
//	  to the next row at col 0, leaving the trailing cell(s)
//	  of the previous row blank (matches inputLineToBuf).
//	- After the walk, if col == cols the cursor visually sits at
//	  the start of the next row (a fully-filled row wraps a
//	  virtual cursor to row+1 col 0).
//
// Caller is responsible for ensuring 0 <= idx <= len(buf) and
// cols > 0; on bad input it returns the zero position.
func wrapRow(buf []rune, idx int, cols int) (row, col int) {
	if cols <= 0 || idx < 0 {
		return 0, 0
	}
	if idx > len(buf) {
		idx = len(buf)
	}
	for i := 0; i < idx; i++ {
		w := runeWidth(buf[i])
		if w == 0 {
			continue
		}
		if col+w > cols {
			row++
			col = 0
		}
		col += w
	}
	if col == cols {
		row++
		col = 0
	}
	return row, col
}