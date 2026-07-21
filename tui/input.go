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
	cursorY int    // vertical offset of the cursor within the input area (0 = input line, ≥1 for multi-line)
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

func (in *Input) HandleLine(ch byte) {
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

func (m *Model) renderInput() {
	Render(NewView(m))
	m.Input.MoveCursorTo(inputRow(m.TermRows))
}

func (m *Model) pollResize() bool {
	cols, rows := term.Size()
	if cols != m.TermCols || rows != m.TermRows {
		m.TermCols, m.TermRows = cols, rows
		if m.Input.cursorX >= cols && cols > 0 {
			m.Input.cursorX = cols - 1
		}
		r, c, _ := term.CursorPos()
		dbgModel(m, r, c)
		return true
	}
	return false
}

func (m *Model) handleInput(ch byte) {
	m.Input.HandleLine(ch)
	m.renderInput()
}

func (m *Model) handleBackspace() {
	m.Input.HandleBackspace()
	m.renderInput()
}

func (m *Model) handleEnter() {
	// Check for empty messages, or messages with only spaces.
	// Opencode allow you to sent messages with only spaces, not here.
	if len(bytes.TrimSpace(m.Input.buf)) == 0 {
		return
	}
	m.Messages = append(m.Messages, Message{Role: "user", Text: string(m.Input.buf)})
	m.Input.HandleEnter()
	m.renderInput()
}

func (m *Model) handleLeft() {
	m.Input.HandleLeft()
	m.renderInput()
}

func (m *Model) handleRight() {
	m.Input.HandleRight()
	m.renderInput()
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

func (m *Model) handleCSI(seq string) {
	switch seq {
	case term.Right:
		m.handleRight()
	case term.Left:
		m.handleLeft()
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
