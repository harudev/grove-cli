package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DeployDateOption represents a selectable deploy date.
type DeployDateOption struct {
	Date  string // "YYYY-MM-DD"
	Label string // display label
}

// koreanWeekdays maps time.Weekday to its Korean single-character name.
var koreanWeekdays = [...]string{"일", "월", "화", "수", "목", "금", "토"}

// weekdayLabel returns e.g. "화요일" for the given weekday.
func weekdayLabel(wd time.Weekday) string {
	return koreanWeekdays[int(wd)] + "요일"
}

// NextWeekdays returns 3 upcoming occurrences of weekday starting from this
// week's occurrence. If today is past that weekday, starts from next week's.
func NextWeekdays(now time.Time, weekday time.Weekday) []DeployDateOption {
	daysUntil := (int(weekday) - int(now.Weekday()) + 7) % 7
	// daysUntil == 0 means today is the target weekday — include it

	first := now.AddDate(0, 0, daysUntil)
	second := first.AddDate(0, 0, 7)
	third := first.AddDate(0, 0, 14)

	label := weekdayLabel(weekday)
	return []DeployDateOption{
		{Date: first.Format("2006-01-02"), Label: fmt.Sprintf("이번주 %s (%s)", label, first.Format("01/02"))},
		{Date: second.Format("2006-01-02"), Label: fmt.Sprintf("다음주 %s (%s)", label, second.Format("01/02"))},
		{Date: third.Format("2006-01-02"), Label: fmt.Sprintf("다다음주 %s (%s)", label, third.Format("01/02"))},
	}
}

// WeekdaysFromDate returns 3 occurrences of weekday starting from the first one
// on or after fromDate.
func WeekdaysFromDate(fromDate time.Time, weekday time.Weekday) []DeployDateOption {
	daysUntil := (int(weekday) - int(fromDate.Weekday()) + 7) % 7

	first := fromDate.AddDate(0, 0, daysUntil)
	second := first.AddDate(0, 0, 7)
	third := first.AddDate(0, 0, 14)

	label := weekdayLabel(weekday)
	return []DeployDateOption{
		{Date: first.Format("2006-01-02"), Label: fmt.Sprintf("%s (%s)", label, first.Format("01/02"))},
		{Date: second.Format("2006-01-02"), Label: fmt.Sprintf("%s (%s)", label, second.Format("01/02"))},
		{Date: third.Format("2006-01-02"), Label: fmt.Sprintf("%s (%s)", label, third.Format("01/02"))},
	}
}

type deployDateModel struct {
	options  []DeployDateOption
	cursor   int
	selected string
	done     bool
	aborted  bool
}

func newDeployDateModel(options []DeployDateOption) deployDateModel {
	return deployDateModel{options: options}
}

func (m deployDateModel) Init() tea.Cmd { return nil }

func (m deployDateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.options[m.cursor].Date
			m.done = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m deployDateModel) View() string {
	s := headerStyle.Render("배포일자를 선택하세요:") + "\n\n"

	for i, opt := range m.options {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
			style = selectedStyle
		}
		s += fmt.Sprintf("%s%s\n", cursor, style.Render(opt.Label))
	}

	s += "\n" + normalStyle.Foreground(lipgloss.Color("8")).Render("↑↓: 이동, Enter: 선택, q: 취소")
	return s
}

// RunDeployDateSelector runs the interactive deploy date selection TUI.
func RunDeployDateSelector(options []DeployDateOption) (string, error) {
	m := newDeployDateModel(options)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("TUI error: %w", err)
	}

	final := result.(deployDateModel)
	if final.aborted {
		return "", fmt.Errorf("배포일자를 선택하지 않았습니다")
	}
	return final.selected, nil
}
