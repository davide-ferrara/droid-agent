package tui

import "droid/term"

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

	// Persistent render buffers — reused across frames so a
	// keystroke does not allocate the screen. Reallocated only
	// when the terminal size changes (see NewView).
	screen        [][]byte // rows indexed 0..TermRows-1
	blank         []byte   // one reusable blank row, len == TermCols
	inputScratch  []byte   // reusable scratch for the input row
}

func NewModel() Model {
	cols, rows := term.Size()
	return Model{
		TermRows:  rows,
		TermCols:  cols,
		Status:    "Droid AI Agent",
		ModelName: "DeepSeek v4 (Pro)",
		Mode:      ModeIdle,
	}
}
