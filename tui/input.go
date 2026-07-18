package tui

import (
	"log"
)

func DbgLine(line *InputLine) {
	log.Printf("line: %s\n", line.buf)
}

func (line *InputLine) HandleBackspace() {
	if line.cx == 0 {
		return
	}
	line.buf = append(line.buf[:line.cx-1], line.buf[line.cx:]...)
	line.cx--
	DbgLine(line)
}

func (line *InputLine) HandleLine(b byte) {
	if line.cx > len(line.buf) {
		line.cx = len(line.buf)
	}
	if line.cx != len(line.buf) {
		head := make([]byte, line.cx)
		copy(head, line.buf[:line.cx])
		tail := line.buf[line.cx:]
		line.buf = append(append(head, b), tail...)
		line.cx++
		return
	}
	line.buf = append(line.buf, b)
	line.cx++
	DbgLine(line)
}
