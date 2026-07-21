package tui

import (
	"bufio"
	"bytes"
	"log"

	"droid/term"
)

type Input struct {
	buf     []byte // user input text, raw bytes
	x       int    // screen column where the input area starts (0 = left edge)
	cursorX int    // byte offset of the cursor within buf (0 = before first char)
	cursorY int    // wrapped row the cursor sits on, 0-indexed (kept in sync with cursorX via syncCursorY)
}

// cursorRow derives which wrapped input row the cursor occupies,
// given the current column count. 0 = first row. Math:
//   - cursorX==0: start of row 0
//   - cursorX>0: cursor sits just after the char at index
//     cursorX-1; that char lives on row (cursorX-1)/cols, so
//     we return the same.
//
// Mirrors the cursorY field; kept as a derivation helper for
// syncCursorY. ASCII-only for now — rune-aware math replaces
// this when HandleLine accepts runes.
func (in *Input) cursorRow(cols int) int {
	if in.cursorX == 0 || cols <= 0 {
		return 0
	}
	return (in.cursorX - 1) / cols
}

// syncCursorY recomputes cursorY from cursorX. Call after any
// mutation that touches cursorX so the on-screen row stays
// consistent with Left/Right/Backspace/Enter movements.
func (in *Input) syncCursorY(cols int) {
	in.cursorY = in.cursorRow(cols)
}

// MoveCursorTo repositions the terminal cursor on its wrapped
// input row. row is the TOP of the input area (inputRow); we
// add cursorY to land on the active row and compute the column
// as cursorX minus the rows already filled.
func (in *Input) MoveCursorTo(row, cols int) {
	col := in.cursorX - in.cursorY*cols
	if col < 0 {
		col = 0
	}
	term.MoveCursor(row+in.cursorY+1, in.x+col+1)
}

func (in *Input) HandleBackspace(cols int) {
	if in.cursorX == 0 {
		return
	}
	in.buf = append(in.buf[:in.cursorX-1], in.buf[in.cursorX:]...)
	in.cursorX--
	in.syncCursorY(cols)
	dbgInput(in)
}

func (in *Input) HandleEnter() {
	in.buf = in.buf[:0]
	in.cursorX = 0
	in.cursorY = 0
	dbgInput(in)
}

// HandleLine appends a printable byte to the input buffer.
// When the cursor sits at the right edge, the wrapped row count
// (cursorY) bumps by one so the next row below becomes a fresh
// empty writing line; the already-typed full-width line stays
// rendered one row above. cursorY is then re-synced from
// cursorX (the explicit bump is preserved for readability —
// syncCursorY lands on the same value).
func (in *Input) HandleLine(ch byte, cols int) {
	if cols > 0 && in.cursorX > 0 && in.cursorX%cols == 0 {
		in.cursorY++
	}
	in.buf = append(in.buf, ch)
	in.cursorX++
	in.syncCursorY(cols)
	dbgInput(in)
}

func (in *Input) HandleLeft(cols int) {
	if in.cursorX > 0 {
		in.cursorX--
		in.syncCursorY(cols)
	}
}

func (in *Input) HandleRight(cols int) {
	if in.cursorX < len(in.buf) {
		in.cursorX++
		in.syncCursorY(cols)
	}
}

// markAllDirty marks every row for full repaint. Used on resize.
func (m *Model) markAllDirty() {
	for i := range m.dirtyRows {
		m.dirtyRows[i] = true
	}
}

// markRowDirty flags a single row for rewrite on the next Render.
// Bounds-checked so callers don't need to guard themselves.
func (m *Model) markRowDirty(row int) {
	if row < 0 || row >= len(m.dirtyRows) {
		return
	}
	m.dirtyRows[row] = true
}

// markInputRowsDirty marks every input-area row dirty. Used when
// the input area's content or height changes.
func (m *Model) markInputRowsDirty() {
	top := inputRow(m)
	for i := 0; i < inputHeight(m); i++ {
		m.markRowDirty(top + i)
	}
}

// markChatRowsDirty marks every chat-area row dirty. Used when
// the input height changes (chat area grows/shrinks) or when
// messages shift on append/scroll.
func (m *Model) markChatRowsDirty() {
	for i := 0; i < chatAreaRows(m); i++ {
		m.markRowDirty(i)
	}
}

func (m *Model) renderFrame() {
	Render(NewView(m), m.dirtyRows, statusBarRow(m.TermRows))
	m.Input.MoveCursorTo(inputRow(m), m.TermCols)
}

func (m *Model) pollResize() bool {
	cols, rows := term.Size()
	if cols != m.TermCols || rows != m.TermRows {
		m.TermCols, m.TermRows = cols, rows
		// Clamp cursor to the buffer end and re-sync its wrapped
		// row. Input byte offsets don't need a cols-based clamp
		// anymore — syncCursorY derives the row from cursorX
		// and cols, so a horizontal shrink just rewraps.
		if m.Input.cursorX > len(m.Input.buf) {
			m.Input.cursorX = len(m.Input.buf)
		}
		m.Input.syncCursorY(cols)
		// NewView will realloc the persistent buffers on size
		// change; mark every row dirty so the first post-resize
		// paint covers the whole screen.
		m.markAllDirty()
		r, c, _ := term.CursorPos()
		dbgModel(m, r, c)
		return true
	}
	return false
}

