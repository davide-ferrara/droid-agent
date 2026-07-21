package tui

import "testing"

// newModelWithInput builds a Model with the given input buffer
// and terminal size, skipping the seed messages from NewModel.
// Used by layout and view tests to isolate just the input-area
// math.
func newModelWithInput(buf string, cols, rows int) Model {
	m := Model{
		TermRows: rows,
		TermCols: cols,
	}
	m.Input.buf = []rune(buf)
	m.Input.cursorX = len(m.Input.buf)
	m.Input.syncCursor(cols)
	return m
}

// TestInputHeight_FloorsAtOne verifies that empty or invalid
// (cols<=0) input reports height 1, not 0, so NewView never has
// to render a zero-row input area.
func TestInputHeight_FloorsAtOne(t *testing.T) {
	if h := inputHeight(&Model{}); h != 1 {
		t.Errorf("empty model: got %d want 1", h)
	}
	if h := inputHeight(&Model{TermCols: 80, Input: Input{buf: []rune("hi")}}); h != 1 {
		t.Errorf("2-char @ 80: got %d want 1", h)
	}
	if h := inputHeight(&Model{TermCols: 0, Input: Input{buf: []rune("hi")}}); h != 1 {
		t.Errorf("cols=0: got %d want 1", h)
	}
}

// TestInputHeight_GrowsOnASCIIWrap locks in the wrap math for
// plain ASCII buffers. Note the virtual-cursor rule: a buffer
// that exactly fills cols bumps the height by 1 so the cursor
// has a place to blink at the start of the next row — verified
// in a 24x80 pty where exactly 80 'a's renders a full row plus
// an empty cursor row below it.
func TestInputHeight_GrowsOnASCIIWrap(t *testing.T) {
	cols := 80
	checks := []struct {
		n    int
		want int
	}{
		{79, 1}, // cursor at col 79, no wrap
		{80, 2}, // full row + cursor wraps
		{81, 2}, // full row + 1 char on row 1
		{159, 2},
		{160, 3}, // two full rows + cursor wraps
		{161, 3}, // two full rows + 1 char on row 2
		{200, 3}, // two full rows + 40 chars on row 2
	}
	for _, c := range checks {
		m := newModelWithInput(string(repeatRune('a', c.n)), cols, 24)
		if h := inputHeight(&m); h != c.want {
			t.Errorf("n=%d cols=%d: got %d want %d", c.n, cols, h, c.want)
		}
	}
}

// TestInputHeight_WideRuneAccounting ensures a buffer of N wide
// runes (each width 2) takes the right number of rows modulo the
// virtual-cursor wrap.
func TestInputHeight_WideRuneAccounting(t *testing.T) {
	cols := 80
	// 39 wide runes (78 cols) → 1 row, cursor at col 78.
	m := newModelWithInput(string(repeatRune('你', 39)), cols, 24)
	if h := inputHeight(&m); h != 1 {
		t.Errorf("39 wide @ 80: got %d want 1", h)
	}
	// 40 wide runes (80 cols) → full row, cursor wraps, height 2.
	m = newModelWithInput(string(repeatRune('你', 40)), cols, 24)
	if h := inputHeight(&m); h != 2 {
		t.Errorf("40 wide @ 80: got %d want 2", h)
	}
	// 41 wide runes (82 cols) → 2 rows (row 0 full, row 1 has 1
	// wide rune at cols 0-1, cursor at col 2).
	m = newModelWithInput(string(repeatRune('你', 41)), cols, 24)
	if h := inputHeight(&m); h != 2 {
		t.Errorf("41 wide @ 80: got %d want 2", h)
	}
	// 1 emoji at cols=2 fills the row exactly, cursor wraps → 2.
	m = newModelWithInput("😀", 2, 24)
	if h := inputHeight(&m); h != 2 {
		t.Errorf("1 emoji @ 2: got %d want 2", h)
	}
}

