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
   CSI, EOF, Quit).
2. `HandleKeyPress` calls `pollResize` first — on change it
   recomputes `TermCols`/`TermRows`, clamps the cursor, marks
   every row dirty.
3. The event is dispatched to `handleChar` / `handleCtrl` /
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
- `layout.go` — `inputRow` / `statusBarRow` / `chatAreaRows`
  derived from `inputHeight` + `statusBarHeight` constants so
  multi-line input growth is a one-line change.
- `input.go` — `Input` struct (buf, cursor), handlers, the
  resize poll, the input loop dispatcher.
- `view.go` — `NewView` orchestrator + helpers (`sizeOK`,
  `reallocScreenBufs`, `fillBlanks`, `clampScroll`,
  `messagesToBuf`, `statusBarToBuf`, `inputLineToBuf`,
  `statusBarView`, `fillLine`).
- `render.go` — `Render(screen, dirty, statusRow)` flushes to
  the terminal, applies `ESC[K` after each row except the
  status row.
- `width.go` — `runeWidth` (delegates to `go-runewidth`) and
  `truncateToCols`, rune-aware width math for message rows.
- `debug.go` / `release.go` — `//go:build debug` split; the
  release build pulls no `log` import, the debug build ships
  `dbgInput` / `dbgModel` trace logging.

## Screen layout

```text
0 .. inputRow-1            messages (scrollable via Model.Scroll)
inputRow .. inputRow+H-1   multi-line input (H rows; grows with the buffer)
statusBarRow               colored status bar
```

`H` = `inputHeight(model)`, derived from the byte length of the
input buffer — typing past `cols` bumps H to 2, then 3, ... and
the chat area shrinks by the same amount. The cursor row inside
the input area is `cursorY` (derived from `cursorX`); the row
the cursor blinks on is `inputRow + cursorY`.

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

## Truncation

`messagesToBuf` uses `truncateToCols(Text, TermCols)` and slices
the message by the returned byte offset. The walk is rune-aware:
Italian accents and emoji never get cut mid-rune, and a wide
rune that doesn't fit is dropped wholesale rather than split. The
width math delegates to `mattn/go-runewidth` so ZWJ sequences,
regional indicators, and CJK terminal detection are correct.

## Debug gating

- `dbgInput` / `dbgModel` compile to empty no-ops in release
  builds. Build with `go build -tags debug` to see the logs.
- The `cols x rows` indicator on the status bar is gated by a
  runtime `Model.Debug` bool — flip it when you want to inspect
  a layout without rebuilding.

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
- Layout is derived from `inputHeight` + `statusBarHeight`
  constants, so a multi-line input is a one-line change in
  principle (the plumbing is ready; see below).
- Stack-allocated fixed buffers in `statusBarView` and direct
  `strconv.AppendInt` instead of `fmt.Sprintf` — matches the
  "no Sprintf" rule.

# Cons / known limits

- Multi-line input is byte-oriented: `HandleLine(ch byte, cols)`
  operates on raw bytes, so a multi-byte UTF-8 sequence arriving
  via raw mode is split into per-byte events and corrupts the
  buffer. Need rune reconstruction before non-ASCII typing
  composes correctly inside the wrap math.
- The wrap math itself is byte-based: `cursorRow = (cursorX-1)/cols`
  and `inputHeight = (len(buf)-1)/cols + 1` assume one byte per
  column, so any rune wider than 1 byte desyncs the cursor.
  `width.go` already has rune-aware width math; the input path
  needs to use it next.
- No Up/Down CSI handlers — the cursor can only move within a
  row via Left/Right. Vertical navigation across wrapped rows
  is the next gap.
- Message rows still hard-truncate at `TermCols` via
  `truncateToCols`. Long messages do not wrap onto the next chat
  row; that's the other half of soft-wrap.
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
- No tests.
- The input loop blocks on `term.ReadKey`. The model is not
  concurrency-safe; everything today runs on the main goroutine.

# Roadmap (priority order)

1. **Rune-aware input + wrap math**
   `HandleLine` accepts runes, `cursorRow` / `inputHeight` use
   `runeWidth` (already in `width.go`) instead of byte-index
   math. Then Up/Down CSI to navigate across wrapped rows by
   screen row; Enter at end-of-buf submits, Shift+Enter inserts
   a newline.
2. **Message wrapping**
   `messagesToBuf` today hard-truncates each message to
   `TermCols`; long messages disappear off the right edge. Make
   it wrap vertically so a single message can occupy multiple
   chat rows. `chatAreaRows` then needs to count *displayed*
   rows, not message indices.
3. **Goroutines for input / render / agent**
   Today the input loop is a blocking `for { term.ReadKey }`.
   Split into three: one reads keys, one renders frames, one
   will talk to the model. They communicate over channels; the
   render goroutine owns `Model` to avoid races. Unblocks
   streaming output later.
4. **Theming stub**
   `type Theme struct{ StatusBarBg, StatusBarFg [3]byte }` (and
   friends), passed into `statusBarView`. Removes the raw
   `\033[38;2;0;0;0m\033[48;2;0;180;244m` literals. Start
   hardcoded — no config file yet.
5. **Tests**
   `truncateToCols`, `clampScroll`, `layout.go` math, the dirty-mask
   plumbing, and `cursorRow` / `inputHeight` derivation are all
   pure functions — easy wins. Then add a fake `term` interface
   for `HandleKeyPress`-level tests.

The next milestone is #1. Everything else stacks on top of it.