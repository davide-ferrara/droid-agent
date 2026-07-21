# tui/ architecture

# How the TUI works

`tui/` is a small Elm-like loop on top of the `term/` raw-mode
package. There is no framework — every byte that reaches stdout
goes through `Render`, and every keystroke flows through
`HandleKeyPress`.

## The loop

```text
ReadKey ─▶ HandleKeyPress ─▶ Input/Model mutation ─▶ NewView
                                      │              │
                                      │              ▼
                                 dirtyRows ◀── screen [][]byte
                                      │              │
                                      ▼              ▼
                                   Render ─▶ escape codes ─▶ TTY
```

1. `term.ReadKey` returns one event per key (printable, Ctrl,
   CSI, EOF, Quit). Printables carry a `Rune` field — multi-byte
   UTF-8 sequences arriving in raw mode are decoded via
   `bufio.Reader.ReadRune` so callers never see half-formed
   sequences.
2. `HandleKeyPress` calls `pollResize` first — on change it
   recomputes `TermCols`/`TermRows`, clamps the cursor, marks
   every row dirty.
3. The event is dispatched to `handleRune` / `handleCtrl` /
   `handleCSI`.
4. Each handler mutates `Input` or `Messages`, then sets the
   relevant bits in `dirtyRows`. Cursor-only moves skip the
   repaint entirely and just `MoveCursorTo`.
5. `renderFrame` calls `NewView` then `Render`. `NewView` fills
   the persistent `screen [][]byte`; `Render` writes only dirty
   rows and clears their bits.

## Files

- `tui.go` — `Run()`, opens the input reader and calls
  `HandleKeyPress`.
- `model.go` — `Model`, `Message`, `Mode`, `NewModel`.
- `layout.go` — `inputRow(*Model)` / `statusBarRow(termRows)` /
  `chatAreaRows(*Model)` derived from `inputHeight(*Model)`
  (rune-aware) and `statusBarHeight` constant so multi-line
  input growth is a one-line change conceptually.
- `input.go` — `Input` struct (`buf []rune`, cursorX rune index,
  cached cursorY/cursorCol), handlers (`HandleLine` takes a
  rune), the resize poll, the input loop dispatcher.
- `view.go` — `NewView` orchestrator + helpers (`sizeOK`,
  `reallocScreenBufs`, `fillBlanks`, `maxScroll`,
  `clampScroll`, `messagesToBuf` (wraps), `statusBarToBuf`,
  `inputLineToBuf` (wraps), `statusBarView`, `fillLine`).
- `render.go` — `Render(screen, dirty, statusRow)` flushes to
  the terminal, applies `ESC[K` after each row except the
  status row.
- `width.go` — three width primitives:
  `runeWidth` (delegates to `go-runewidth`),
  `wrapRow(buf, idx, cols)` (cursor position inside a wrapped
  input buffer, with virtual-cursor rule),
  `wrapMessage(text, cols)` + `messageRows(text, cols)` (split
  a chat message into display rows, no virtual cursor), and
  `truncateToCols` (legacy byte-offset truncation kept for
  callers that want a single short row instead of a wrap).
- `debug.go` / `release.go` — `//go:build debug` split; the
  release build pulls no `log` import, the debug build ships
  `dbgInput` / `dbgModel` trace logging.

## Screen layout

```text
0 .. inputRow-1            messages (scrollable via Model.Scroll)
inputRow .. inputRow+H-1   multi-line input (H rows; grows with the buffer)
statusBarRow               colored status bar
```

`H` = `inputHeight(model)`, derived from the rune-aware
`wrapRow(buf, len(buf), cols)` so wide runes (CJK, emoji) bump
the height the right number of times and combining marks don't
advance it at all. The cursor row inside the input area is
`cursorY` (cached from `wrapRow`) and the on-screen column is
`cursorCol`; the cell the cursor blinks on is
`inputRow + cursorY` at column `cursorCol`.

## Persistent buffers

`Model` carries three reusable buffers so a keystroke in the
common case allocates zero bytes:

- `screen [][]byte` — one slice header per row, indexed as above
- `blank []byte` — one reusable blank row, shared by every empty
  chat row (safe because blank rows are never mutated in place)
- `inputScratches [][]byte` — one scratch per wrapped input row,
  each `len == TermCols`; grows with `inputHeight` (per-row
  capacities are checked independently so a horizontal resize
  only reallocates the rows whose `cap` is below `cols`)
