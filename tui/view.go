// Package tui view for droid
package tui

import (
	"bytes"
	"strconv"

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

// reallocInputScratches grows the per-row scratch slice for the
// input area so each wrapped row has its own [cols]byte backing.
// Reuses (reslicing to cols) when the same row is kept across a
// resize; allocates fresh when the row count grows or a row's
// capacity is below cols.
func reallocInputScratches(model *Model) {
	need := inputHeight(model)
	if cap(model.inputScratches) < need {
		model.inputScratches = make([][]byte, need)
	} else {
		model.inputScratches = model.inputScratches[:need]
	}
	for i := range model.inputScratches {
		if cap(model.inputScratches[i]) < model.TermCols {
			model.inputScratches[i] = make([]byte, model.TermCols)
		} else {
			model.inputScratches[i] = model.inputScratches[i][:model.TermCols]
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

// clampScroll keeps model.Scroll inside [0, max(0, nMsg-chatAreaRows)]
// so a resize, deletion, or new message never leaves a blank
// viewport. Returns the clamped value and the last visible index.
func clampScroll(model *Model) (start, end int) {
	maxRows := chatAreaRows(model)
	nMsg := len(model.Messages)
	if model.Scroll < 0 {
		model.Scroll = 0
	}
	if max := nMsg - maxRows; model.Scroll > max {
		model.Scroll = max
		if model.Scroll < 0 {
			model.Scroll = 0
		}
	}
	end = model.Scroll + maxRows
	if end > nMsg {
		end = nMsg
	}
	return model.Scroll, end
}

// messagesToBuf writes the visible slice of model.Messages into
// the chat area of screen. Each row is rune-aware-truncated to
// TermCols; NOTE: this will wrap once soft-wrap lands.
func messagesToBuf(model *Model, screen [][]byte) {
	start, end := clampScroll(model)
	for i := start; i < end; i++ {
		// NOTE: In the future it will wrap not be only cutted out
		endByte, _ := truncateToCols(model.Messages[i].Text, model.TermCols)
		screen[i-start] = []byte(model.Messages[i].Text[:endByte])
	}
}

// statusBarToBuf writes the status bar into its layout row.
func statusBarToBuf(model *Model, screen [][]byte) {
	screen[statusBarRow(model.TermRows)] = statusBarView(*model)
}

// inputLineToBuf writes the wrapped input area into its layout
// rows. Each wrapped row gets its own dedicated scratch backing
// (model.inputScratches[i]) so they don't alias each other or
// the shared blank. The slice of buf that belongs on row i is
// the half-open interval [i*cols, (i+1)*cols) clamped to buf
// length.
func inputLineToBuf(model *Model, screen [][]byte) {
	top := inputRow(model)
	cols := model.TermCols
	buf := model.Input.buf
	for i, scratch := range model.inputScratches {
		copy(scratch, model.blank)
		start := i * cols
		if start > len(buf) {
			start = len(buf)
		}
		end := start + cols
		if end > len(buf) {
			end = len(buf)
		}
		copy(scratch, buf[start:end])
		screen[top+i] = scratch
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