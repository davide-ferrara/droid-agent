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
	// Debug toggles dev-only status-bar info (cols x rows).
	// Off in normal use; flip on to inspect layouts.
	Debug bool
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
	// DEBUG: seed a mixed UTF-8 sample so we can exercise
	// scroll/PageUp/PageDown and rune-aware truncation without
	// typing. Covers Italian accents, precomposed + decomposed,
	// CJK, emoji, ZWJ sequences, and a regional-indicator flag.
	// Remove once real agent output is wired in.
	m.Messages = []Message{
		{Role: "user", Text: "caffè per favore"},
		{Role: "assistant", Text: "Subito! Ecco il tuo caffè ☕"},
		{Role: "user", Text: "perché non funziona?"},
		{Role: "assistant", Text: "è solo un test, non preoccuparti"},
		{Role: "user", Text: "come stai?"},
		{Role: "assistant", Text: "Tutto bene, grazie! 😊"},
		{Role: "user", Text: "你好世界"},
		{Role: "assistant", Text: "こんにちは世界 🌏"},
		{Role: "user", Text: "flag test 🇮🇹 end"},
		{Role: "assistant", Text: "ZWJ family 👨‍👩‍👧 done"},
		{Role: "user", Text: "decomposed caf\u0300 test"},
		{Role: "assistant", Text: "naïve façade résumé"},
		{Role: "user", Text: "mix 😀ab cd😀😀 ef"},
		{Role: "assistant", Text: " régua: ñ, ü, ç, à, é, è, ì, ò, ù"},
		{Role: "user", Text: "long line: " + string(repeatRune('a', 200))},
		{Role: "assistant", Text: "long emoji: " + string(repeatRune('😀', 60))},
		{Role: "user", Text: "padding 1"},
		{Role: "assistant", Text: "padding 2"},
		{Role: "user", Text: "padding 3"},
		{Role: "assistant", Text: "padding 4"},
		{Role: "user", Text: "padding 5"},
		{Role: "assistant", Text: "padding 6"},
		{Role: "user", Text: "padding 7"},
		{Role: "assistant", Text: "padding 8"},
		{Role: "user", Text: "padding 9"},
		{Role: "assistant", Text: "padding 10"},
		{Role: "user", Text: "padding 11"},
		{Role: "assistant", Text: "padding 12"},
		{Role: "user", Text: "padding 13"},
		{Role: "assistant", Text: "padding 14"},
		{Role: "user", Text: "padding 15"},
		{Role: "assistant", Text: "padding 16"},
		{Role: "user", Text: "padding 17"},
		{Role: "assistant", Text: "padding 18"},
		{Role: "user", Text: "padding 19"},
		{Role: "assistant", Text: "padding 20"},
		{Role: "user", Text: "padding 21"},
		{Role: "assistant", Text: "padding 22"},
		{Role: "user", Text: "padding 23"},
		{Role: "assistant", Text: "padding 24"},
		{Role: "user", Text: "padding 25"},
		{Role: "assistant", Text: "padding 26"},
		{Role: "user", Text: "padding 27"},
		{Role: "assistant", Text: "padding 28"},
		{Role: "user", Text: "padding 29"},
		{Role: "assistant", Text: "padding 30"},
		{Role: "user", Text: "padding 31"},
		{Role: "assistant", Text: "padding 32"},
		{Role: "user", Text: "padding 33"},
		{Role: "assistant", Text: "padding 34"},
		{Role: "user", Text: "padding 35"},
		{Role: "assistant", Text: "padding 36"},
		{Role: "user", Text: "padding 37"},
		{Role: "assistant", Text: "padding 38"},
		{Role: "user", Text: "padding 39"},
		{Role: "assistant", Text: "padding 40"},
		{Role: "user", Text: "padding 41"},
		{Role: "assistant", Text: "padding 42"},
		{Role: "user", Text: "padding 43"},
		{Role: "assistant", Text: "padding 44"},
		{Role: "user", Text: "padding 45"},
		{Role: "assistant", Text: "padding 46"},
		{Role: "user", Text: "padding 47"},
		{Role: "assistant", Text: "padding 48"},
		{Role: "user", Text: "padding 49"},
		{Role: "assistant", Text: "padding 50"},
		{Role: "user", Text: "padding 51"},
		{Role: "assistant", Text: "padding 52"},
		{Role: "user", Text: "padding 53"},
		{Role: "assistant", Text: "padding 54"},
		{Role: "user", Text: "padding 55"},
		{Role: "assistant", Text: "padding 56"},
		{Role: "user", Text: "padding 57"},
		{Role: "assistant", Text: "padding 58"},
		{Role: "user", Text: "padding 59"},
		{Role: "assistant", Text: "padding 60"},
		{Role: "user", Text: "padding 61"},
		{Role: "assistant", Text: "padding 62"},
		{Role: "user", Text: "padding 63"},
		{Role: "assistant", Text: "padding 64"},
		{Role: "user", Text: "padding 65"},
		{Role: "assistant", Text: "padding 66"},
		{Role: "user", Text: "padding 67"},
		{Role: "assistant", Text: "padding 68"},
		{Role: "user", Text: "padding 69"},
		{Role: "assistant", Text: "padding 70"},
		{Role: "user", Text: "padding 71"},
		{Role: "assistant", Text: "padding 72"},
		{Role: "user", Text: "padding 73"},
		{Role: "assistant", Text: "padding 74"},
		{Role: "user", Text: "padding 75"},
		{Role: "assistant", Text: "padding 76"},
		{Role: "user", Text: "padding 77"},
		{Role: "assistant", Text: "padding 78"},
		{Role: "user", Text: "padding 79"},
		{Role: "assistant", Text: "padding 80"},
		{Role: "user", Text: "padding 81"},
		{Role: "assistant", Text: "padding 82"},
		{Role: "user", Text: "padding 83"},
		{Role: "assistant", Text: "padding 84"},
		{Role: "user", Text: "padding 85"},
		{Role: "assistant", Text: "padding 86"},
		{Role: "user", Text: "padding 87"},
		{Role: "assistant", Text: "padding 88"},
	}
	return m
}

func repeatRune(r rune, n int) []rune {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return out
}
