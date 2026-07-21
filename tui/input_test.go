package tui

import "testing"

// TestHandleLine_AppendsAndSetsCursor exercises HandleLine with
// ASCII, multi-byte Latin, CJK, and a 4-byte emoji in one go:
// the rune stays whole in the buffer and cursorX advances by
// 1 (rune index, not byte count).
func TestHandleLine_AppendsAndSetsCursor(t *testing.T) {
	cols := 80
	in := Input{}
	in.HandleLine('a', cols)
	in.HandleLine('b', cols)
	in.HandleLine('è', cols)
	in.HandleLine('你', cols)
	in.HandleLine('😀', cols)

	wantBuf := []rune{'a', 'b', 'è', '你', '😀'}
	if len(in.buf) != len(wantBuf) {
		t.Fatalf("buf len: got %d want %d", len(in.buf), len(wantBuf))
	}
	for i, r := range wantBuf {
		if in.buf[i] != r {
			t.Errorf("buf[%d]: got %q want %q", i, in.buf[i], r)
		}
	}
	if in.cursorX != 5 {
		t.Errorf("cursorX: got %d want 5", in.cursorX)
	}
}

// TestHandleLine_WrapAdvancesCursorRow verifies that typing past
// the right edge advances cursorY rather than hard-capping. At
// cols=80, after 80 'a's the cursor wraps to row 1, col 0.
func TestHandleLine_WrapAdvancesCursorRow(t *testing.T) {
	cols := 80
	in := Input{}
	for i := 0; i < cols; i++ {
		in.HandleLine('a', cols)
	}
	if in.cursorX != cols {
		t.Errorf("cursorX: got %d want %d", in.cursorX, cols)
	}
	if in.cursorY != 1 {
		t.Errorf("cursorY after full row: got %d want 1", in.cursorY)
	}
	if in.cursorCol != 0 {
		t.Errorf("cursorCol after full row: got %d want 0", in.cursorCol)
	}
	// One more char lands on row 1 col 1.
	in.HandleLine('a', cols)
	if in.cursorY != 1 || in.cursorCol != 1 {
		t.Errorf("after wrap+1: got (%d,%d) want (1,1)", in.cursorY, in.cursorCol)
	}
}

// TestHandleLine_WideRuneOverflowCreatesGap exercises the pty
// scenario "79 a's + emoji at cols=80": the emoji needs 2 cols
// but only 1 is left, so it wraps to row 1 col 0, leaving col 79
// of row 0 as a blank gap.
func TestHandleLine_WideRuneOverflowCreatesGap(t *testing.T) {
	cols := 80
	in := Input{}
	for i := 0; i < 79; i++ {
		in.HandleLine('a', cols)
	}
	if in.cursorY != 0 || in.cursorCol != 79 {
		t.Fatalf("after 79 a's: got (%d,%d) want (0,79)", in.cursorY, in.cursorCol)
	}
	in.HandleLine('😀', cols)
	// Cursor is now past the emoji on row 1 cols 0-1, so at col 2.
	if in.cursorY != 1 || in.cursorCol != 2 {
		t.Errorf("after wide overflow: got (%d,%d) want (1,2)", in.cursorY, in.cursorCol)
	}
	if len(in.buf) != 80 {
		t.Errorf("buf len: got %d want 80", len(in.buf))
	}
}

// TestHandleLine_CombiningMarkNoAdvance verifies that a 0-width
// combining mark attaches to the previous rune without moving
// the cursor: after "e" + U+0300 GRAVE the cursor stays at col 1.
func TestHandleLine_CombiningMarkNoAdvance(t *testing.T) {
	cols := 80
	in := Input{}
	in.HandleLine('e', cols)
	if in.cursorCol != 1 {
		t.Fatalf("after e: cursorCol=%d want 1", in.cursorCol)
	}
	in.HandleLine('\u0300', cols)
	if in.cursorCol != 1 {
		t.Errorf("after combining: cursorCol=%d want 1 (no advance)", in.cursorCol)
	}
	if in.cursorX != 2 {
		t.Errorf("cursorX: got %d want 2 (rune index moved)", in.cursorX)
	}
}

