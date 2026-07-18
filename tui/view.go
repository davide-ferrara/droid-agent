// Package tui view for droid
package tui

import (
	"bytes"
	"strconv"

	"droid/term"
)

func statusBarView(m Model) []byte {
	var b [256]byte
	buf := b[:0]

	buf = append(buf, "\033[38;2;0;0;0m\033[48;2;0;180;244m"...)

	switch m.Mode {
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
	buf = append(buf, m.ModelName...)
	buf = append(buf, " │ "...)
	buf = append(buf, m.Status...)
	buf = append(buf, "  │  "...)
	buf = strconv.AppendInt(buf, int64(m.TermCols), 10)
	buf = append(buf, 'x')
	buf = strconv.AppendInt(buf, int64(m.TermRows), 10)
	buf = append(buf, ' ')
	buf = append(buf, term.ClearLine...)
	buf = append(buf, "\033[0m"...)
	return buf
}

func InitView(m Model) [][]byte {
	s := statusBarView(m)

	screenBuf := make([][]byte, m.TermRows)
	for i := range len(screenBuf) {
		screenBuf[i] = bytes.Repeat([]byte("."), m.TermCols)
	}
	screenBuf[m.TermRows-1] = s
	return screenBuf
}
