package tui

import (
	"bytes"
	"testing"
)

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

// TestHandleRune_AnchoredScrollFollowsInputGrowth verifies that
// when the input area grows (typing past the right edge wraps to a
// new row) and Model.Scroll was anchored at maxScroll, Scroll is
// re-anchored to the new maxScroll so the latest message stays
// visible. Without this fix, the latest messages slide off the top
// as the chat area shrinks and maxScroll increases.
func TestHandleRune_AnchoredScrollFollowsInputGrowth(t *testing.T) {
	cols := 80
	m := Model{TermRows: 24, TermCols: cols}
	for i := 0; i < 25; i++ {
		m.Messages = append(m.Messages, Message{Text: string(repeatRune('a', cols))})
	}
	// +1 separator per message (except latest) → last 1 row,
	// others 2 rows.
	// Empty input: inputHeight=1, chatAreaRows=20, maxScroll=15
	// (last 1 + 9×2 = 19 ≤ 20, next +2=22 > 20).
	m.Scroll = maxScroll(&m)
	if m.Scroll != 15 {
		t.Fatalf("setup: Scroll=%d want 15", m.Scroll)
	}

	// Type 80 a's: inputHeight grows from 1 to 2. chatAreaRows
	// shrinks from 20 to 19, maxScroll stays 15
	// (last 1 + 9×2 = 19).
	for i := 0; i < 80; i++ {
		m.handleRune('a')
	}
	if m.Scroll != 15 {
		t.Errorf("after 80 a's (1->2 rows): Scroll=%d want 15", m.Scroll)
	}

	// Type 79 more a's (total 159): inputHeight stays 2 so Scroll
	// should remain at 15 (no re-anchor needed).
	for i := 0; i < 79; i++ {
		m.handleRune('a')
	}
	if m.Scroll != 15 {
		t.Errorf("after 159 a's (same 2 rows): Scroll=%d want 15", m.Scroll)
	}

	// Type the 160th 'a': triggers inputHeight 2→3 (2 full rows,
	// cursor at virtual row 2 col 0). chatAreaRows=18, maxScroll
	// goes to 16 (last 1 + 8×2 = 17 ≤ 18, next +2=20 > 18).
	m.handleRune('a')
	if m.Scroll != 16 {
		t.Errorf("after 160 a's (2->3 rows): Scroll=%d want 16", m.Scroll)
	}
}

// TestBackspace_AnchoredScrollPreserved verifies that backspace
// shrinking the input preserves anchoring when Scroll was at
// maxScroll.
func TestBackspace_AnchoredScrollPreserved(t *testing.T) {
	cols := 80
	m := Model{TermRows: 24, TermCols: cols}
	for i := 0; i < 25; i++ {
		m.Messages = append(m.Messages, Message{Text: string(repeatRune('a', cols))})
	}
	// +1 separator per message (except latest).
	// Type 81 a's: inputHeight=2, chatAreaRows=19, maxScroll=15.
	for i := 0; i < 81; i++ {
		m.Input.HandleLine('a', cols)
	}
	m.Scroll = maxScroll(&m)
	if m.Scroll != 15 {
		t.Fatalf("setup: Scroll=%d want 15", m.Scroll)
	}

	// Backspace 1 → 80 a's: inputHeight still 2 (same wrapped
	// rows). Scroll stays 15.
	m.handleBackspace()
	if m.Scroll != 15 {
		t.Errorf("after 1 BS (still 2 rows): Scroll=%d want 15", m.Scroll)
	}

	// Backspace 1 more → 79 a's: inputHeight drops to 1
	// (cursor retreats to row 0). Scroll clamps to new maxScroll=15.
	m.handleBackspace()
	if m.Scroll != 15 {
		t.Errorf("after 2 BS (2->1 rows): Scroll=%d want 15", m.Scroll)
	}
}

// TestHandleRune_ScrollUnchangedWhenNotAnchored ensures that typing
// with input growth does NOT change Scroll when the user was
// scrolled up (not anchored at maxScroll).
func TestHandleRune_ScrollUnchangedWhenNotAnchored(t *testing.T) {
	cols := 80
	m := Model{TermRows: 24, TermCols: cols}
	for i := 0; i < 25; i++ {
		m.Messages = append(m.Messages, Message{Text: string(repeatRune('a', cols))})
	}
	// maxScroll is much higher, but set Scroll to 0 (scrolled way up).
	m.Scroll = 0

	// Type 80 a's: inputHeight grows from 1 to 2. Scroll stays 0.
	for i := 0; i < 80; i++ {
		m.handleRune('a')
	}
	if m.Scroll != 0 {
		t.Errorf("not anchored: Scroll=%d want 0 (unchanged)", m.Scroll)
	}
}