- `dirtyRows []bool` — row-level dirty mask

`reallocScreenBufs` only reallocates `screen`/`blank`/`dirtyRows`
on resize (cap check); `inputScratches` is checked every frame
because `inputHeight` can change any time `buf` grows past a
wrap. The dirty mask starts fully set right after a full realloc
so the first paint covers the whole screen.

## Wrapping (input + messages)

Both the input line and chat messages wrap via the same width
math (runeWidth sums):

- `wrapRow(buf []rune, idx, cols)` walks runes [0, idx) and
  returns the (row, col) where the cursor sits. It applies the
  virtual-cursor rule: a fully-filled row's cursor visually
  wraps to (row+1, col 0). Used by `Input.syncCursor` and
  `inputHeight`.
- `wrapMessage(text string, cols)` splits a message into its
  display rows, WITHOUT the virtual-cursor rule (a message has
  no cursor to blink). Returns the byte content of each row.
  Used by `messagesToBuf` to fill the chat area.
- `messageRows(text, cols)` returns only the count, used by
  `maxScroll`'s backward-fill.

Wide runes that don't fit at the end of a row wrap to the next,
leaving a trailing blank gap on the previous row (padded with
spaces in the scratch so the gap is visually correct).
Combining marks (runeWidth 0) append their UTF-8 bytes after the
base rune without advancing the column — the terminal overlays
them on the same cell.

## Scrolling

`Model.Scroll` is the index of the first visible message. With
wrapping, a message can span multiple display rows, so the upper
bound for Scroll is computed by backward-fill: walk from the last
message accumulating `messageRows` until `chatAreaRows` is hit.
That gives `maxScroll` — the smallest index whose tail fits.
`handleEnter` sets `Scroll = len(Messages)` and lets
`clampScroll` snap it down to `maxScroll`, anchoring the newest
message at the bottom. `pageUpShift` / `pageDownShift` shift
`Scroll` by `chatAreaRows` indices and clamp the same way.

Edge case: when even the latest message alone is taller than the
chat area, `maxScroll = nMsg - 1` so the message still shows
(its leading rows scroll out, keeping the bottom rows visible —
typical TUI chat behavior).

## Debug gating

- `dbgInput` / `dbgModel` compile to empty no-ops in release
  builds. Build with `go build -tags debug` to see the logs.
- The `cols x rows` indicator on the status bar is gated by a
  runtime `Model.Debug` bool — flip it when you want to inspect
  a layout without rebuilding.

## Tests

Pure-function units in `*_test.go` (run with `go test ./tui/`):

- `width_test.go` — `wrapRow` (ASCII, wide runes, combining
  marks, bounds clamping) and `truncateToCols` (mixed-UTF-8
  byte-offset math).
- `layout_test.go` — `inputHeight` (floor=1, ASCII wrap, wide
  runes, combining marks), `inputRow`, `chatAreaRows`,
  `statusBarRow`.
- `input_test.go` — `HandleLine` (rune append, wrap, wide-rune
  overflow gap, combining mark no-advance), `HandleBackspace`
  (whole-rune removal, wrap retract, no-op at zero),
  `HandleLeft`/`HandleRight` (wrap boundary, no-op at ends),
  `HandleEnter` (reset), `cursorRow` derivation.
- `view_test.go` — `inputLineToBuf` (ASCII wraps, wide-rune
  gap at row end with blank padding, combining mark overlay,
  mixed-width wraps, empty buffer, scratch backing reuse
  across frames).

Each pty smoke scenario from the manual wrap-debugging session
is encoded as a named test case so future regressions are
caught without re-running the pty harness.

# Pros

- Small and idiomatic. Each file has one responsibility; every
  function is short and named for what it does.
- Allocation-free keystrokes thanks to persistent `screen` /
  `blank` / `inputScratches` buffers on `Model`.
- Multi-line input via soft-wrap: typing past `cols` bumps
  `inputHeight` and the chat area shrinks by the same number of
  rows. Backspace correctly retracts wrap; Left/Right cross wrap
  boundaries without repaints (cursor-only moves reposition via
  `MoveCursorTo`).
- Row-level dirty mask — cursor-only moves skip `Render`
  entirely and just reposition via `MoveCursorTo`.
- Rune-aware truncation: Italian accents, CJK, emoji, ZWJ all
  render correctly. No broken bytes on the screen edge.
- Scrolling (`Model.Scroll` + PageUp/PageDown); `handleEnter`
  snaps to latest so new messages are immediately visible.
