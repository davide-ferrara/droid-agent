// Package term is
package term

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// Init switches the terminal to raw mode and clears the screen.
// Returns a restore function to be called with defer in main().
func Init() (func(), error) {
	restoreFunc, err := enableRawMode()
	if err != nil {
		return nil, err
	}

	_, _ = os.Stdout.Write([]byte(Cls))
	_, _ = os.Stdout.Write([]byte(CurHome))

	return restoreFunc, nil
}

// enableRawMode modifies the terminal's termios struct via ioctl.
// termios is the POSIX data structure that stores terminal settings
// (input/output modes, control chars, line discipline flags).
//
// Flags we disable:
//
//	IGNBRK — ignore break condition
//	BRKINT — break condition sends SIGINT
//	PARMRK — mark parity errors in input
//	INLCR  — map NL (0x0A) to CR (0x0D)
//	IGNCR  — ignore CR
//	ICRNL  — map CR (0x0D) to NL (0x0A)
//	INPCK  — parity checking
//	ISTRIP — strip 8th bit from input bytes
//	IXON   — software flow control (Ctrl+S/XOFF)
//	OPOST  — output processing (e.g. \n → \r\n)
//	ECHO   — echo input back to screen
//	ECHONL — echo newline in canonical mode
//	ICANON — canonical line-buffered mode → char-by-char
//	ISIG   — Ctrl+C/Z/\ generate signals
//	IEXTEN — extended input processing (Ctrl+V literal)
//	CSIZE  — character size mask (we set CS8 explicitly)
//	PARENB — parity generation/detection
//
// Flags we set:
//
//	CS8        — 8-bit characters (pass full 0x00-0xFF range)
//	VMIN = 1   — read() returns as soon as 1 byte arrives
//	VTIME = 0  — no timeout on read()
func enableRawMode() (func(), error) {
	termios, err := unix.IoctlGetTermios(unix.Stdin, unix.TCGETS)
	if err != nil {
		err = fmt.Errorf("error getting terminal flags: %w", err)
		log.Println(err)
		return nil, err
	}

	original := *termios

	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0

	if err = unix.IoctlSetTermios(unix.Stdin, unix.TCSETS, termios); err != nil {
		err = fmt.Errorf("EnableRawMode: error setting terminal flags: %w", err)
		log.Println(err)
		return nil, err
	}

	enterAltScreen()

	return func() {
		exitAltScreen()
		if err := unix.IoctlSetTermios(unix.Stdin, unix.TCSETS, &original); err != nil {
			log.Printf("error setting terminal flags: %s\n", err)
			os.Exit(1)
		}
	}, nil
}

// Size returns (col, row)
func Size() (int, int) {
	ws, err := unix.IoctlGetWinsize(unix.Stdout, unix.TIOCGWINSZ)
	if err != nil {
		return 80, 24
	}
	return int(ws.Col), int(ws.Row)
}

func HideCursor() { _, _ = os.Stdout.Write([]byte("\033[?25l")) }
func ShowCursor() { _, _ = os.Stdout.Write([]byte("\033[?25h")) }
func ClearCurrentLine() { _, _ = os.Stdout.Write([]byte("\033[K")) }

// CUP — Cursor Position: ESC [ <row> ; <col> H  (1-indexed)
func MoveCursor(row, col int) {
	if row == 0 || col == 0 {
		return
	}
	var b [32]byte
	buf := b[:0]
	buf = append(buf, "\033["...)
	buf = strconv.AppendInt(buf, int64(row), 10)
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(col), 10)
	buf = append(buf, 'H')
	_, _ = os.Stdout.Write(buf)
}

// Write buffer to the terminal
func Write(buf []byte, row int, col int) {
	MoveCursor(row, col)
	_, _ = os.Stdout.Write(buf)
}

// CursorPos queries the terminal for current cursor position via DSR
// (Device Status Report, ESC[6n). Terminal responds with ESC[row;colR.
func CursorPos() (row, col int, err error) {
	_, _ = os.Stdout.Write([]byte("\033[6n"))

	in := os.Stdin
	var b [1]byte
	read := func() (byte, error) {
		_, e := in.Read(b[:])
		return b[0], e
	}

	c, err := read()
	if err != nil {
		return
	}
	if c != 0x1b {
		err = fmt.Errorf("CursorPos: expected ESC, got %x", c)
		return
	}

	c, err = read()
	if err != nil {
		return
	}
	if c != '[' {
		err = fmt.Errorf("CursorPos: expected '[', got %x", c)
		return
	}

	for {
		c, err = read()
		if err != nil {
			return
		}
		if c == ';' {
			break
		}
		row = row*10 + int(c-'0')
	}

	for {
		c, err = read()
		if err != nil {
			return
		}
		if c == 'R' {
			break
		}
		col = col*10 + int(c-'0')
	}

	return
}

func enterAltScreen() {
	_, _ = os.Stdout.Write([]byte(AltScreen))
}

func exitAltScreen() {
	_, _ = os.Stdout.Write([]byte(MainScreen))
}
