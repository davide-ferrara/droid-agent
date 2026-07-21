// Package tui view for droid
package tui

import (
	"bytes"
	"strconv"
	"unicode/utf8"

	"droid/term"
)

// NOTE: if the screen is too zoomed in the status bar
// will mess the input line if occupies more than 1 line
func statusBarView(model Model) []byte {
	var fixedBuf [256]byte
	buf := fixedBuf[:0]

	buf = append(buf, "\033[38;2;0;0;0m\033[48;2;0;180;244m"...)

	switch model.Mode {
	case ModeIdle:
		buf = append(buf, " ◆ idle "...)
	case ModeLoading:
		buf = append(buf, " ◉ load "...)
	case ModeStreaming:
		buf = append(buf, " ▶ strm "...)
	case ModeError:
		buf = append(buf, " ✖ err  "...)
	}

	buf = append(buf, "│ "...)
	buf = append(buf, model.ModelName...)
	buf = append(buf, " │ "...)
	buf = append(buf, model.Status...)
	if model.Debug {
		buf = append(buf, "  │  "...)
		buf = strconv.AppendInt(buf, int64(model.TermCols), 10)
		buf = append(buf, 'x')
		buf = strconv.AppendInt(buf, int64(model.TermRows), 10)
		buf = append(buf, ' ')
	}
	buf = append(buf, term.ClearLine...)
	buf = append(buf, "\033[0m"...)
	return buf
}

func fillLine(n int, fill byte) []byte {
	return bytes.Repeat([]byte{fill}, n)
}

// sizeOK reports whether the terminal has enough room to render.
// Caller checks this before indexing screen rows.
func sizeOK(model *Model) bool {
	return model.TermRows >= 2 && model.TermCols >= 1
}

// reallocScreenBufs (re)allocates the persistent render buffers on
// the Model so subsequent frames stay allocation-free. On realloc
// the dirty mask is fully set so the first paint covers the whole
// screen; on reuse we just reslice to the new dimensions. The
// per-row input scratches grow with inputHeight (one []byte per
// wrapped input row, distinct backings so they don't alias each
// other or the shared blank).
func reallocScreenBufs(model *Model) {
	if cap(model.screen) < model.TermRows || cap(model.blank) < model.TermCols {
		model.screen = make([][]byte, model.TermRows)
		model.blank = make([]byte, model.TermCols)
		model.dirtyRows = make([]bool, model.TermRows)
		for i := range model.dirtyRows {
			model.dirtyRows[i] = true
		}
	} else {
		model.screen = model.screen[:model.TermRows]
		model.blank = model.blank[:model.TermCols]
		model.dirtyRows = model.dirtyRows[:model.TermRows]
	}
	// inputScratches depends on inputHeight (which depends on
	// buf length and cols), so it can change every frame —
	// resize it outside the capacity-grow branch above.
	reallocInputScratches(model)
}

// maxBytesPerCol is the worst-case byte-to-column ratio for a
// UTF-8 encoded input row. Real input rarely hits it (a row of
// 4-byte emoji uses 2 cols each → 2 bytes/col; 2-byte Latin
// accents use 1 col each → 2 bytes/col); we leave headroom so
// the per-row scratch never needs to grow mid-frame.
const maxBytesPerCol = 4

// reallocInputScratches grows the per-row scratch slice for the
// input area so each wrapped row has its own backing []byte with
// capacity maxBytesPerCol*cols — enough to hold the row's UTF-8
// encoding without reallocating. Reuses (reslicing to len 0)
// existing scratches across frames; allocates fresh only when
// the row count grows or a row's capacity is below the threshold.
func reallocInputScratches(model *Model) {
	need := inputHeight(model)
	if cap(model.inputScratches) < need {
		model.inputScratches = make([][]byte, need)
	} else {
		model.inputScratches = model.inputScratches[:need]
	}
	want := maxBytesPerCol * model.TermCols
	for i := range model.inputScratches {
		if cap(model.inputScratches[i]) < want {
			model.inputScratches[i] = make([]byte, 0, want)
		} else {
			model.inputScratches[i] = model.inputScratches[i][:0]
		}
	}
}

