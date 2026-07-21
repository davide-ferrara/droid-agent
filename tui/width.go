package tui

import (
	"unicode/utf8"

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
	row, col = 0, 0
	rowStart := 0
	for i := 0; i < idx; i++ {
		r := buf[i]
		w := runeWidth(r)
		if w == 0 {
			continue
		}
		if r == ' ' {
			if col+1 > cols {
				row++
				rowStart = i + 1
				col = 0
				continue
			}
			col++
			continue
		}
		if col+w > cols {
			// Scan backward from i-1 to rowStart for a space.
			spaceAt := -1
			for j := i - 1; j >= rowStart; j-- {
				if buf[j] == ' ' {
					spaceAt = j
					break
				}
			}
			if spaceAt >= 0 {
				row++
				rowStart = spaceAt + 1
				col = 0
				for j := rowStart; j <= i; j++ {
					col += runeWidth(buf[j])
				}
				continue
			}
			row++
			rowStart = i
			col = w
			continue
		}
		col += w
		if col == cols && r != ' ' {
			// Exact fill: scan backward for a space and move the
			// trailing word to the next row. r is never a space
			// here (caught above), so start at i-1.
			spaceAt := -1
			for j := i - 1; j >= rowStart; j-- {
				if buf[j] == ' ' {
					spaceAt = j
					break
				}
			}
			if spaceAt >= 0 {
				row++
				rowStart = spaceAt + 1
				col = 0
				for j := rowStart; j <= i; j++ {
					col += runeWidth(buf[j])
				}
			}
		}
	}
	if col == cols {
		row++
		col = 0
	}
	return row, col
}

// wrapMessage splits text into display rows, each at most cols
// columns wide, applying the same width math as wrapRow but
// WITHOUT the virtual-cursor rule (a message has no cursor to
// blink, so a row that's filled exactly is just done; it does
// not get a phantom trailing empty row). Wide runes that don't
// fit wrap to the next row, padding the previous row with a
// trailing blank so the gap is visible. Combining marks append
// their UTF-8 bytes after the base rune without advancing the
// column.
//
// Returns the byte content of each row so messagesToBuf can
// write them directly into the screen. An empty text returns a
// single empty row slot so callers can always index [0].
func wrapMessage(text string, cols int) [][]byte {
	if cols <= 0 {
		return [][]byte{[]byte(text)}
	}
	rows := [][]byte{{}}
	col := 0
	for _, r := range text {
		w := runeWidth(r)
		if w == 0 {
			rows[len(rows)-1] = utf8.AppendRune(rows[len(rows)-1], r)
			continue
		}
		if col+w > cols {
			// Pad the previous row's trailing blank gap left
			// by a wide rune that didn't fit, then start a
			// fresh row.
			last := rows[len(rows)-1]
			for len(last) < cols {
				last = append(last, ' ')
			}
			rows[len(rows)-1] = last
			rows = append(rows, []byte{})
			col = 0
		}
		// Pad up to col so earlier wide-rune gaps inside the
		// same row resolve to blanks rather than the next
		// rune's leading content.
		last := rows[len(rows)-1]
		for len(last) < col {
			last = append(last, ' ')
		}
		rows[len(rows)-1] = last
		rows[len(rows)-1] = utf8.AppendRune(rows[len(rows)-1], r)
		col += w
	}
	return rows
}

// messageRows reports the number of display rows text occupies
// at the given column count. Shortcut for len(wrapMessage) that
// avoids building the byte slices; useful for clampScroll's
// backward-fill which only needs the count.
func messageRows(text string, cols int) int {
	if cols <= 0 {
		return 1
	}
	row, col := 0, 0
	for _, r := range text {
		w := runeWidth(r)
		if w == 0 {
			continue
		}
		if col+w > cols {
			row++
			col = 0
		}
		col += w
	}
	// Always at least one row (even for empty text).
	return row + 1
}