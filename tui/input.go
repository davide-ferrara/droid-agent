package tui

import (
	"bufio"
	"bytes"
	"log"

	"droid/term"
)

type Input struct {
	buf     []byte // user input text, raw bytes
	x       int    // screen column where the input line starts (0 = left edge)
	cursorX int    // horizontal offset of the cursor within buf (0 = before first char)
	cursorY int    // vertical offset for future multi-line input (0 = input line; reserved, always 0 today — wire when soft-wrap lands)
}

func (in *Input) MoveCursorTo(row int) {
	term.MoveCursor(row+in.cursorY+1, in.x+in.cursorX+1)
}

func (in *Input) HandleBackspace() {
	if in.cursorX == 0 {
		return
	}
	in.buf = append(in.buf[:in.cursorX-1], in.buf[in.cursorX:]...)
	in.cursorX--
	dbgInput(in)
}

func (in *Input) HandleEnter() {
	in.buf = in.buf[:0]
	in.cursorX = 0
	dbgInput(in)
}

func (in *Input) HandleLine(ch byte, cols int) {
	// NOTE: in the future it will wrap, for now we just stop
	// accepting input once the cursor reaches the right edge.
	if in.cursorX >= cols {
		return
	}
	in.buf = append(in.buf, ch)
	in.cursorX++
	dbgInput(in)
}

func (in *Input) HandleLeft() {
	if in.cursorX > 0 {
		in.cursorX--
	}
}

func (in *Input) HandleRight() {
	if in.cursorX < len(in.buf) {
		in.cursorX++
	}
}

// markAllDirty marks every row for full repaint. Used on resize.
func (m *Model) markAllDirty() {
	for i := range m.dirtyRows {
		m.dirtyRows[i] = true
	}
}

// markRowDirty flags a single row for rewrite on the next Render.
// Bounds-checked so callers don't need to guard themselves.
func (m *Model) markRowDirty(row int) {
	if row < 0 || row >= len(m.dirtyRows) {
		return
	}
	m.dirtyRows[row] = true
}

func (m *Model) renderInput() {
	Render(NewView(m), m.dirtyRows, statusBarRow(m.TermRows))
	m.Input.MoveCursorTo(inputRow(m.TermRows))
}

func (m *Model) pollResize() bool {
	cols, rows := term.Size()
	if cols != m.TermCols || rows != m.TermRows {
		m.TermCols, m.TermRows = cols, rows
		// Clamp cursor to the new content boundary. Its valid
		// range is 0..len(buf); on a horizontal shrink it must
		// also stay inside the visible columns.
		if m.Input.cursorX > len(m.Input.buf) {
			m.Input.cursorX = len(m.Input.buf)
		}
		if cols > 0 && m.Input.cursorX >= cols {
			m.Input.cursorX = cols - 1
		}
		// NewView will realloc the persistent buffers on size
		// change; mark every row dirty so the first post-resize
		// paint covers the whole screen.
		m.markAllDirty()
		r, c, _ := term.CursorPos()
		dbgModel(m, r, c)
		return true
	}
	return false
}

func (m *Model) handleInput(ch byte) {
	m.Input.HandleLine(ch, m.TermCols)
	m.markRowDirty(inputRow(m.TermRows))
	m.renderInput()
}

func (m *Model) handleBackspace() {
	// HandleBackspace is a no-op when cursorX == 0; skip the
	// repaint in that case (the row did not actually change).
	before := len(m.Input.buf)
	m.Input.HandleBackspace()
	if len(m.Input.buf) == before {
		return
	}
	m.markRowDirty(inputRow(m.TermRows))
	m.renderInput()
}

func (m *Model) handleEnter() {
	// Check for empty messages, or messages with only spaces.
	// Opencode allow you to sent messages with only spaces, not here.
	if len(bytes.TrimSpace(m.Input.buf)) == 0 {
		return
	}
	m.Messages = append(m.Messages, Message{Role: "user", Text: string(m.Input.buf)})
	// Snap to latest so the new message is immediately visible.
	m.Scroll = len(m.Messages) - chatAreaRows(m.TermRows)
	if m.Scroll < 0 {
		m.Scroll = 0
	}
	// NewView shifts messages up via the start offset when the
	// list overflows the chat area, so the whole column may have
	// changed — mark every chat row dirty.
	for i := 0; i < chatAreaRows(m.TermRows); i++ {
		m.markRowDirty(i)
	}
	m.markRowDirty(inputRow(m.TermRows))
	m.Input.HandleEnter()
	m.renderInput()
}

func (m *Model) handleLeft() {
	// Cursor-only movement: the input row's content is unchanged,
	// so there is nothing to repaint. Just reposition the cursor.
	before := m.Input.cursorX
	m.Input.HandleLeft()
	if m.Input.cursorX != before {
		m.Input.MoveCursorTo(inputRow(m.TermRows))
	}
}

func (m *Model) handleRight() {
	before := m.Input.cursorX
	m.Input.HandleRight()
	if m.Input.cursorX != before {
		m.Input.MoveCursorTo(inputRow(m.TermRows))
	}
}

func (m *Model) handleCtrl(key byte) {
	switch key {
	case term.CtrlH, term.Backspace:
		m.handleBackspace()
	case term.Enter, term.CtrlJ:
		m.handleEnter()
	default:
		log.Printf("Unhandled: %x", key)
	}
}

// pageUpShift / pageDownShift scroll the chat area by one
// viewport, clamped to [0, max(0, nMsg-chatAreaRows)].
func (m *Model) pageUpShift() {
	maxRows := chatAreaRows(m.TermRows)
	m.Scroll -= maxRows
	if m.Scroll < 0 {
		m.Scroll = 0
	}
	for i := 0; i < maxRows; i++ {
		m.markRowDirty(i)
	}
	m.renderInput()
}

func (m *Model) pageDownShift() {
	maxRows := chatAreaRows(m.TermRows)
	max := len(m.Messages) - maxRows
	if max < 0 {
		max = 0
	}
	m.Scroll += maxRows
	if m.Scroll > max {
		m.Scroll = max
	}
	for i := 0; i < maxRows; i++ {
		m.markRowDirty(i)
	}
	m.renderInput()
}

func (m *Model) handleCSI(seq string) {
	switch seq {
	case term.Right:
		m.handleRight()
	case term.Left:
		m.handleLeft()
	case term.PageUp:
		m.pageUpShift()
	case term.PageDown:
		m.pageDownShift()
	default:
		log.Printf("CSI unhandled: %s", seq)
	}
}

func HandleInput(reader *bufio.Reader, model *Model) {
	model.renderInput()
	for {
		ev := term.ReadKey(reader)

		if model.pollResize() {
			model.renderInput()
			continue
		}

		switch ev.Kind {
		case term.KindEOF:
			return
		case term.KindQuit:
			log.Println("CtrlC")
			return
		case term.KindPrintable:
			model.handleInput(ev.Byte)
		case term.KindCtrl:
			model.handleCtrl(ev.Byte)
		case term.KindCSI:
			model.handleCSI(ev.Seq)
		}
	}
}
