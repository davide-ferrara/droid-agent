# tui/ architecture

# How the TUI works

`tui/` is a small Elm-like loop on top of the `term/` raw-mode
package. There is no framework — every byte that reaches stdout
goes through `Render`, and every keystroke flows through
`HandleInput`.

## The loop

```text
ReadKey ─▶ HandleInput ─▶ Input/Model mutation ─▶ NewView
                                      │              │
                                      │              ▼
                                 dirtyRows ◀── screen [][]byte
                                      │              │
                                      ▼              ▼
                                   Render ─▶ escape codes ─▶ TTY
```

1. `term.ReadKey` returns one event per key (printable, Ctrl,
   CSI, EOF, Quit).
2. `HandleInput` calls `pollResize` first — on change it
   recomputes `TermCols`/`TermRows`, clamps the cursor, marks
   every row dirty.
3. The event is dispatched to `handleInput` / `handleCtrl` /
   `handleCSI`.
4. Each handler mutates `Input` or `Messages`, then sets the
   relevant bits in `dirtyRows`. Cursor-only moves skip the
   repaint entirely and just `MoveCursorTo`.
5. `renderInput` calls `NewView` then `Render`. `NewView` fills
   the persistent `screen [][]byte`; `Render` writes only dirty
   rows and clears their bits.

## Files

- `tui.go` — `Run()`, opens the input reader and calls
  `HandleInput`.
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
0 .. chatAreaRows-1        messages (scrollable via Model.Scroll)
chatAreaRows .. inputRow-1 reserved for future input growth
inputRow                   single-line input (today)
statusBarRow               colored status bar
```

## Persistent buffers

`Model` carries three reusable buffers so a keystroke in the
common case allocates zero bytes:

- `screen [][]byte` — one slice header per row, indexed as above
- `blank []byte` — one reusable blank row, shared by every empty
  chat row (safe because blank rows are never mutated in place)
- `inputScratch []byte` — a private scratch row for the input
  line; the input row is the one row that does get mutated, so
  it avoids aliasing `blank`
- `dirtyRows []bool` — row-level dirty mask

`reallocScreenBufs` only reallocates on resize (cap check);
otherwise it reslices to the new dimensions. The dirty mask
starts fully set right after a realloc so the first paint covers
the whole screen.

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
  `blank` / `inputScratch` buffers on `Model`.
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

- Single-line input only. `Input.cursorY` is reserved for future
  soft-wrap but always zero today. No multi-line editing,
  no vertical cursor movement, no up/down CSI handling.
- `HandleLine(ch byte)` operates on raw bytes. UTF-8 sequences
  received in raw mode are split into individual byte events,
  so typing non-ASCII characters corrupts the buffer. Needs
  rune reconstruction before multi-line input is useful.
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

1. **Multi-line cursor + message rendering**
   Make `Input.cursorY` real: when `buf` exceeds `TermCols` it
   should soft-wrap into `inputHeight > 1` rows, shrinking the
   chat area via `chatAreaRows`. Up/Down arrows move by screen
   row, Enter at end-of-buf submits, Shift+Enter inserts a
   newline (or whatever keybinding wins). Messages also need
   wrap-aware rendering — long messages currently truncate at
   the right edge, they should flow onto the next row.
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
4. **Tests**
   `truncateToCols`, `clampScroll`, `layout.go` math, and the
   dirty-mask plumbing are all pure functions — easy wins. Then
   add a fake `term` interface for `HandleInput`-level tests.

The next milestone is #1. Everything else stacks on top of it.