// fillBlanks refills the shared blank row with spaces and points
// every screen row at it. Safe because blank rows are never mutated
// in place — the status and input rows get dedicated scratch rows.
func fillBlanks(model *Model) {
	for i := range model.blank {
		model.blank[i] = ' '
	}
	for i := range model.screen {
		model.screen[i] = model.blank
	}
}

// maxScroll returns the largest valid value for Model.Scroll —
// the smallest index i such that the display rows of messages
// [i..nMsg) fit in chatAreaRows. Edge case: when even the latest
// message alone doesn't fit, it returns nMsg-1 so messagesToBuf
// still renders its bottom rows (top rows scroll out).
//
// Shared by clampScroll (resets Scroll past the end to anchor
// the bottom) and pageDownShift (clamps a forward-page step).
func maxScroll(model *Model) int {
	maxRows := chatAreaRows(model)
	nMsg := len(model.Messages)
	rowsUsed := 0
	i := nMsg
	for i > 0 {
		mr := messageRows(model.Messages[i-1].Text, model.TermCols) + 1 // +1 for blank separator
		if rowsUsed+mr > maxRows {
			break
		}
		rowsUsed += mr
		i--
	}
	if i == nMsg && nMsg > 0 {
		i = nMsg - 1
	}
	return i
}

// clampScroll keeps model.Scroll inside [0, maxScroll]. A Scroll
// past maxScroll (e.g. set to len(Messages) by handleEnter) snaps
// down so the newest message is anchored at the bottom.
//
// Returns the clamped start and end (end always == nMsg; the
// caller iterates messages forward and stops when the chat area
// is full).
func clampScroll(model *Model) (start, end int) {
	nMsg := len(model.Messages)
	if nMsg == 0 {
		model.Scroll = 0
		return 0, 0
	}
	ms := maxScroll(model)
	if model.Scroll < 0 {
		model.Scroll = 0
	}
	if model.Scroll > ms {
		model.Scroll = ms
	}
	return model.Scroll, nMsg
}

// messagesToBuf writes the visible messages into the chat area of
// screen, wrapping each message into display rows via wrapMessage.
// Walks forward from Scroll, writing wrapped rows until either
// the messages run out or the chat area is full. Single messages
// taller than the viewport are truncated at the top by ScreenIdx
// so the latest rows of that message remain visible (rare case;
// keeps the bottom anchored).
func messagesToBuf(model *Model, screen [][]byte) {
	start, end := clampScroll(model)
	maxRows := chatAreaRows(model)
	cols := model.TermCols
	screenIdx := 0
	for i := start; i < end && screenIdx < maxRows; i++ {
		rows := wrapMessage(model.Messages[i].Text, cols)
		// If a single message doesn't fit, drop its leading rows
		// so the bottom-of-message stays visible.
		skip := 0
		if len(rows) > maxRows-screenIdx {
			skip = len(rows) - (maxRows - screenIdx)
		}
		isUser := model.Messages[i].Role == "user"
		for _, r := range rows[skip:] {
			if screenIdx >= maxRows {
				return
			}
			if isUser {
				styled := make([]byte, 0, len(r)+20)
				styled = append(styled, "\033[48;2;42;42;46m"...)
				styled = append(styled, r...)
				styled = append(styled, term.ClearLine...)
				styled = append(styled, "\033[0m"...)
				screen[screenIdx] = styled
			} else {
				screen[screenIdx] = r
			}
			screenIdx++
		}
		// Blank separator between messages (fillBlanks already
		// filled every row with spaces; we just skip one).
		if screenIdx < maxRows {
			screenIdx++
		}
	}
}

// statusBarToBuf writes the status bar into its layout row.
func statusBarToBuf(model *Model, screen [][]byte) {
	screen[statusBarRow(model.TermRows)] = statusBarView(*model)
}

