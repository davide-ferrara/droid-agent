package tui

import "testing"

// TestWrapRow_ASCII covers byte-by-byte ASCII walks:
//   - empty buffer (idx 0) sits at row 0 col 0
//   - mid-buffer cursor lands on (row, col) by simple index math
//   - end-of-full-row advances to next row col 0 (virtual cursor)
func TestWrapRow_ASCII(t *testing.T) {
	cols := 80
	buf := []rune("hello")
	for i := 0; i <= len(buf); i++ {
		row, col := wrapRow(buf, i, cols)
		if row != 0 || col != i {
			t.Errorf("idx=%d: got (%d,%d) want (0,%d)", i, row, col, i)
		}
	}
	// Exactly cols: cursor visually wraps to next row col 0.
	full := []rune(string(repeatRune('a', cols)))
	row, col := wrapRow(full, cols, cols)
	if row != 1 || col != 0 {
		t.Errorf("full row: got (%d,%d) want (1,0)", row, col)
	}
	// Past a full row by 5 chars.
	past := []rune(string(repeatRune('a', cols+5)))
	row, col = wrapRow(past, len(past), cols)
	if row != 1 || col != 5 {
		t.Errorf("past row by 5: got (%d,%d) want (1,5)", row, col)
	}
}

// TestWrapRow_WideRunes covers CJK / emoji (width 2) and the
// gap-at-end-of-row rule: a 2-col rune that doesn't fit must
// wrap to next row leaving the trailing cell of the previous
// row blank.
func TestWrapRow_WideRunes(t *testing.T) {
	cols := 4
	// 2 wide runes fit exactly: cursor visually on next row.
	full := []rune("你好")
	row, col := wrapRow(full, 2, cols)
	if row != 1 || col != 0 {
		t.Errorf("2 wide @ cols=4: got (%d,%d) want (1,0)", row, col)
	}
	// 1 ASCII + 2 wide: a at col 0, 你 at 1-2, 好 at cols 3-4 would
	// overflow → wraps. Layout: row 0 = a你 (cols 0,1-2 col 3 blank),
	// row 1 = 好 at 0-1.
	mixed := []rune("a你好")
	row, col = wrapRow(mixed, 3, cols)
	if row != 1 || col != 2 {
		t.Errorf("a你好 @ cols=4: got (%d,%d) want (1,2)", row, col)
	}
	// Cursor between 你 and 好: col 3 on row 0 (after 你 consumed
	// cols 1-2, cursor at col 3, that's the blank-gap cell).
	row, col = wrapRow(mixed, 2, cols)
	if row != 0 || col != 3 {
		t.Errorf("after 你 @ cols=4: got (%d,%d) want (0,3)", row, col)
	}
}

// TestWrapRow_CombiningMarks verifies that runes with width 0
// (combining marks like U+0300 GRAVE ACCENT) do not advance the
// column, so "cafè" typed as decomposed "cafe\u0300" lands the
// cursor at col 4 (same cell as the e), not col 5.
func TestWrapRow_CombiningMarks(t *testing.T) {
	cols := 80
	// 'e' followed by combining grave
	buf := []rune("cafe\u0300")
	row, col := wrapRow(buf, len(buf), cols)
	if row != 0 || col != 4 {
		t.Errorf("cafe+combining: got (%d,%d) want (0,4)", row, col)
	}
	// Cursor right after the combining mark: still col 4.
	row, col = wrapRow(buf, 5, cols)
	if row != 0 || col != 4 {
		t.Errorf("after combining: got (%d,%d) want (0,4)", row, col)
	}
	// Cursor between e and combining: col 4 (e sits at col 3,
	// cursor after it).
	row, col = wrapRow(buf, 4, cols)
	if row != 0 || col != 4 {
		t.Errorf("before combining: got (%d,%d) want (0,4)", row, col)
	}
}

// TestWrapRow_ClampsBounds forbids negative idx, treats idx > len
// as len, and returns (0,0) for cols <= 0.
func TestWrapRow_ClampsBounds(t *testing.T) {
	buf := []rune("hi")
	if row, col := wrapRow(buf, -1, 80); row != 0 || col != 0 {
		t.Errorf("idx=-1: got (%d,%d) want (0,0)", row, col)
	}
	if row, col := wrapRow(buf, 99, 80); row != 0 || col != 2 {
		t.Errorf("idx>len: got (%d,%d) want (0,2)", row, col)
	}
	if row, col := wrapRow(buf, 1, 0); row != 0 || col != 0 {
		t.Errorf("cols=0: got (%d,%d) want (0,0)", row, col)
	}
	if row, col := wrapRow(buf, 1, -1); row != 0 || col != 0 {
		t.Errorf("cols<0: got (%d,%d) want (0,0)", row, col)
	}
}

// TestTruncateToCols_MixedUTF8 locks in rune-aware byte-offset
// truncation: combining marks stay attached to their base, wide
// runes don't get cut mid-encoding, and a wide rune that exactly
// fits is kept.
func TestTruncateToCols_MixedUTF8(t *testing.T) {
	// fits entirely
	end, used := truncateToCols("caffè", 80)
	if end != len("caffè") || used != 5 {
		t.Errorf("caffè @ 80: got end=%d used=%d want %d/5", end, used, len("caffè"))
	}
	// wide rune that fits in cols=2 is kept
	end, used = truncateToCols("你好", 2)
	if end != 3 || used != 2 {
		t.Errorf("你好 @ 2: got end=%d used=%d want 3/2", end, used)
	}
	// wide rune that doesn't fit in 1 col is dropped
	end, used = truncateToCols("你好", 1)
	if end != 0 || used != 0 {
		t.Errorf("你好 @ 1: got end=%d used=%d want 0/0", end, used)
	}
	// combining mark at the boundary is preserved with its base
	end, used = truncateToCols("cafe\u0300", 4)
	if end != len("cafe\u0300") || used != 4 {
		t.Errorf("cafe+grave @ 4: got end=%d used=%d want %d/4", end, used, len("cafe\u0300"))
	}
}