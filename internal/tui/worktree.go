package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type worktreeModel struct {
	items    []string
	cursor   int
	selected string
	done     bool
	aborted  bool
}

func newWorktreeModel(items []string) worktreeModel {
	return worktreeModel{items: items}
}

func (m worktreeModel) Init() tea.Cmd { return nil }

func (m worktreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.items[m.cursor]
			m.done = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m worktreeModel) View() string {
	s := headerStyle.Render("정리할 워크트리를 선택하세요:") + "\n\n"

	for i, item := range m.items {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
			style = selectedStyle
		}
		s += cursor + style.Render(item) + "\n"
	}

	s += "\n" + normalStyle.Foreground(lipgloss.Color("8")).Render("↑↓: 이동, Enter: 선택, q: 취소")
	return s
}

// RunWorktreeSelector runs the interactive worktree selection TUI.
func RunWorktreeSelector(branches []string) (string, error) {
	if len(branches) == 0 {
		return "", fmt.Errorf("워크트리를 찾을 수 없습니다")
	}

	m := newWorktreeModel(branches)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("TUI error: %w", err)
	}

	final := result.(worktreeModel)
	if final.aborted {
		return "", fmt.Errorf("워크트리를 선택하지 않았습니다")
	}
	return final.selected, nil
}
