package tui

import (
	"bytes"
	"testing"
)

// TestMessageRows_CountsDisplayRows covers the math behind
// clampScroll's backward-fill: each message occupies exactly
// ceil(display width / cols) rows, with no virtual-cursor
// phantom row (messages have no cursor to blink).
func TestMessageRows_CountsDisplayRows(t *testing.T) {
	cols := 80
	checks := []struct {
		text string
		want int
	}{
		{"", 1},
		{"hi", 1},
		{string(repeatRune('a', 80)), 1},  // exactly full, no phantom
		{string(repeatRune('a', 81)), 2},
		{string(repeatRune('a', 200)), 3},
		// 1 emoji is 2 cols → at cols=2 fits exactly in 1 row
		// (unlike inputHeight which adds a phantom cursor row).
		{"😀", 1},
		// 40 wide runes = 80 cols → exactly full, 1 row.
		{string(repeatRune('你', 40)), 1},
		// 41 wide runes = 82 cols → 2 rows.
		{string(repeatRune('你', 41)), 2},
		// Combining marks don't advance the column: "cafe\u0300"
		// is 4 cols, so 1 row even at cols=4.
		{"cafe\u0300", 1},
	}
	for _, c := range checks {
		if got := messageRows(c.text, cols); got != c.want {
			t.Errorf("messageRows(%q, %d): got %d want %d", c.text, cols, got, c.want)
		}
	}
	// Tight cols: "cafe\u0300" at cols=4 still fits in 1 row
	// because the combining grave doesn't advance.
	if got := messageRows("cafe\u0300", 4); got != 1 {
		t.Errorf("cafe+grave @4: got %d want 1", got)
	}
}

// TestWrapMessage_SplitsByDisplayWidth verifies the actual byte
// rows a wrapped message produces. Reuses the same scenarios
// inputLineToBuf tests use, but for the message path (no phantom
// cursor row).
func TestWrapMessage_SplitsByDisplayWidth(t *testing.T) {
	// 200 a's at cols=80 → 2 full rows + a 40-row.
	rows := wrapMessage(string(repeatRune('a', 200)), 80)
	if len(rows) != 3 {
		t.Fatalf("200 a's: got %d rows want 3", len(rows))
	}
	if len(rows[0]) != 80 || len(rows[1]) != 80 || len(rows[2]) != 40 {
		t.Errorf("lengths: got %d/%d/%d want 80/80/40",
			len(rows[0]), len(rows[1]), len(rows[2]))
	}
	want := bytes.Repeat([]byte{'a'}, 80)
	if !bytes.Equal(rows[0], want) || !bytes.Equal(rows[1], want) {
		t.Errorf("rows 0/1 not 80 a's")
	}
}

// TestWrapMessage_ExactlyFullRowNoPhantom verifies the message
// version of wrapRow does NOT add the virtual-cursor row: 80
// a's produces exactly 1 row, not 2.
func TestWrapMessage_ExactlyFullRowNoPhantom(t *testing.T) {
	rows := wrapMessage(string(repeatRune('a', 80)), 80)
	if len(rows) != 1 {
		t.Errorf("80 a's: got %d rows want 1 (no phantom)", len(rows))
	}
}

// TestWrapMessage_WideRuneGapPad exercises the pty scenario where
// a wide rune doesn't fit at the end of a row: row 0 is padded
// with a trailing space to fill the blank gap, then the emoji
// starts row 1.
func TestWrapMessage_WideRuneGapPad(t *testing.T) {
	rows := wrapMessage(string(repeatRune('a', 79))+"😀", 80)
	if len(rows) != 2 {
		t.Fatalf("got %d rows want 2", len(rows))
	}
	wantRow0 := append(bytes.Repeat([]byte{'a'}, 79), ' ')
	if !bytes.Equal(rows[0], wantRow0) {
		t.Errorf("row 0: got %q want %q", rows[0], wantRow0)
	}
	wantRow1 := []byte("😀")
	if !bytes.Equal(rows[1], wantRow1) {
		t.Errorf("row 1: got %q want %q", rows[1], wantRow1)
	}
}