func (m *Model) handleChar(ch byte) {
	// Detect a wrap-height change so we know whether to repaint
	// just the active input row or also the chat rows (which
	// shift up when input grows).
	beforeRows := inputHeight(m)
	beforeCursorRow := m.Input.cursorRow(m.TermCols)
	m.Input.HandleLine(ch, m.TermCols)
	afterRows := inputHeight(m)
	afterCursorRow := m.Input.cursorRow(m.TermCols)
	if beforeRows != afterRows {
		// Input area grew by a row: chat area shrunk; repaint
		// from scratch to fix both regions.
		m.markAllDirty()
	} else if beforeCursorRow != afterCursorRow {
		// Cursor moved to a different input row without height
		// change (e.g. Right across a wrap boundary): repaint
		// the input area since the row the cursor blinks in
		// changed.
		m.markInputRowsDirty()
	} else {
		m.markRowDirty(inputRow(m) + afterCursorRow)
	}
	m.renderFrame()
}

func (m *Model) handleBackspace() {
	// HandleBackspace is a no-op when cursorX == 0; skip the
	// repaint in that case (the row did not actually change).
	beforeBuf := len(m.Input.buf)
	beforeRows := inputHeight(m)
	m.Input.HandleBackspace(m.TermCols)
	if len(m.Input.buf) == beforeBuf {
		return
	}
	afterRows := inputHeight(m)
	if beforeRows == afterRows {
		// CursorRow may still change (backspace at a wrap
		// boundary) but the active row is the one the cursor
		// just retreated onto.
		m.markRowDirty(inputRow(m) + m.Input.cursorRow(m.TermCols))
	} else {
		// Input area shrunk: repaint chat (grew) and input.
		m.markAllDirty()
	}
	m.renderFrame()
}

func (m *Model) handleEnter() {
	// Check for empty messages, or messages with only spaces.
	// Opencode allow you to sent messages with only spaces, not here.
	if len(bytes.TrimSpace(m.Input.buf)) == 0 {
		return
	}
	m.Messages = append(m.Messages, Message{Role: "user", Text: string(m.Input.buf)})
	// Snap to latest so the new message is immediately visible.
	m.Scroll = len(m.Messages) - chatAreaRows(m)
	if m.Scroll < 0 {
		m.Scroll = 0
	}
	// NewView shifts messages up via the start offset when the
	// list overflows the chat area, so the whole column may have
	// changed — mark every chat row dirty.
	m.markChatRowsDirty()
	// Resetting the input releases its wrapped rows; the chat
	// area grows back, so repaint from scratch.
	m.Input.HandleEnter()
	m.markAllDirty()
	m.renderFrame()
}

// handleLeft / handleRight are cursor-only moves. The buffer
// content is unchanged so there is nothing to repaint regardless
// of whether the cursor stays on the same wrapped row or crosses
// a wrap boundary — MoveCursorTo recomputes the (row, col) on
// its own. Skip the move entirely when the cursor is already at
// an edge and didn't move.
func (m *Model) handleLeft() {
	before := m.Input.cursorX
	m.Input.HandleLeft(m.TermCols)
	if m.Input.cursorX != before {
		m.Input.MoveCursorTo(inputRow(m), m.TermCols)
	}
}

func (m *Model) handleRight() {
	before := m.Input.cursorX
	m.Input.HandleRight(m.TermCols)
	if m.Input.cursorX != before {
		m.Input.MoveCursorTo(inputRow(m), m.TermCols)
	}
}

func (m *Model) handleCtrl(key byte) {
	switch key {
	case term.CtrlH, term.Backspace:
		m.handleBackspace()
	case term.Enter, term.CtrlJ:
		m.handleEnter()
	default:
		log.Printf("Unhandled: %x", key)
	}
}

// pageUpShift / pageDownShift scroll the chat area by one
// viewport, clamped to [0, max(0, nMsg-chatAreaRows)].
func (m *Model) pageUpShift() {
	maxRows := chatAreaRows(m)
	m.Scroll -= maxRows
	if m.Scroll < 0 {
		m.Scroll = 0
	}
	m.markChatRowsDirty()
	m.renderFrame()
}

func (m *Model) pageDownShift() {
	maxRows := chatAreaRows(m)
	max := len(m.Messages) - maxRows
	if max < 0 {
		max = 0
	}
	m.Scroll += maxRows
	if m.Scroll > max {
		m.Scroll = max
	}
	m.markChatRowsDirty()
	m.renderFrame()
}

func (m *Model) handleCSI(seq string) {
	switch seq {
	case term.Right:
		m.handleRight()
	case term.Left:
		m.handleLeft()
	case term.PageUp:
		m.pageUpShift()
	case term.PageDown:
		m.pageDownShift()
	default:
		log.Printf("CSI unhandled: %s", seq)
	}
}

func HandleKeyPress(reader *bufio.Reader, model *Model) {
	model.renderFrame()
	for {
		ev := term.ReadKey(reader)

		if model.pollResize() {
			model.renderFrame()
			continue
		}

		switch ev.Kind {
		case term.KindEOF:
			return
		case term.KindQuit:
			log.Println("CtrlC")
			return
		case term.KindPrintable:
			model.handleChar(ev.Byte)
		case term.KindCtrl:
			model.handleCtrl(ev.Byte)
		case term.KindCSI:
			model.handleCSI(ev.Seq)
		}
	}
}