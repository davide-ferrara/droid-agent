// Package tui view for droid
package tui

import (
	"bytes"
	"strconv"

	"droid/term"
)

func inputView(model Model) []byte {
	return model.Input.buf
}

// NOTE: if the screen is too zommed in the status bar
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
	buf = append(buf, "  │  "...)
	buf = strconv.AppendInt(buf, int64(model.TermCols), 10)
	buf = append(buf, 'x')
	buf = strconv.AppendInt(buf, int64(model.TermRows), 10)
	buf = append(buf, ' ')
	buf = append(buf, term.ClearLine...)
	buf = append(buf, "\033[0m"...)
	return buf
}

func fillLine(n int, fill byte) []byte {
	return bytes.Repeat([]byte{fill}, n)
}

func NewView(model *Model) [][]byte {
	// Layout needs >=2 rows (input + status) and >=1 col.
	// Below that, indexing screenBuf[-1] would panic, so return
	// a single-line notice instead of a full screen.
	if model.TermRows < 2 || model.TermCols < 1 {
		return [][]byte{[]byte("terminal too small (need >=2 rows, >=1 col)")}
	}

	// Reuse model.screen / model.blank / model.inputScratch across
	// frames. Reallocate only when the terminal has resized (or
	// first call). This keeps a keystroke allocation-free in the
	// common case. The dirty mask starts fully set on every
	// (re)allocation so the first paint covers the whole screen.
	if cap(model.screen) < model.TermRows || cap(model.blank) < model.TermCols {
		model.screen = make([][]byte, model.TermRows)
		model.blank = make([]byte, model.TermCols)
		model.inputScratch = make([]byte, model.TermCols)
		model.dirtyRows = make([]bool, model.TermRows)
		for i := range model.dirtyRows {
			model.dirtyRows[i] = true
		}
	} else {
		model.screen = model.screen[:model.TermRows]
		model.blank = model.blank[:model.TermCols]
		model.inputScratch = model.inputScratch[:model.TermCols]
		model.dirtyRows = model.dirtyRows[:model.TermRows]
	}

	// Refill the blank with spaces — cheap, no new alloc. We
	// share the same backing array across every empty row, safe
	// because we never mutate blank rows in place.
	for i := range model.blank {
		model.blank[i] = ' '
	}

	screenBuf := model.screen
	for i := range screenBuf {
		screenBuf[i] = model.blank
	}

	// NOTE: With UFT-8 len(text) is not enough since char are of 4
	// bytes, must use runewidht
	maxRows := chatAreaRows(model.TermRows)
	nMsg := len(model.Messages)
	// Clamp scroll to its valid range so a resize, deletion,
	// or new message never leaves a blank viewport.
	if model.Scroll < 0 {
		model.Scroll = 0
	}
	if max := nMsg - maxRows; model.Scroll > max {
		model.Scroll = max
		if model.Scroll < 0 {
			model.Scroll = 0
		}
	}
	end := model.Scroll + maxRows
	if end > nMsg {
		end = nMsg
	}
	for i := model.Scroll; i < end; i++ {
		// NOTE: In the future it will wrap not be only cutted out
		clamp := min(model.TermCols, len(model.Messages[i].Text))
		screenBuf[i-model.Scroll] = []byte(model.Messages[i].Text[:clamp])
	}

	screenBuf[statusBarRow(model.TermRows)] = statusBarView(*model)

	// Input row: reuse inputScratch explicitly so we don't alias
	// the shared blank (the input line is the one row that gets
	// mutated in place each frame). Refill from blank, then copy
	// the current input buffer on top.
	scratch := model.inputScratch
	copy(scratch, model.blank)
	copy(scratch, inputView(*model))
	screenBuf[inputRow(model.TermRows)] = scratch

	return screenBuf
}
