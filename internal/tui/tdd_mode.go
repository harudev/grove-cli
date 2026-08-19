package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TDDMode represents the test case strategy when planning with CLAUDE.
type TDDMode int

const (
	// TDDModeTDD writes test cases first, then implements (TDD).
	TDDModeTDD TDDMode = iota
	// TDDModePostTC implements first, then writes test cases for verification.
	TDDModePostTC
	// TDDModeNone does not add test cases.
	TDDModeNone
)

type tddModeInfo struct {
	Mode        TDDMode
	Description string
}

func allTDDModes() []tddModeInfo {
	return []tddModeInfo{
		{TDDModeTDD, "TC 먼저 작성하고 TDD로 구현"},
		{TDDModePostTC, "구현 후 동작 확인용 TC 생성"},
		{TDDModeNone, "TC 없이 진행"},
	}
}

type tddModeModel struct {
	items   []tddModeInfo
	cursor  int
	selected TDDMode
	done    bool
	aborted bool
}

func newTDDModeModel() tddModeModel {
	return tddModeModel{
		items: allTDDModes(),
	}
}

func (m tddModeModel) Init() tea.Cmd { return nil }

func (m tddModeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.selected = m.items[m.cursor].Mode
			m.done = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m tddModeModel) View() string {
	s := headerStyle.Render("테스트 케이스 전략을 선택하세요:") + "\n\n"

	for i, item := range m.items {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
			style = selectedStyle
		}
		s += fmt.Sprintf("%s%s\n", cursor, style.Render(item.Description))
	}

	s += "\n" + normalStyle.Foreground(lipgloss.Color("8")).Render("↑↓: 이동, Enter: 선택, q: 취소")
	return s
}

// RunTDDModeSelector runs the interactive TDD mode selection TUI.
func RunTDDModeSelector() (TDDMode, error) {
	m := newTDDModeModel()
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return TDDModeNone, fmt.Errorf("TUI error: %w", err)
	}

	final := result.(tddModeModel)
	if final.aborted {
		return TDDModeNone, nil
	}
	return final.selected, nil
}
