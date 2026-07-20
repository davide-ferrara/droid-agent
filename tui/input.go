package tui

import (
	"bufio"
	"log"

	"droid/term"
)

func dbgLine(line *InputLine) {
	log.Printf("line: %s\n", line.buf)
}

func (line *InputLine) Update() {
	term.MoveCursor(line.y+1, line.cursorX+1)
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
}

func (line *InputLine) HandleLine(ch byte) {
	if line.cursorX > len(line.buf) {
		line.cursorX = len(line.buf)
	}
	if line.cursorX != len(line.buf) {
		head := make([]byte, line.cursorX)
		copy(head, line.buf[:line.cursorX])
		tail := line.buf[line.cursorX:]
		line.buf = append(append(head, ch), tail...)
		line.cursorX++
		return
	}
	line.buf = append(line.buf, ch)
	line.cursorX++
	line.Update()
	dbgLine(line)
}

func handleCtrl(model *Model, key byte) {
	switch key {
	case term.CtrlH, term.Backspace:
		model.Input.HandleBackspace()
		model.refresh()
	case term.Enter, term.CtrlJ:
		model.Input.HandleEnter()
		model.refresh()
	case term.CtrlD:
		log.Println("CtrlD")
	case term.CtrlL:
		log.Println("CtrlL")
	case term.CtrlU:
		log.Println("CtrlU")
	case term.CtrlK:
		log.Println("CtrlK")
	case term.CtrlW:
		log.Println("CtrlW")
	case term.CtrlA:
		log.Println("CtrlA")
	case term.CtrlE:
		log.Println("CtrlE")
	case term.CtrlB:
		log.Println("CtrlB")
	case term.CtrlF:
		log.Println("CtrlF")
	case term.CtrlN:
		log.Println("CtrlN")
	case term.CtrlP:
		log.Println("CtrlP")
	case term.CtrlR:
		log.Println("CtrlR")
	case term.CtrlZ:
		log.Println("CtrlZ")
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
	case term.Home:
		log.Println("Key Home")
	case term.End:
		log.Println("Key End")
	case term.F1:
		log.Println("Key F1")
	case term.F2:
		log.Println("Key F2")
	case term.F3:
		log.Println("Key F3")
	case term.F4:
		log.Println("Key F4")
	case term.Ins:
		log.Println("Key Ins")
	case term.Canc:
		log.Println("Key Canc")
	case term.PageUp:
		log.Println("Key PageUp")
	case term.PageDown:
		log.Println("Key PageDown")
	case term.F5:
		log.Println("Key F5")
	case term.F6:
		log.Println("Key F6")
	case term.F7:
		log.Println("Key F7")
	case term.F8:
		log.Println("Key F8")
	case term.F9:
		log.Println("Key F9")
	case term.F10:
		log.Println("Key F10")
	case term.F11:
		log.Println("Key F11")
	case term.F12:
		log.Println("Key F12")
	default:
		log.Println("Seq INOP")
	}
}

func pollResize(model *Model) bool {
	cols, rows := term.Size()
	if cols != model.TermCols || rows != model.TermRows {
		model.TermCols, model.TermRows = cols, rows
		return true
	}
	return false
}

func HandleInput(reader *bufio.Reader, model *Model) {
	term.MoveCursor(model.Input.y+1, model.Input.cursorX+1)
	for {
		ev := term.ReadKey(reader)

		if pollResize(model) {
			model.Input.y = model.TermRows - 2
			model.refresh()
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
			model.refresh()
		case term.KindCtrl:
			handleCtrl(model, ev.Byte)
		case term.KindCSI:
			handleCSI(ev.Seq)
		}
	}
}