// TestHandleBackspace_RemovesWholeRune verifies that Backspace
// on a buffer ending in a 4-byte emoji removes the whole rune
// in one press (pty scenario "abc😀 + Backspace + XY → abcXY").
func TestHandleBackspace_RemovesWholeRune(t *testing.T) {
	cols := 80
	in := Input{}
	for _, r := range "abc😀" {
		in.HandleLine(r, cols)
	}
	if len(in.buf) != 4 {
		t.Fatalf("setup: buf len=%d want 4", len(in.buf))
	}
	in.HandleBackspace(cols)
	if len(in.buf) != 3 {
		t.Errorf("after 1 BS: buf len=%d want 3 (whole emoji gone)", len(in.buf))
	}
	want := []rune{'a', 'b', 'c'}
	for i, r := range want {
		if in.buf[i] != r {
			t.Errorf("buf[%d]: got %q want %q", i, in.buf[i], r)
		}
	}
	if in.cursorX != 3 {
		t.Errorf("cursorX: got %d want 3", in.cursorX)
	}
}

// TestHandleBackspace_RetractsWrap exercises the pty scenario
// "85 a's + 5 Backspaces": cursor retreats from row 1 back to
// row 0 as the buffer length drops below the cols boundary.
func TestHandleBackspace_RetractsWrap(t *testing.T) {
	cols := 80
	in := Input{}
	for i := 0; i < 85; i++ {
		in.HandleLine('a', cols)
	}
	if in.cursorY != 1 {
		t.Fatalf("setup: cursorY=%d want 1", in.cursorY)
	}
	// Backspace 1: cursor at col 4 on row 1, still row 1.
	in.HandleBackspace(cols)
	if in.cursorY != 1 || in.cursorCol != 4 {
		t.Errorf("after 1 BS: got (%d,%d) want (1,4)", in.cursorY, in.cursorCol)
	}
	// Backspace to col 0 on row 1 (3 more BS unless the cursor
	// affects row 0). After BS removes the last 4 chars buf is
	// 80 long, which exactly fills row 0; cursor visually wraps
	// but stays (row=1, col=0) per the virtual-cursor rule.
	for i := 0; i < 4; i++ {
		in.HandleBackspace(cols)
	}
	if len(in.buf) != 80 {
		t.Errorf("after 5 BS: buf len=%d want 80", len(in.buf))
	}
	// Cursor at end of exactly-full row 0 lands at row 1 col 0
	// per wrapRow's virtual-cursor rule.
	if in.cursorY != 1 || in.cursorCol != 0 {
		t.Errorf("at exactly full row: got (%d,%d) want (1,0)", in.cursorY, in.cursorCol)
	}
	// One more Backspace leaves 79 a's: cursor at row 0 col 79.
	in.HandleBackspace(cols)
	if in.cursorY != 0 || in.cursorCol != 79 {
		t.Errorf("after 6 BS: got (%d,%d) want (0,79)", in.cursorY, in.cursorCol)
	}
}

// TestHandleBackspace_NoOpAtZero verifies Backspace at the start
// of an empty buffer does nothing and the caller can cheaply
// skip the repaint (this is how handleBackspace avoids a no-op
// render frame).
func TestHandleBackspace_NoOpAtZero(t *testing.T) {
	cols := 80
	in := Input{}
	in.HandleBackspace(cols)
	if len(in.buf) != 0 || in.cursorX != 0 {
		t.Errorf("empty BS mutated state: buf=%v cursorX=%d", in.buf, in.cursorX)
	}
}

