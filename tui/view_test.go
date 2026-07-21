package tui

import (
	"bytes"
	"testing"
)

// TestInputLineToBuf_ASCIIWraps encodes the pty scenario "200
// a's rendered across 3 input rows": row 0 and row 1 are full
// of 80 a's, row 2 has 40.
func TestInputLineToBuf_ASCIIWraps(t *testing.T) {
	cols, rows := 80, 24
	buf := string(repeatRune('a', 200))
	m := newModelWithInput(buf, cols, rows)
	// Wire the persistent buffers the way NewView would, so
	// inputLineToBuf has a screen to write into.
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	reallocInputScratches(&m)

	inputLineToBuf(&m, m.screen)

	top := inputRow(&m)
	wantRow0 := bytes.Repeat([]byte{'a'}, 80)
	wantRow1 := bytes.Repeat([]byte{'a'}, 80)
	wantRow2 := bytes.Repeat([]byte{'a'}, 40)

	if got := m.screen[top]; !bytes.Equal(got, wantRow0) {
		t.Errorf("row 0: got %d bytes want %d", len(got), len(wantRow0))
	}
	if got := m.screen[top+1]; !bytes.Equal(got, wantRow1) {
		t.Errorf("row 1: got %d bytes want %d", len(got), len(wantRow1))
	}
	if got := m.screen[top+2]; !bytes.Equal(got, wantRow2) {
		t.Errorf("row 2: got %d bytes want %d", len(got), len(wantRow2))
	}
}

// TestInputLineToBuf_WideRuneAtRowEnd encodes the pty scenario
// "79 a's + emoji at cols=80": the emoji needs 2 cols but only
// 1 is left, so it wraps to row 1 cols 0-1, leaving col 79 of
// row 0 as a blank gap. The scratch for row 0 must contain 79
// a's followed by a single space (the pad that fills the gap
// the wide rune left behind).
func TestInputLineToBuf_WideRuneAtRowEnd(t *testing.T) {
	cols, rows := 80, 24
	in := Input{}
	for i := 0; i < 79; i++ {
		in.HandleLine('a', cols)
	}
	in.HandleLine('😀', cols)
	m := Model{
		TermRows: rows, TermCols: cols,
		Input:    in,
	}
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	reallocInputScratches(&m)

	inputLineToBuf(&m, m.screen)

	top := inputRow(&m)
	// Row 0: 79 a's + 1 space (blank gap).
	wantRow0 := append(bytes.Repeat([]byte{'a'}, 79), ' ')
	if got := m.screen[top]; !bytes.Equal(got, wantRow0) {
		t.Errorf("row 0: got %q want %q", got, wantRow0)
	}
	// Row 1: the emoji's 4 UTF-8 bytes only (col 0-1).
	wantRow1 := []byte("😀")
	if got := m.screen[top+1]; !bytes.Equal(got, wantRow1) {
		t.Errorf("row 1: got %q want %q", got, wantRow1)
	}
}

