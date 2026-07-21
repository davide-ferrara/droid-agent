package tui

import (
	"bufio"
	"log"

	"droid/term"
)

// trimSpaceRunes is the []rune equivalent of bytes.TrimSpace —
// drops leading and trailing Unicode whitespace. We use it
// instead of bytes.TrimSpace because Input.buf is now []rune.
func trimSpaceRunes(buf []rune) []rune {
	start, end := 0, len(buf)
	for start < end && isSpaceRune(buf[start]) {
		start++
	}
	for end > start && isSpaceRune(buf[end-1]) {
		end--
	}
	return buf[start:end]
}

// isSpaceRune reports whether r is whitespace per Unicode.
// Mirrors unicode.IsSpace without pulling the unicode package
// in for one commonly-used helper.
func isSpaceRune(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r',
		0x85, 0xA0,
		0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000:
		return true
	}
	return false
}

type Input struct {
	// buf holds the user's input as runes. Storing runes instead
	// of bytes keeps cursorX a rune index, so HandleBackspace /
	// Left / Right move a whole grapheme cluster at a time even
	// when bytes are split across multi-byte UTF-8 sequences.
	buf []rune
	// x is the screen column where the input area starts (0 = left edge).
	x int
	// cursorX is the rune index of the cursor within buf
	// (0 = before first rune, len(buf) = past the last).
	cursorX int
	// cursorY is the wrapped input row the cursor sits on,
	// derived from cursorX via wrapRow and cached here so
	// MoveCursorTo doesn't have to re-walk the buffer.
	cursorY int
	// cursorCol is the column within the wrapped row where the
	// cursor sits, derived from wrapRow alongside cursorY. It's
	// the display column (sum of rune widths), not a byte offset,
	// so wide runes that filled a partial row count correctly.
	cursorCol int
}

// cursorRow returns the wrapped input row the cursor currently
// occupies, derived from the buffer (independent of the cached
// cursorY field). Used by callers that need to know what would
// happen at the current cursor if the wrap math were re-run.
func (in *Input) cursorRow(cols int) int {
	row, _ := wrapRow(in.buf, in.cursorX, cols)
	return row
}

// syncCursor re-derives cursorY and cursorCol from cursorX using
// wrapRow. Call after any mutation that touches cursorX or buf
// so the cached on-screen (row, col) stays consistent.
func (in *Input) syncCursor(cols int) {
	in.cursorY, in.cursorCol = wrapRow(in.buf, in.cursorX, cols)
}

// MoveCursorTo repositions the terminal cursor on its wrapped
// input row using the cached cursorY / cursorCol. row is the
// top of the input area (inputRow); the actual terminal row is
// row + cursorY.
func (in *Input) MoveCursorTo(row int) {
	term.MoveCursor(row+in.cursorY+1, in.x+in.cursorCol+1)
}

// HandleBackspace removes one rune to the left of the cursor.
// Because buf is []rune, this always removes a whole UTF-8
// sequence even if it's 2-4 bytes long (e.g. one emoji press).
func (in *Input) HandleBackspace(cols int) {
	if in.cursorX == 0 {
		return
	}
	in.buf = append(in.buf[:in.cursorX-1], in.buf[in.cursorX:]...)
	in.cursorX--
	in.syncCursor(cols)
	dbgInput(in)
}

// HandleEnter clears the input buffer back to the empty state.
// Called after the message has been appended to Model.Messages.
func (in *Input) HandleEnter() {
	in.buf = in.buf[:0]
	in.cursorX = 0
	in.cursorY = 0
	in.cursorCol = 0
	dbgInput(in)
}

// HandleLine appends a printable rune to the input buffer.
// The wrap math is left to wrapRow (called via syncCursor), so
// wide runes (CJK, emoji) advance the right number of columns
// and combining marks (runeWidth 0) sit on the same cell as
// the previous rune without advancing the layout.
func (in *Input) HandleLine(r rune, cols int) {
	in.buf = append(in.buf, r)
	in.cursorX++
	in.syncCursor(cols)
	dbgInput(in)
}

func (in *Input) HandleLeft(cols int) {
	if in.cursorX > 0 {
		in.cursorX--
		in.syncCursor(cols)
	}
}