// TestHandleLeft_HandleRight_CrossesWrapBoundary covers the pty
// scenario "200 a's + 5 Left + 2 Right": the cursor crosses the
// wrap boundary between rows without ever repainting the screen
// (cursor-only moves).
func TestHandleLeft_HandleRight_CrossesWrapBoundary(t *testing.T) {
	cols := 80
	in := Input{}
	for i := 0; i < 200; i++ {
		in.HandleLine('a', cols)
	}
	// Cursor: row 2, col 40 (200 - 160 = 40 on row 2).
	if in.cursorY != 2 || in.cursorCol != 40 {
		t.Fatalf("setup: got (%d,%d) want (2,40)", in.cursorY, in.cursorCol)
	}
	// Left 5: cursor at row 2 col 35.
	for i := 0; i < 5; i++ {
		in.HandleLeft(cols)
	}
	if in.cursorY != 2 || in.cursorCol != 35 {
		t.Errorf("after 5 Left: got (%d,%d) want (2,35)", in.cursorY, in.cursorCol)
	}
	// Right 2: back at row 2 col 37.
	for i := 0; i < 2; i++ {
		in.HandleRight(cols)
	}
	if in.cursorY != 2 || in.cursorCol != 37 {
		t.Errorf("after 5 L + 2 R: got (%d,%d) want (2,37)", in.cursorY, in.cursorCol)
	}
}

// TestHandleLeft_WrapsUpToPreviousRow verifies Left at col 0 of
// a wrapped row jumps to the last col of the previous row, not
// to col -1.
func TestHandleLeft_WrapsUpToPreviousRow(t *testing.T) {
	cols := 80
	in := Input{}
	for i := 0; i < 81; i++ {
		in.HandleLine('a', cols)
	}
	// Cursor: row 1 col 1.
	if in.cursorY != 1 || in.cursorCol != 1 {
		t.Fatalf("setup: got (%d,%d) want (1,1)", in.cursorY, in.cursorCol)
	}
	// Left 1: cursor at row 1 col 0.
	in.HandleLeft(cols)
	if in.cursorY != 1 || in.cursorCol != 0 {
		t.Errorf("after L at row1 col1: got (%d,%d) want (1,0)", in.cursorY, in.cursorCol)
	}
	// Left 1 again: wrap up to row 0 col 79.
	in.HandleLeft(cols)
	if in.cursorY != 0 || in.cursorCol != 79 {
		t.Errorf("after L wrap up: got (%d,%d) want (0,79)", in.cursorY, in.cursorCol)
	}
}

// TestHandleRight_NoOpAtEnd verifies Right at end-of-buf is a
// no-op like HandleBackspace at start-of-buf.
func TestHandleRight_NoOpAtEnd(t *testing.T) {
	cols := 80
	in := Input{}
	for _, r := range "hi" {
		in.HandleLine(r, cols)
	}
	in.HandleRight(cols)
	if in.cursorX != 2 {
		t.Errorf("Right at end moved cursor: cursorX=%d want 2", in.cursorX)
	}
}

// TestHandleEnter_ResetsState verifies that HandleEnter after a
// wrapped multi-row buffer returns the Input to a blank slate so
// the next keystroke starts on row 0.
func TestHandleEnter_ResetsState(t *testing.T) {
	cols := 80
	in := Input{}
	for i := 0; i < 200; i++ {
		in.HandleLine('a', cols)
	}
	in.HandleEnter()
	if len(in.buf) != 0 {
		t.Errorf("buf: got %d want 0", len(in.buf))
	}
	if in.cursorX != 0 || in.cursorY != 0 || in.cursorCol != 0 {
		t.Errorf("after Enter: cursor=(%d,%d,%d) want all 0",
			in.cursorX, in.cursorY, in.cursorCol)
	}
}

// TestCursorRow_DerivesFromCursorX verifies the read-only helper
// matches the cached cursorY after a series of mutations.
func TestCursorRow_DerivesFromCursorX(t *testing.T) {
	cols := 80
	in := Input{}
	for i := 0; i < 165; i++ {
		in.HandleLine('a', cols)
	}
	if got := in.cursorRow(cols); got != in.cursorY {
		t.Errorf("cursorRow()=%d cached cursorY=%d (mismatch)", got, in.cursorY)
	}
	in.HandleLeft(cols)
	if got := in.cursorRow(cols); got != in.cursorY {
		t.Errorf("after Left: cursorRow()=%d cached=%d", got, in.cursorY)
	}
}