package tui

import (
	"bufio"
	"log"

	"droid/term"
)

type Input struct {
	buf     []byte
	x       int
	cursorX int
	cursorY int
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
	m.Input.HandleEnter()
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

func handleCSI(sequence string) {
	switch sequence {
	case term.Up:
		log.Println("Key Up")
	case term.Down:
		log.Println("Key Down")
	case term.Right:
		log.Println("Key Right")
	case term.Left:
		log.Println("Key Left")
	default:
		log.Println("Seq INOP")
	}
}

func HandleInput(reader *bufio.Reader, model *Model) {
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
			handleCSI(ev.Seq)
		}
	}
}