// inputLineToBuf writes the wrapped input area into its layout
// rows. Each wrapped row gets its own dedicated scratch backing
// (model.inputScratches[i]); the scratch is rebuilt per frame by
// walking the rune buffer with the same width math as wrapRow,
// so wide runes that don't fit jump to the next row leaving a
// trailing blank on the previous one. Combining marks (width 0)
// are encoded right after the previous rune so the terminal
// overlays them; they don't advance the column.
//
// Trailing blanks are not written: Render's ESC[K already clears
// everything past the row's bytes, so a scratch only needs to
// hold the actual content up to its used column count (plus
// padding spaces that fill gaps left by wide runes).
func inputLineToBuf(model *Model, screen [][]byte) {
	top := inputRow(model)
	cols := model.TermCols
	buf := model.Input.buf
	scratches := model.inputScratches
	for i := range scratches {
		scratches[i] = scratches[i][:0]
	}
	row, col := 0, 0
	for _, r := range buf {
		w := runeWidth(r)
		if w == 0 {
			// Combining mark: append its UTF-8 bytes right after
			// the previous rune. The terminal renders it on top
			// of the same cell, which is correct.
			if row < len(scratches) {
				scratches[row] = utf8.AppendRune(scratches[row], r)
			}
			continue
		}
		if r == ' ' {
			if col+1 > cols {
				// Space at overflow: consume as word-wrap break.
				row++
				col = 0
				if row >= len(scratches) {
					break
				}
				continue
			}
			if row < len(scratches) {
				scratches[row] = utf8.AppendRune(scratches[row], r)
			}
			col++
			continue
		}
		if col+w > cols {
			// Non-space overflow: try word-wrap at last space.
			if row < len(scratches) {
				s := scratches[row]
				lastSpace := bytes.LastIndexByte(s, ' ')
				if lastSpace >= 0 {
					tail := s[lastSpace+1:]
					scratches[row] = s[:lastSpace]
					row++
					col = 0
					if row < len(scratches) {
						scratches[row] = append(scratches[row], tail...)
						scratches[row] = utf8.AppendRune(scratches[row], r)
					}
					for _, tr := range []rune(string(tail)) {
						col += runeWidth(tr)
					}
					col += w
					continue
				}
				// Hard-wrap: fill trailing blank gap.
				for len(scratches[row]) < cols {
					scratches[row] = append(scratches[row], ' ')
				}
			}
			row++
			col = 0
		}
		if row >= len(scratches) {
			break
		}
		// Pad with spaces up to col so earlier wide-rune gaps
		// inside the same row resolve to blanks rather than the
		// next rune's leading content.
		for len(scratches[row]) < col {
			scratches[row] = append(scratches[row], ' ')
		}
		scratches[row] = utf8.AppendRune(scratches[row], r)
		col += w
		if col == cols && r != ' ' {
			// Exact fill: scan backward for a space and move
			// the trailing word to the next row.
			s := scratches[row]
			if lastSpace := bytes.LastIndexByte(s, ' '); lastSpace >= 0 {
				tail := s[lastSpace+1:]
				scratches[row] = s[:lastSpace]
				row++
				col = 0
				if row < len(scratches) {
					scratches[row] = append(scratches[row], tail...)
				}
				for _, tr := range []rune(string(tail)) {
					col += runeWidth(tr)
				}
			}
		}
	}
	for i := range scratches {
		screen[top+i] = scratches[i]
	}
}

// NewView builds the next screen for Render. It orchestrates the
// four phases — bounds, buffer reuse, content fill, and the three
// row regions (chat / status / input) — keeping each step small
// and separately testable.
func NewView(model *Model) [][]byte {
	if !sizeOK(model) {
		return [][]byte{[]byte("terminal too small (need >=2 rows, >=1 col)")}
	}
	reallocScreenBufs(model)
	fillBlanks(model)
	messagesToBuf(model, model.screen)
	statusBarToBuf(model, model.screen)
	inputLineToBuf(model, model.screen)
	return model.screen
}