// TestWrapRow_WordWrap verifies that wrapRow breaks at the last
// space before overflow instead of hard-wrapping mid-word.
func TestWrapRow_WordWrap(t *testing.T) {
	cols := 10

	// "hello bigworld": space at idx 5, "bigworld" overflows.
	buf := []rune("hello bigworld")
	row, col := wrapRow(buf, len(buf), cols)
	if row != 1 || col != 8 {
		t.Errorf("hello bigworld @10: got (%d,%d) want (1,8)", row, col)
	}

	// "hello   bigworld": three spaces, last at idx 7. Break after
	// the last space. Row 0: "hello  " (7 cols). Row 1: "bigworld".
	buf = []rune("hello   bigworld")
	row, col = wrapRow(buf, len(buf), cols)
	if row != 1 || col != 8 {
		t.Errorf("hello+3spaces+bigworld @10: got (%d,%d) want (1,8)", row, col)
	}

	// "hello big": no overflow, single row.
	buf = []rune("hello big")
	row, col = wrapRow(buf, len(buf), cols)
	if row != 0 || col != 9 {
		t.Errorf("hello big @10: got (%d,%d) want (0,9)", row, col)
	}

	// "nospace": no spaces at all, hard-wrap at col boundary.
	buf = []rune("nospace")
	row, col = wrapRow(buf, len(buf), 4)
	if row != 1 || col != 3 {
		t.Errorf("nospace @4: got (%d,%d) want (1,3)", row, col)
	}

	// "abcdef gh": space at idx 6, overflow at idx 7 ('g').
	// Word-wrap: break after space. Row 1: "gh" at cols 0-1.
	buf = []rune("abcdef gh")
	row, col = wrapRow(buf, len(buf), 7)
	if row != 1 || col != 2 {
		t.Errorf("abcdef gh @7: got (%d,%d) want (1,2)", row, col)
	}

	// Space at exact overflow point: "abcdef g" @7.
	// 'g' overflows (col=6+1=7, cols=7, but col+w=7+1=8>7).
	// Wait, let me recalculate. 7 chars: a-f=6, space=7th char.
	// Actually "abcdef g" = 8 runes. Let me just check the behavior.
	buf = []rune("abcdefg h")
	// "abcdefg" = 7 cols, overflow at ' ' (col=7, col+1=8>7).
	// Space consumed as break.
	row, col = wrapRow(buf, len(buf), 7)
	// Row 0: "abcdefg" (7 cols). Row 1: "h" (1 col).
	// Wait, let me trace: no space before the overflow.
	// Actually "abcdefg" has no spaces. The first space is at idx 7.
	// col after "abcdefg": col=7. Space: col+1=8>7 → consumed.
	// Then 'h' at row=1, col=1.
	if row != 1 || col != 1 {
		t.Errorf("abcdefg h @7: got (%d,%d) want (1,1)", row, col)
	}

	// Cursor at a position before the word-wrap break should
	// still be on row 0 (the break happens at a later rune).
	buf = []rune("hello bigworld")
	row, col = wrapRow(buf, 6, cols) // idx 6 = 'b' (first char after space)
	if row != 0 || col != 6 {
		t.Errorf("cursor at 'b' @10: got (%d,%d) want (0,6)", row, col)
	}
	row, col = wrapRow(buf, 5, cols) // idx 5 = ' ' (the space itself)
	if row != 0 || col != 5 {
		t.Errorf("cursor at space @10: got (%d,%d) want (0,5)", row, col)
	}
}

// TestInputLineToBuf_WordWrap verifies that inputLineToBuf breaks
// at word boundaries, consuming the space at the break point.
func TestInputLineToBuf_WordWrap(t *testing.T) {
	cols, rows := 10, 24

	// "hello bigworld" should split as "hello" / "bigworld".
	buf := "hello bigworld"
	m := newModelWithInput(buf, cols, rows)
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	reallocInputScratches(&m)

	inputLineToBuf(&m, m.screen)

	top := inputRow(&m)
	wantRow0 := styledRow([]byte("hello"))
	wantRow1 := styledRow([]byte("bigworld"))
	if got := m.screen[top]; !bytes.Equal(got, wantRow0) {
		t.Errorf("row 0: got %q want %q", got, wantRow0)
	}
	if got := m.screen[top+1]; !bytes.Equal(got, wantRow1) {
		t.Errorf("row 1: got %q want %q", got, wantRow1)
	}
}

// TestInputLineToBuf_WordWrapConsumedSpace verifies that a space
// at the overflow point is consumed (not rendered on either row).
func TestInputLineToBuf_WordWrapConsumedSpace(t *testing.T) {
	cols, rows := 7, 24

	// "abcdefg h": after "abcdefg" (7 cols), space at overflow
	// point should be consumed. "h" starts on row 1.
	buf := "abcdefg h"
	m := newModelWithInput(buf, cols, rows)
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	reallocInputScratches(&m)

	inputLineToBuf(&m, m.screen)

	top := inputRow(&m)
	wantRow0 := styledRow([]byte("abcdefg"))
	wantRow1 := styledRow([]byte("h"))
	if got := m.screen[top]; !bytes.Equal(got, wantRow0) {
		t.Errorf("row 0: got %q want %q", got, wantRow0)
	}
	if got := m.screen[top+1]; !bytes.Equal(got, wantRow1) {
		t.Errorf("row 1: got %q want %q", got, wantRow1)
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