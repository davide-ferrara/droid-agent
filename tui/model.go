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
	Input     InputLine
	Messages  []Message
	Mode      Mode
	ModelName string
	Err       error
}

func NewModel() Model {
	cols, rows := term.Size()
	initLine := InputLine{}
	initLine.y = rows - 2
	return Model{
		TermRows:  rows,
		TermCols:  cols,
		Status:    "Droid AI Agent",
		Input:     initLine,
		ModelName: "DeepSeek v4 (Pro)",
		Mode:      ModeIdle,
	}
}
