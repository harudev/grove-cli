package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// IssueContextMode represents how to handle Jira issue context.
type IssueContextMode int

const (
	// IssueContextFull saves title + description as context.
	IssueContextFull IssueContextMode = iota
	// IssueContextPlanning saves title and initialises a local planning file.
	IssueContextPlanning
	// IssueContextPlanningTDD saves title and initialises a local planning file with TDD workflow.
	IssueContextPlanningTDD
	// IssueContextPlanningPostTC saves title and initialises a planning file; TC is written after implementation.
	IssueContextPlanningPostTC
	// IssueContextSkip does not link the Jira issue.
	IssueContextSkip
)

type issueContextModeInfo struct {
	Mode        IssueContextMode
	Description string
	Hint        string
}

func allIssueContextModes() []issueContextModeInfo {
	return []issueContextModeInfo{
		{IssueContextFull, "이슈 컨텍스트 전체 저장", "이슈에 작업 내용이 상세히 기재되어 있을 때"},
		{IssueContextPlanning, "CLAUDE와 함께 기획", "이슈가 생성만 되어 있을 때"},
		{IssueContextSkip, "워크트리만 생성", "JIRA 연동 없이 바로 시작"},
	}
}

type issueContextModeModel struct {
	issueKey string
	items    []issueContextModeInfo
	cursor   int
	selected IssueContextMode
	done     bool
	aborted  bool
}

func newIssueContextModeModel(issueKey string) issueContextModeModel {
	return issueContextModeModel{
		issueKey: issueKey,
		items:    allIssueContextModes(),
	}
}

func (m issueContextModeModel) Init() tea.Cmd { return nil }

func (m issueContextModeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m issueContextModeModel) View() string {
	s := headerStyle.Render(fmt.Sprintf("Jira 이슈 컨텍스트 저장 방식을 선택하세요 (%s):", m.issueKey)) + "\n\n"

	hintStyle := normalStyle.Foreground(lipgloss.Color("8"))
	for i, item := range m.items {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
			style = selectedStyle
		}
		line := style.Render(item.Description)
		if item.Hint != "" {
			line += hintStyle.Render(" — " + item.Hint)
		}
		s += fmt.Sprintf("%s%s\n", cursor, line)
	}

	s += "\n" + normalStyle.Foreground(lipgloss.Color("8")).Render("↑↓: 이동, Enter: 선택, q: 취소")
	return s
}

// RunIssueContextModeSelector runs the interactive issue context mode selection TUI.
func RunIssueContextModeSelector(issueKey string) (IssueContextMode, error) {
	m := newIssueContextModeModel(issueKey)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return IssueContextSkip, fmt.Errorf("TUI error: %w", err)
	}

	final := result.(issueContextModeModel)
	if final.aborted {
		return IssueContextSkip, nil
	}
	return final.selected, nil
}
