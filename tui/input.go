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

// MoveCursorTo positions the terminal cursor at the input line.
// row is 0-indexed; +1 converts to 1-indexed for the terminal.
func (in *Input) MoveCursorTo(row int) {
	term.MoveCursor(row+in.cursorY+1, in.x+in.cursorX+1)
}

func pollResize(model *Model) bool {
	cols, rows := term.Size()
	if cols != model.TermCols || rows != model.TermRows {
		model.TermCols, model.TermRows = cols, rows
		if model.Input.cursorX >= cols && cols > 0 {
			model.Input.cursorX = cols - 1
		}
		r, c, _ := term.CursorPos()
		dbgModel(model, r, c)
		return true
	}
	return false
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

func renderInput(m *Model) {
	Render(NewView(m))
	m.Input.MoveCursorTo(inputRow(m.TermRows))
}

func (m *Model) handleInput(ch byte) {
	m.Input.HandleLine(ch)
	renderInput(m)
}

func (m *Model) handleBackspace() {
	m.Input.HandleBackspace()
	renderInput(m)
}

func (m *Model) handleEnter() {
	m.Input.HandleEnter()
	renderInput(m)
}

func handleCtrl(model *Model, key byte) {
	switch key {
	case term.CtrlH, term.Backspace:
		model.handleBackspace()
	case term.Enter, term.CtrlJ:
		model.handleEnter()
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

		if pollResize(model) {
			renderInput(model)
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
			handleCtrl(model, ev.Byte)
		case term.KindCSI:
			handleCSI(ev.Seq)
		}
	}
}
