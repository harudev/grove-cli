package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const refSelectorWindow = 10 // max visible rows

type refSelectorModel struct {
	title    string
	all      []string // pre-sorted (newest first)
	filtered []string
	query    string
	cursor   int
	offset   int
	selected string
	done     bool
	aborted  bool
}

func newRefSelectorModel(title string, refs []string) refSelectorModel {
	return refSelectorModel{
		title:    title,
		all:      refs,
		filtered: refs,
	}
}

func (m *refSelectorModel) refilter() {
	q := strings.ToLower(strings.TrimSpace(m.query))
	if q == "" {
		m.filtered = m.all
	} else {
		m.filtered = m.filtered[:0]
		for _, r := range m.all {
			if strings.Contains(strings.ToLower(r), q) {
				m.filtered = append(m.filtered, r)
			}
		}
	}
	m.cursor = 0
	m.offset = 0
}

func (m refSelectorModel) Init() tea.Cmd { return nil }

func (m refSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.aborted = true
		return m, tea.Quit
	case tea.KeyEnter:
		if len(m.filtered) > 0 {
			m.selected = m.filtered[m.cursor]
			m.done = true
		}
		return m, tea.Quit
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
		return m, nil
	case tea.KeyDown:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			if m.cursor >= m.offset+refSelectorWindow {
				m.offset = m.cursor - refSelectorWindow + 1
			}
		}
		return m, nil
	case tea.KeyBackspace:
		if len(m.query) > 0 {
			r := []rune(m.query)
			m.query = string(r[:len(r)-1])
			m.refilter()
		}
		return m, nil
	case tea.KeyRunes, tea.KeySpace:
		m.query += string(key.Runes)
		m.refilter()
		return m, nil
	}
	return m, nil
}

func (m refSelectorModel) View() string {
	s := headerStyle.Render(m.title) + "\n"
	dim := normalStyle.Foreground(lipgloss.Color("8"))
	s += dim.Render("검색어 입력 → 필터, ↑↓ 이동, Enter 선택, Esc 취소") + "\n\n"
	s += "🔎 " + selectedStyle.Render(m.query) + dim.Render("▏") + "\n\n"

	if len(m.filtered) == 0 {
		s += dim.Render("  일치하는 항목 없음") + "\n"
		return s
	}

	end := m.offset + refSelectorWindow
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	for i := m.offset; i < end; i++ {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
			style = selectedStyle
		}
		s += fmt.Sprintf("%s%s\n", cursor, style.Render(m.filtered[i]))
	}
	s += "\n" + dim.Render(fmt.Sprintf("  %d/%d", m.cursor+1, len(m.filtered)))
	return s
}

// RunRefSelector runs a searchable selector over refs (already sorted newest-first)
// and returns the chosen ref.
func RunRefSelector(title string, refs []string) (string, error) {
	if len(refs) == 0 {
		return "", fmt.Errorf("선택할 수 있는 항목이 없습니다")
	}
	m := newRefSelectorModel(title, refs)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("TUI error: %w", err)
	}
	final := result.(refSelectorModel)
	if final.aborted || !final.done {
		return "", fmt.Errorf("선택이 취소되었습니다")
	}
	return final.selected, nil
}
