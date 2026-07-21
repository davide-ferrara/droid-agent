// Package tui for droid
package tui

import (
	"bufio"
	"os"
)

func Run() {
	reader := bufio.NewReader(os.Stdin)
	model := NewModel()
	view := NewView(&model)
	Render(view)
	HandleInput(reader, &model)
}
