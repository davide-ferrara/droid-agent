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
	screenBuf := make([][]byte, model.TermRows)
	for i := range screenBuf {
		screenBuf[i] = fillLine(model.TermCols, '.')
	}

	screenBuf[statusBarRow(model.TermRows)] = statusBarView(*model)

	row := inputRow(model.TermRows)
	line := fillLine(model.TermCols, ' ')
	copy(line, inputView(*model))
	screenBuf[row] = line

	return screenBuf
}
