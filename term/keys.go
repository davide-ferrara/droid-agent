package term

// C0 control codes — ASCII range 0x00-0x1F.
// In raw mode these bytes arrive directly from the terminal,
// no longer interpreted by the kernel's line discipline.
// The mnemonic names (CtrlA, CtrlB...) come from the convention
// that the byte sent when pressing Ctrl+letter is the letter's
// ASCII code masked to 5 bits: ctrl = letter & 0x1F.
// E.g. 'A' (0x41) & 0x1F = 0x01 = CtrlA.
const (
	CtrlA     byte = 0x01
	CtrlB     byte = 0x02
	CtrlC     byte = 0x03
	CtrlD     byte = 0x04
	CtrlE     byte = 0x05
	CtrlF     byte = 0x06
	CtrlH     byte = 0x08
	Tab       byte = 0x09
	CtrlJ     byte = 0x0A
	CtrlK     byte = 0x0B
	CtrlL     byte = 0x0C
	Enter     byte = 0x0D
	CtrlN     byte = 0x0E
	CtrlP     byte = 0x10
	CtrlR     byte = 0x12
	CtrlT     byte = 0x14
	CtrlU     byte = 0x15
	CtrlW     byte = 0x17
	CtrlZ     byte = 0x1A
	Esc       byte = 0x1B
	Backspace byte = 0x7F
)

// Output strings are ANSI escape sequences sent to stdout.
// \033 is the ESC character (0x1B), [ introduces a CSI sequence.
const (
	Cls          string = "\033[2J" // [2J = clear entire screen
	CurHome      string = "\033[H"  // [H   = move cursor to home (1,1)
	ClearLine    string = "\033[K"
	RetClearLine string = "\r\033[K"
	AltScreen    string = "\033[?1049h"
	MainScreen   string = "\033[?1049l"
)

// CSI (Control Sequence Introducer) sequences for cursor keys.
// After ESC+[ the terminal sends one more byte A-D / H / F.
const (
	Up    string = "[A"
	Down  string = "[B"
	Right string = "[C"
	Left  string = "[D"
	Home  string = "[H"
	End   string = "[F"
)

// SS3 (Single Shift 3) sequences used by F1-F4.
// Pattern: ESC O P/Q/R/S — no [ prefix.
const (
	F1 string = "OP"
	F2 string = "OQ"
	F3 string = "OR"
	F4 string = "OS"
)

// 4-byte CSI sequences: ESC [ digit ~
const (
	Ins      string = "[2~"
	Canc     string = "[3~"
	PageUp   string = "[5~"
	PageDown string = "[6~"
)

// 5+ byte CSI sequences: ESC [ digits ~
const (
	F5  string = "[15~"
	F6  string = "[17~"
	F7  string = "[18~"
	F8  string = "[19~"
	F9  string = "[20~"
	F10 string = "[21~"
	F11 string = "[23~"
	F12 string = "[24~"
)