// TestInputLineToBuf_CombiningMarkOverlay encodes the pty
// behavior where a 0-width combining mark appends its UTF-8
// bytes after the base rune without advancing the column: the
// rendered bytes for "e\u0300" are the full 3-byte sequence,
// but cursorCol stays at 1.
func TestInputLineToBuf_CombiningMarkOverlay(t *testing.T) {
	cols, rows := 80, 24
	in := Input{}
	in.HandleLine('e', cols)
	in.HandleLine('\u0300', cols)
	m := Model{
		TermRows: rows, TermCols: cols,
		Input:    in,
	}
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	reallocInputScratches(&m)

	inputLineToBuf(&m, m.screen)

	top := inputRow(&m)
	// The scratch should contain the raw 3-byte sequence
	// "e\u0300" since inputLineToBuf appends the combining
	// mark's UTF-8 bytes to the same row.
	want := []byte("e\u0300")
	if got := m.screen[top]; !bytes.Equal(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
	if m.Input.cursorCol != 1 {
		t.Errorf("cursorCol: got %d want 1", m.Input.cursorCol)
	}
}

// TestInputLineToBuf_MixedWidthAcrossWrap covers the pty
// scenario "78 a's + 你好 at cols=80": 你 fits at cols 78-79 of
// row 0, then 好 wraps to row 1 cols 0-1.
func TestInputLineToBuf_MixedWidthAcrossWrap(t *testing.T) {
	cols, rows := 80, 24
	in := Input{}
	for i := 0; i < 78; i++ {
		in.HandleLine('a', cols)
	}
	in.HandleLine('你', cols)
	in.HandleLine('好', cols)
	m := Model{
		TermRows: rows, TermCols: cols,
		Input:    in,
	}
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	reallocInputScratches(&m)

	inputLineToBuf(&m, m.screen)

	top := inputRow(&m)
	// Row 0: 78 a's + 你 (3 UTF-8 bytes for cols 78-79).
	wantRow0 := append(bytes.Repeat([]byte{'a'}, 78), []byte("你")...)
	if got := m.screen[top]; !bytes.Equal(got, wantRow0) {
		t.Errorf("row 0: got %q want %q", got, wantRow0)
	}
	// Row 1: 好 only.
	wantRow1 := []byte("好")
	if got := m.screen[top+1]; !bytes.Equal(got, wantRow1) {
		t.Errorf("row 1: got %q want %q", got, wantRow1)
	}
}

// TestInputLineToBuf_EmptyBuffer produces no bytes on the single
// input row: the scratch stays at len 0 and the leading edge is
// left blanked by Render's ESC[K.
func TestInputLineToBuf_EmptyBuffer(t *testing.T) {
	cols, rows := 80, 24
	m := newModelWithInput("", cols, rows)
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	reallocInputScratches(&m)

	inputLineToBuf(&m, m.screen)

	top := inputRow(&m)
	if len(m.screen[top]) != 0 {
		t.Errorf("empty buf row: got %d bytes want 0", len(m.screen[top]))
	}
}

// TestInputLineToBuf_ScratchReuse verifies that calling
// inputLineToBuf twice with different buffers reuses the same
// scratch backings (no realloc) and produces the correct output
// for each pass. Catches a reslicing bug where the second pass
// would have leftover bytes from the first.
func TestInputLineToBuf_ScratchReuse(t *testing.T) {
	cols, rows := 80, 24
	m := newModelWithInput(string(repeatRune('a', 200)), cols, rows)
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	reallocInputScratches(&m)

	inputLineToBuf(&m, m.screen)
	firstScratches := make([][]byte, len(m.inputScratches))
	for i, s := range m.inputScratches {
		firstScratches[i] = s
	}

	// Shorter buffer on second pass: 5 a's only.
	m.Input = Input{}
	for i := 0; i < 5; i++ {
		m.Input.HandleLine('a', cols)
	}
	// Re-allocate scratches to match the new inputHeight (1 row
	// instead of 3); NewView does this each frame but the test
	// is calling inputLineToBuf directly to isolate it.
	reallocInputScratches(&m)
	// Confirm the scratches slice shrank to the new height.
	if len(m.inputScratches) != 1 {
		t.Fatalf("realloc: got %d scratches want 1", len(m.inputScratches))
	}
	inputLineToBuf(&m, m.screen)

	// Backing arrays must be reused (same pointer).
	for i := range m.inputScratches {
		if &m.inputScratches[i][:1][0] != &firstScratches[i][:1][0] {
			// Different cap means realloc happened; acceptable
			// only if the row count grew past the prior cap,
			// which it didn't here. Assert pointer equality.
			t.Errorf("scratch %d reallocated", i)
		}
	}

	// Row 0 must hold exactly 5 a's, not the 80 from the first
	// pass (catches the reslice-but-not-truncate bug).
	top := inputRow(&m)
	want := []byte("aaaaa")
	if got := m.screen[top]; !bytes.Equal(got, want) {
		t.Errorf("second pass row 0: got %q want %q", got, want)
	}
	// Row 1 must be empty (len 0), not 80 a's left over.
	if len(m.screen[top+1]) != 0 {
		t.Errorf("second pass row 1: got %d bytes want 0 (reslice bug)",
			len(m.screen[top+1]))
	}
}