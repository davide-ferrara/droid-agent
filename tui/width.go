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