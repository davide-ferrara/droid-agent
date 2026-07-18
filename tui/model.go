package tui

import "droid/term"

type Mode int

const (
	ModeIdle Mode = iota
	ModeLoading
	ModeStreaming
	ModeError
)

type InputLine struct {
	buf []byte
	cx  int
}

type Message struct {
	Role string // "user" | "assistant" | "system"
	Text string
}

type Model struct {
	TermRows  int
	TermCols  int
	Status    string
	Input     InputLine
	Messages  []Message
	Mode      Mode
	ModelName string
	Err       error
}

func InitModel() Model {
	cols, rows := term.Size()
	return Model{
		TermRows:  rows,
		TermCols:  cols,
		Status:    "Droid AI Agent",
		ModelName: "DeepSeek v4 (Pro)",
		Mode:      ModeIdle,
	}
}
