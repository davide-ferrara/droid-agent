//go:build !debug

package tui

// In release builds the debug helpers compile to no-ops so log
// spam never ships and the `log` import is not pulled in. To
// build with debug logging on: go build -tags debug.

func dbgInput(in *Input) {}

func dbgModel(m *Model, cursorRow, cursorCol int) {}