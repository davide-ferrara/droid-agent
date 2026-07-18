# droid

Self-contained LLM agent with a minimal TUI built in Go.

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