// TestInputHeight_CombiningMarksNoAdvance checks that combining
// marks (runeWidth 0) don't advance the column, so they don't
// grow the input height beyond where the base rune left the
// cursor. With cols=4 and "cafe\u0300": cursor wraps after 'e'
// (col reaches 4); the following combining grave doesn't change
// the (row, col) so height stays at 2 (the wrapped cursor row).
func TestInputHeight_CombiningMarksNoAdvance(t *testing.T) {
	buf := "cafe\u0300"
	// cols=4: base fills the row, height 2 (cursor wraps to
	// next row after 'e'); combining grave doesn't bump it.
	m := newModelWithInput(buf, 4, 24)
	if h := inputHeight(&m); h != 2 {
		t.Errorf("cols=4: got %d want 2", h)
	}
	// cols=80: base stops at col 4, no wrap, height 1.
	m = newModelWithInput(buf, 80, 24)
	if h := inputHeight(&m); h != 1 {
		t.Errorf("cols=80: got %d want 1", h)
	}
}

// TestInputRow_PinsToBottomMinusHeight verifies that inputRow
// keeps the input area flush with the status bar regardless of
// the input height: a 1-row input sits at row 22 on a 24-row
// terminal, a 3-row input sits at row 20.
func TestInputRow_PinsToBottomMinusHeight(t *testing.T) {
	cols, rows := 80, 24
	checks := []struct {
		buf  string
		want int
	}{
		{"", rows - 2},                          // 1 input row
		{string(repeatRune('a', 79)), rows - 2},  // 1 input row (cursor col 79)
		{string(repeatRune('a', 80)), rows - 3},  // 2 input rows (cursor wrap)
		{string(repeatRune('a', 81)), rows - 3},  // 2 input rows
		{string(repeatRune('a', 200)), rows - 4}, // 3 input rows
	}
	for _, c := range checks {
		m := newModelWithInput(c.buf, cols, rows)
		if got := inputRow(&m); got != c.want {
			t.Errorf("buf=%d a's: got %d want %d", len(c.buf), got, c.want)
		}
	}
}

// TestChatAreaRows_ShrinksOnInputGrowth verifies the chat area
// yields rows back to the input area as the buffer wraps.
func TestChatAreaRows_ShrinksOnInputGrowth(t *testing.T) {
	cols, rows := 80, 24
	// Empty input → chat takes 22, status 1, input 1.
	m := newModelWithInput("", cols, rows)
	if got := chatAreaRows(&m); got != rows-2 {
		t.Errorf("empty: got %d want %d", got, rows-2)
	}
	// 79 a's → still 1 input row, chat still 22.
	m = newModelWithInput(string(repeatRune('a', 79)), cols, rows)
	if got := chatAreaRows(&m); got != rows-2 {
		t.Errorf("79 a's: got %d want %d", got, rows-2)
	}
	// 80 a's → 2 input rows (cursor wrap) → chat 21.
	m = newModelWithInput(string(repeatRune('a', 80)), cols, rows)
	if got := chatAreaRows(&m); got != rows-3 {
		t.Errorf("80 a's: got %d want %d", got, rows-3)
	}
	// 200 a's → 3 input rows → chat 20.
	m = newModelWithInput(string(repeatRune('a', 200)), cols, rows)
	if got := chatAreaRows(&m); got != rows-4 {
		t.Errorf("200 a's: got %d want %d", got, rows-4)
	}
}

// TestStatusBarRow_UnaffectedByInput asserts the status bar row
// is pinned to the last terminal row regardless of how tall the
// input area grows. This is the whole reason statusBarRow takes
// only termRows and not *Model.
func TestStatusBarRow_UnaffectedByInput(t *testing.T) {
	rows := 24
	want := rows - 1
	if got := statusBarRow(rows); got != want {
		t.Errorf("statusBarRow: got %d want %d", got, want)
	}
}