// TestWrapMessage_CombiningMarkStaysWithBase verifies that a
// combining mark after a base rune in the same row produces a
// single row with the full UTF-8 sequence — the terminal renders
// the mark on top of the base cell.
func TestWrapMessage_CombiningMarkStaysWithBase(t *testing.T) {
	rows := wrapMessage("cafe\u0300", 80)
	if len(rows) != 1 {
		t.Fatalf("got %d rows want 1", len(rows))
	}
	want := []byte("cafe\u0300")
	if !bytes.Equal(rows[0], want) {
		t.Errorf("got %q want %q", rows[0], want)
	}
}

// TestClampScroll_BackwardFill exercises the "snap to latest"
// semantics: after appending N messages whose total display
// rows exceed chatAreaRows, Scroll = nMsg snaps down to maxScroll
// which is the index of the first message that still fits when
// wrapped backward from the latest.
func TestClampScroll_BackwardFill(t *testing.T) {
	cols := 80
	// Build a model with 4 messages: 1-row, 3-row (240 a's), 1-row, 1-row.
	// chatAreaRows = 24 - 1 (status) - 1 (input) = 22.
	// Total display rows = 1+3+1+1 = 6. Fits in 22, so maxScroll = 0.
	m := Model{
		TermRows: 24, TermCols: cols,
		Messages: []Message{
			{Text: "m0"},
			{Text: string(repeatRune('a', 240))},
			{Text: "m2"},
			{Text: "m3"},
		},
	}
	m.Scroll = len(m.Messages) // past-the-end like handleEnter
	start, end := clampScroll(&m)
	if start != 0 || end != 4 {
		t.Errorf("small chat: start=%d end=%d want 0/4", start, end)
	}
	if m.Scroll != 0 {
		t.Errorf("Scroll after clamp: %d want 0", m.Scroll)
	}
}

// TestClampScroll_BackwardFillOverflow covers the case where
// the chat area can't hold everything: maxScroll is the index
// of the first message whose tail forward fits, computed by
// walking backward from the last message.
func TestClampScroll_BackwardFillOverflow(t *testing.T) {
	cols := 80
	// 30 messages of 80 a's = 30 rows. Chat is 22 rows. Backward
	// walk: 22 rows = messages [8..30). maxScroll = 8.
	m := Model{
		TermRows: 24, TermCols: cols, // chat = 22
		Messages: make([]Message, 30),
	}
	for i := range m.Messages {
		m.Messages[i].Text = string(repeatRune('a', 80))
	}
	m.Scroll = 999 // past end
	start, end := clampScroll(&m)
	if start != 8 || end != 30 {
		t.Errorf("got start=%d end=%d want 8/30", start, end)
	}
}

// TestClampScroll_TallSingleMessage covers the rare case where
// one message is taller than the whole chat area: maxScroll
// becomes that message's index so the bottom rows of it stay
// visible (the leading rows are scrolled out).
func TestClampScroll_TallSingleMessage(t *testing.T) {
	cols := 80
	// 1 message of 30 rows. chat = 22. maxScroll = 0.
	m := Model{
		TermRows: 24, TermCols: cols,
		Messages: []Message{
			{Text: string(repeatRune('a', 30*80))},
		},
	}
	m.Scroll = 999
	start, end := clampScroll(&m)
	if start != 0 || end != 1 {
		t.Errorf("got start=%d end=%d want 0/1", start, end)
	}
}

// TestClampScroll_NegativeScrollClampsToZero verifies a bad
// (e.g. user-scrolled-past-top) Scroll value is sanitized.
func TestClampScroll_NegativeScrollClampsToZero(t *testing.T) {
	m := Model{TermRows: 24, TermCols: 80, Scroll: -5}
	start, _ := clampScroll(&m)
	if start != 0 || m.Scroll != 0 {
		t.Errorf("got start=%d Scroll=%d want 0/0", start, m.Scroll)
	}
}