func (in *Input) HandleRight(cols int) {
	if in.cursorX < len(in.buf) {
		in.cursorX++
		in.syncCursor(cols)
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
	m.Input.MoveCursorTo(inputRow(m))
}

func (m *Model) pollResize() bool {
	cols, rows := term.Size()
	if cols != m.TermCols || rows != m.TermRows {
		m.TermCols, m.TermRows = cols, rows
		// Clamp cursor to the buffer end and re-sync its wrapped
		// row. SyncCursor derives cursorY/cursorCol from cursorX
		// via wrapRow, so a horizontal shrink just rewraps.
		if m.Input.cursorX > len(m.Input.buf) {
			m.Input.cursorX = len(m.Input.buf)
		}
		m.Input.syncCursor(cols)
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

// handleRune processes a printable rune from term.ReadKey. The
// wrap math is rune-aware: wide runes bump inputHeight only when
// they actually overflow a row, and combining marks leave the
// layout untouched.
func (m *Model) handleRune(r rune) {
	// Detect a wrap-height change so we know whether to repaint
	// just the active input row or also the chat rows (which
	// shift up when input grows).
	beforeRows := inputHeight(m)
	beforeCursorRow := m.Input.cursorRow(m.TermCols)
	beforeScroll := m.Scroll
	beforeMaxScroll := maxScroll(m)
	m.Input.HandleLine(r, m.TermCols)
	afterRows := inputHeight(m)
	afterCursorRow := m.Input.cursorRow(m.TermCols)
	if beforeRows != afterRows {
		// Input area grew by a row: chat area shrunk; repaint
		// from scratch to fix both regions.
		if beforeScroll == beforeMaxScroll {
			m.Scroll = maxScroll(m)
		}
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
	beforeScroll := m.Scroll
	beforeMaxScroll := maxScroll(m)
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
		if beforeScroll == beforeMaxScroll {
			m.Scroll = maxScroll(m)
		}
		m.markAllDirty()
	}
	m.renderFrame()
}

func (m *Model) handleEnter() {
	// Check for empty messages, or messages with only spaces.
	// Opencode allow you to sent messages with only spaces, not here.
	if len(trimSpaceRunes(m.Input.buf)) == 0 {
		return
	}
	m.Messages = append(m.Messages, Message{Role: "user", Text: string(m.Input.buf)})
	// Snap to latest: set Scroll past the end so the next
	// clampScroll run (inside messagesToBuf via renderFrame)
	// recomputes maxScroll by backward-fill and pins the
	// newest message at the bottom.
	m.Scroll = len(m.Messages)
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
// a wrap boundary — MoveCursorTo uses the cached cursorY /
// cursorCol recomputed by syncCursor inside HandleLeft/Right.
// Skip the move entirely when the cursor is already at an edge
// and didn't move.
func (m *Model) handleLeft() {
	before := m.Input.cursorX
	m.Input.HandleLeft(m.TermCols)
	if m.Input.cursorX != before {
		m.Input.MoveCursorTo(inputRow(m))
	}
}

func (m *Model) handleRight() {
	before := m.Input.cursorX
	m.Input.HandleRight(m.TermCols)
	if m.Input.cursorX != before {
		m.Input.MoveCursorTo(inputRow(m))
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

// pageUpShift / pageDownShift scroll the chat area by chatAreaRows
// counted in DISPLAY rows (not message indices). Scrolling by 1
// message index would jump the wrong amount when a wrapped
// message spans multiple rows, so we walk the message list
// forward (down) or backward (up) skipping whole messages until
// we've consumed chatAreaRows display rows. Clamp at [0, maxScroll]
// where maxScroll is computed by clampScroll's backward-fill.
func (m *Model) pageUpShift() {
	target := chatAreaRows(m)
	m.Scroll -= target
	if m.Scroll < 0 {
		m.Scroll = 0
	}
	m.markChatRowsDirty()
	m.renderFrame()
}

func (m *Model) pageDownShift() {
	ms := maxScroll(m)
	m.Scroll += chatAreaRows(m)
	if m.Scroll > ms {
		m.Scroll = ms
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
			model.handleRune(ev.Rune)
		case term.KindCtrl:
			model.handleCtrl(ev.Byte)
		case term.KindCSI:
			model.handleCSI(ev.Seq)
		}
	}
}