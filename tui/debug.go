package tui

import "log"

func dbgInput(in *Input) {
	log.Printf("input: %s\n", in.buf)
}

func dbgModel(m *Model, cursorRow, cursorCol int) {
	log.Printf("--- Model ---")
	log.Printf("TermRows: %d, TermCols: %d", m.TermRows, m.TermCols)
	log.Printf("Mode: %d", m.Mode)
	log.Printf("Status: %s", m.Status)
	log.Printf("ModelName: %s", m.ModelName)
	log.Printf("Input: x=%d cursorX=%d cursorY=%d buf=%q", m.Input.x, m.Input.cursorX, m.Input.cursorY, m.Input.buf)
	log.Printf("Messages: %d", len(m.Messages))
	log.Printf("Term cursor: %d,%d", cursorRow, cursorCol)
	if m.Err != nil {
		log.Printf("Err: %s", m.Err)
	}
}
