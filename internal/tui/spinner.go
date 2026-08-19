package tui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var spinnerCyan = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

// Spinner displays an animated spinner to the right of a message until the returned stop function is called.
// Usage:
//
//	stop := tui.Spinner("📌 feature 브랜치 생성")
//	defer stop()
func Spinner(message string) func() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	var once sync.Once
	done := make(chan struct{})

	go func() {
		i := 0
		for {
			select {
			case <-done:
				fmt.Fprintf(os.Stderr, "\r\033[K")
				fmt.Fprintln(os.Stderr, spinnerCyan.Render(message))
				return
			default:
				fmt.Fprintf(os.Stderr, "\r%s %s", spinnerCyan.Render(message), spinnerCyan.Render(frames[i%len(frames)]))
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()

	return func() {
		once.Do(func() { close(done) })
		time.Sleep(100 * time.Millisecond)
	}
}
