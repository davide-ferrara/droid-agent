// Package tui view for droid
package tui

import (
	"bytes"
	"strconv"

	"droid/term"
)

func inputLineView(model Model) []byte {
	return model.Input.buf
}

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

func NewView(model Model) [][]byte {
	statusBar := statusBarView(model)
	inputLine := inputLineView(model)

	screenBuf := make([][]byte, model.TermRows)
	for i := range len(screenBuf) {
		screenBuf[i] = bytes.Repeat([]byte("."), model.TermCols)
	}
	screenBuf[model.TermRows-1] = statusBar
	if n := model.TermCols - len(inputLine); n > 0 {
		inputLine = append(inputLine, bytes.Repeat([]byte(" "), n)...)
	}
	screenBuf[model.Input.y] = inputLine

	return screenBuf
}
