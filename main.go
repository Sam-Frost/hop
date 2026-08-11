package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := LoadConfig()
	mustExit(err)

	p := tea.NewProgram(initialModel(cfg))
	_, err = p.Run()
	mustExit(err)
}