- Bounds-checked everywhere: `TermRows < 2` / `TermCols < 1`
  returns a friendly one-line notice instead of panicking on
  `screenBuf[-1]`.
- Debug logging and the cols/rows indicator both gated —
  release builds pull no `log` import.
- Layout is derived from `inputHeight(*Model)` (rune-aware) +
  `statusBarHeight` constant, so multi-line input growth is a
  one-line change in principle; the math is centralized.
- Stack-allocated fixed buffers in `statusBarView` and direct
  `strconv.AppendInt` instead of `fmt.Sprintf` — matches the
  "no Sprintf" rule.
- Rune-aware input end to end: `term.ReadKey` decodes multi-byte
  UTF-8 sequences via `bufio.Reader.ReadRune`, `Input.buf` is
  `[]rune`, `HandleLine` takes a rune, and `wrapRow` sums
  display widths to derive `(row, col)` so wide runes (CJK,
  emoji) and combining marks compose correctly inside the wrap.
- Message wrapping: long messages flow across multiple chat
  rows instead of being hard-truncated at the right edge; the
  chat area renders wrapped rows forward from `Scroll`.

# Cons / known limits

- No Up/Down CSI handlers — the cursor can only move within a
  wrapped row via Left/Right. Vertical navigation across wrapped
  rows is the next gap.
- PageUp / PageDown shift `Scroll` by `chatAreaRows` indices,
  not by display rows; the jump size is approximate when
  wrapped messages dominate the viewport. Could be reworked to
  walk backward by `chatAreaRows` display rows if precision
  matters.
- No streaming output / spinners for `ModeLoading` and
  `ModeStreaming`. The modes exist but the visual does not.
- No theming hook. ANSI color codes are inlined in
  `statusBarView`. The AGENTS.md theming goal is not addressed.
- No Home/End/Delete CSI handlers in `handleCSI`.
- `handleCtrl` is inconsistent on exit: Ctrl-C prints `"CtrlC"`
  via `KindQuit`, Ctrl-D returns silently via `KindEOF`.
- `ModelName: "DeepSeek v4 (Pro)"` is hardcoded in `NewModel`;
  no config injection path exists yet despite AGENTS.md
  mentioning swappable offline models.
- `handleCtrl`'s "unhandled" branch and `handleCSI`'s default
  branch still hit `log.Printf` directly, not gated.
- DEBUG message seed in `NewModel` (mixed UTF-8 fixtures) needs
  removing once real agent output is wired in.
- No end-to-end / pty integration test harness — the smoke
  scenarios from the wrap-debugging session are encoded as
  unit tests against the pure functions and `inputLineToBuf`,
  but nothing drives `HandleKeyPress` over a fake `term` reader.
- The input loop blocks on `term.ReadKey`. The model is not
  concurrency-safe; everything today runs on the main goroutine.

# Roadmap (priority order)

1. **Up/Down CSI to navigate across wrapped rows**
   Now that `Input.cursorY` is real and stays in sync across
   wrap math, wire `handleCSI` cases for `term.Up` / `term.Down`
   so the cursor moves by one wrapped row at a time, snapping to
   the start or end of the destination row as appropriate.
2. **Goroutines for input / render / agent**
   Today the input loop is a blocking `for { term.ReadKey }`.
   Split into three: one reads keys, one renders frames, one
   will talk to the model. They communicate over channels; the
   render goroutine owns `Model` to avoid races. Unblocks
   streaming output later.
3. **Theming stub**
   `type Theme struct{ StatusBarBg, StatusBarFg [3]byte }` (and
   friends), passed into `statusBarView`. Removes the raw
   `\033[38;2;0;0;0m\033[48;2;0;180;244m` literals. Start
   hardcoded — no config file yet.
4. **Tests — extend to integration level**
   `wrapRow`, `wrapMessage`, `messageRows`, `truncateToCols`,
   `inputHeight`, `inputRow`, `chatAreaRows`, `maxScroll`,
   `HandleLine` / `HandleBackspace` / `HandleLeft` /
   `HandleRight` / `HandleEnter`, `messagesToBuf`, and
   `inputLineToBuf` are all covered by unit tests encoding the
   pty smoke scenarios as named cases. Next gap: a fake
   `term` reader interface so `HandleKeyPress`-level tests can
   drive multi-event sequences (typing + resizing + scrolling in
   one run) without spawning a pty.

The next milestone is #1. Everything else stacks on top of it.