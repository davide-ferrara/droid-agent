// Package tui for droid
package tui

import (
	"bufio"
	"log"
	"os"

	"droid/term"
)

func handleCtrl(m *Model, b byte) {
	switch b {
	case term.CtrlH, term.Backspace:
		m.Input.HandleBackspace()
	case term.Enter, term.CtrlJ:
		log.Println("Enter")
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
		log.Printf("Unhandled: %x", b)
	}
}

func handleCSI(seq string) {
	switch seq {
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

// NOTE: temporary — poll-based resize check.
// Replaces proper SIGWINCH handling via goroutines + channels.
// Will be refactored once we learn concurrency/signals.
func pollResize(m *Model) bool {
	cols, rows := term.Size()
	if cols != m.TermCols || rows != m.TermRows {
		m.TermCols, m.TermRows = cols, rows
		return true
	}
	return false
}

func HandleInput(r *bufio.Reader, m *Model) {
	for {
		ev := term.ReadKey(r)

		// NOTE: temporary, poll resize after every keypress.
		// Later: subResize goroutine sends MsgResize on SIGWINCH.
		if pollResize(m) {
			Render(InitView(*m))
			continue
		}

		switch ev.Kind {
		case term.KindEOF:
			return
		case term.KindQuit:
			log.Println("CtrlC")
			return
		case term.KindPrintable:
			if ev.Byte == 'u' {
				// NOTE: temporary — 'u' forces a full re-render to test resize.
				log.Println("re-render (resize test)")
				Render(InitView(*m))
				continue
			}
			m.Input.HandleLine(ev.Byte)
		case term.KindCtrl:
			handleCtrl(m, ev.Byte)
		case term.KindCSI:
			handleCSI(ev.Seq)
		}
	}
}

func Run() {
	r := bufio.NewReader(os.Stdin)
	m := InitModel()
	buf := InitView(m)
	Render(buf)
	HandleInput(r, &m)
}
