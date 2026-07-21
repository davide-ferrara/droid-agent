package tui

import (
	"bufio"
	"log"

	"droid/term"
)

// NOTE: Dont remove the unused for now
type InputLine struct {
	buf     []byte
	y       int
	x       int
	cursorX int
	cursorY int
}

func (line *InputLine) Update() {
	term.MoveCursor(line.y+line.cursorY+1, line.x+line.cursorX+1)
}

func pollResize(model *Model) bool {
	cols, rows := term.Size()
	if cols != model.TermCols || rows != model.TermRows {
		model.TermCols, model.TermRows = cols, rows
		model.Input.y = rows - 2
		if model.Input.cursorX >= cols && cols > 0 {
			model.Input.cursorX = cols - 1
		}
		r, c, _ := term.CursorPos()
		dbgModel(model, r, c)
		return true
	}
	return false
}

func (line *InputLine) HandleBackspace() {
	if line.cursorX == 0 {
		return
	}
	line.buf = append(line.buf[:line.cursorX-1], line.buf[line.cursorX:]...)
	line.cursorX--
	line.Update()
	dbgLine(line)
}

func (line *InputLine) HandleEnter() {
	line.buf = line.buf[:0]
	line.cursorX = 0
	dbgLine(line)
}

func (line *InputLine) HandleLine(ch byte) {
	line.buf = append(line.buf, ch)
	line.cursorX++
	line.Update()
	dbgLine(line)
}

func handleCtrl(model *Model, key byte) {
	switch key {
	case term.CtrlH, term.Backspace:
		model.Input.HandleBackspace()
		Render(NewView(model))
		model.Input.Update()
	case term.Enter, term.CtrlJ:
		model.Input.HandleEnter()
		Render(NewView(model))
		model.Input.Update()
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
	model.Input.Update()
	for {
		ev := term.ReadKey(reader)

		if pollResize(model) {
			Render(NewView(model))
			model.Input.Update()
			continue
		}

		switch ev.Kind {
		case term.KindEOF:
			return
		case term.KindQuit:
			log.Println("CtrlC")
			return
		case term.KindPrintable:
			model.Input.HandleLine(ev.Byte)
			Render(NewView(model))
			model.Input.Update()
		case term.KindCtrl:
			handleCtrl(model, ev.Byte)
		case term.KindCSI:
			handleCSI(ev.Seq)
		}
	}
}
