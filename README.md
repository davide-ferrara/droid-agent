# droid

LLM agent for Linux with a minimal TUI built in Go.

Near-zero dependencies: `golang.org/x/sys` for raw termios,
`github.com/mattn/go-runewidth` for correct UTF-8 column widths
(Italian accents, CJK, emoji, ZWJ sequences). No TUI framework,
no LLM SDK — just stdlib and those two.

## Development

```bash
make build      # build binary
make run        # build + run
make log        # tail -f /tmp/droid.log
```

## Architecture

- `term/` — raw terminal mode, key event parsing, ANSI sequences
- `tui/` — ELM-like loop: View(Model) → [][]byte, Render(prev, cur) → diff write on terminal

> [!NOTE]
> This project is still in very early stage, right now I'm working on my
> simple TUI system.
