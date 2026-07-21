package tui

import (
	"strconv"

	"droid/term"
)

type Mode int

const (
	ModeIdle Mode = iota
	ModeLoading
	ModeStreaming
	ModeError
)

type Message struct {
	Role string // "user" | "assistant" | "system"
	Text string
}

type Model struct {
	TermRows  int
	TermCols  int
	Status    string
	Input     Input
	Messages  []Message
	Mode      Mode
	ModelName string
	Err       error
	// Scroll is the index of the first visible message in
	// Messages. Snap value is max(0, nMsg-chatAreaRows) so the
	// most recent messages stay on the bottom. PageUp/PageDown
	// shift it by chatAreaRows within that clamp.
	Scroll int

	// Persistent render buffers — reused across frames so a
	// keystroke does not allocate the screen. Reallocated only
	// when the terminal size changes (see NewView).
	screen        [][]byte // rows indexed 0..TermRows-1
	blank         []byte   // one reusable blank row, len == TermCols
	inputScratch  []byte   // reusable scratch for the input row
	dirtyRows     []bool   // row-level dirty mask, true = needs rewrite
}

func NewModel() Model {
	cols, rows := term.Size()
	m := Model{
		TermRows:  rows,
		TermCols:  cols,
		Status:    "Droid AI Agent",
		ModelName: "DeepSeek v4 (Pro)",
		Mode:      ModeIdle,
	}
	// DEBUG: seed a long list of messages so scroll/PageUp/
	// PageDown can be exercised without typing. Remove once
	// real agent output is wired in.
	m.Messages = make([]Message, 100)
	for i := range m.Messages {
		m.Messages[i] = Message{
			Role: "system",
			Text: "debug line " + strconv.Itoa(i),
		}
	}
	return m
}