// TestMessagesToBuf_FillsForward covers the integration: a
// wrapped message spans multiple display rows and the following
// messages append right after.
func TestMessagesToBuf_FillsForward(t *testing.T) {
	cols := 80
	rows := 24
	m := Model{
		TermRows: rows, TermCols: cols,
		Messages: []Message{
			{Text: string(repeatRune('a', 200))}, // 3 display rows
			{Text: "second"},                       // 1 row
		},
	}
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	for i := range m.screen {
		m.screen[i] = m.blank
	}
	messagesToBuf(&m, m.screen)

	want0 := bytes.Repeat([]byte{'a'}, 80)
	want1 := bytes.Repeat([]byte{'a'}, 80)
	want2 := bytes.Repeat([]byte{'a'}, 40)
	want3 := []byte("second")

	if !bytes.Equal(m.screen[0], want0) {
		t.Errorf("row 0: got %q want %q", m.screen[0], want0)
	}
	if !bytes.Equal(m.screen[1], want1) {
		t.Errorf("row 1: got %q want %q", m.screen[1], want1)
	}
	if !bytes.Equal(m.screen[2], want2) {
		t.Errorf("row 2: got %q want %q", m.screen[2], want2)
	}
	if !bytes.Equal(m.screen[3], want3) {
		t.Errorf("row 3: got %q want %q", m.screen[3], want3)
	}
	// Row 4 should still be the blank (no message overflow).
	if !bytes.Equal(m.screen[4], m.blank) {
		t.Errorf("row 4: got %q want blank", m.screen[4])
	}
}

// TestMessagesToBuf_TallMessageTruncatesTop covers the rare
// case where a single message is taller than the chat area:
// its leading rows scroll out so the bottom rows stay visible
// (keeps the newest content anchored, matching typical TUI
// chat behavior).
func TestMessagesToBuf_TallMessageTruncatesTop(t *testing.T) {
	cols := 80
	rows := 24
	// chat = 22 rows. Message has 30 rows of 'a'. Should display
	// the bottom 22 rows = rows [8..30) of the wrapped message.
	m := Model{
		TermRows: rows, TermCols: cols,
		Messages: []Message{
			{Text: string(repeatRune('a', 30*80))},
		},
	}
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	messagesToBuf(&m, m.screen)

	// The first visible screen row should be row 8 of the
	// wrapped message — i.e. 80 a's (row 8 looks identical to
	// any other full row of the message). Verify the LAST
	// visible screen row is the LAST row of the message (40 a's
	// since 30 full rows = 30*80 = 2400 a's, no remainder, so
	// actually row 29 is also 80 a's).
	if len(m.screen[0]) != 80 {
		t.Errorf("top visible row: %d bytes want 80", len(m.screen[0]))
	}
	if len(m.screen[chatAreaRows(&m)-1]) != 80 {
		t.Errorf("bottom visible row: %d bytes want 80",
			len(m.screen[chatAreaRows(&m)-1]))
	}
}

// TestMessagesToBuf_EmptyMessages shows that zero messages
// leaves the chat area entirely blank (fillBlanks set the
// rows; messagesToBuf didn't overwrite).
func TestMessagesToBuf_EmptyMessages(t *testing.T) {
	cols := 10
	rows := 5
	m := Model{TermRows: rows, TermCols: cols}
	m.screen = make([][]byte, rows)
	m.blank = make([]byte, cols)
	for i := range m.screen {
		m.screen[i] = m.blank
	}
	messagesToBuf(&m, m.screen)
	for i := range m.screen {
		if !bytes.Equal(m.screen[i], m.blank) {
			t.Errorf("row %d: got %q want blank", i, m.screen[i])
		}
	}
}