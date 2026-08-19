package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type confirmModel struct {
	message  string
	cursor   int // 0=yes, 1=no
	result   bool
	done     bool
	aborted  bool
}

func newConfirmModel(message string) confirmModel {
	return confirmModel{message: message}
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h", "tab":
			m.cursor = 1 - m.cursor
		case "right", "l":
			m.cursor = 1 - m.cursor
		case "y", "Y":
			m.result = true
			m.done = true
			return m, tea.Quit
		case "n", "N":
			m.result = false
			m.done = true
			return m, tea.Quit
		case "enter":
			m.result = m.cursor == 0
			m.done = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	s := headerStyle.Render(m.message) + "\n\n"

	yesStyle := normalStyle
	noStyle := normalStyle
	if m.cursor == 0 {
		yesStyle = selectedStyle
	} else {
		noStyle = selectedStyle
	}

	s += "  " + yesStyle.Render("[Yes]") + "  " + noStyle.Render("[No]")
	s += "\n\n" + normalStyle.Foreground(lipgloss.Color("8")).Render("←→: 이동, Enter: 확인, y/n: 직접 선택")
	return s
}

// RunConfirm runs an interactive yes/no confirmation TUI.
func RunConfirm(message string) (bool, error) {
	m := newConfirmModel(message)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("TUI error: %w", err)
	}

	final := result.(confirmModel)
	if final.aborted {
		return false, nil
	}
	return final.result, nil
}
