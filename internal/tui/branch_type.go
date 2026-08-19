package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	normalStyle   = lipgloss.NewStyle()
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	headerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
)

type branchTypeModel struct {
	items    []BranchTypeInfo
	cursor   int
	selected BranchType
	done     bool
	aborted  bool
}

func newBranchTypeModel(items []BranchTypeInfo) branchTypeModel {
	return branchTypeModel{
		items: items,
	}
}

func (m branchTypeModel) Init() tea.Cmd {
	return nil
}

func (m branchTypeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.selected = m.items[m.cursor].Type
			m.done = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m branchTypeModel) View() string {
	s := headerStyle.Render("브랜치 타입을 선택하세요:") + "\n\n"

	for i, item := range m.items {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
			style = selectedStyle
		}
		s += fmt.Sprintf("%s%s  %s\n",
			cursor,
			style.Render(string(item.Type)),
			normalStyle.Foreground(lipgloss.Color("8")).Render("— "+item.Description))
	}

	s += "\n" + normalStyle.Foreground(lipgloss.Color("8")).Render("↑↓: 이동, Enter: 선택, q: 취소")
	return s
}

// RunBranchTypeSelector runs the interactive branch type selection TUI.
func RunBranchTypeSelector(items []BranchTypeInfo) (BranchType, error) {
	m := newBranchTypeModel(items)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("TUI error: %w", err)
	}

	final := result.(branchTypeModel)
	if final.aborted {
		return "", fmt.Errorf("브랜치 타입을 선택하지 않았습니다")
	}
	return final.selected, nil
}
