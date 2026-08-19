package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/harudev/grove-cli/internal/tui"
)

var (
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	cyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	bold   = lipgloss.NewStyle().Bold(true)
)

func logInfo(format string, args ...any) {
	fmt.Fprintln(os.Stderr, cyan.Render(fmt.Sprintf(format, args...)))
}

func logSuccess(format string, args ...any) {
	fmt.Fprintln(os.Stderr, green.Render("✅ "+fmt.Sprintf(format, args...)))
}

func logWarn(format string, args ...any) {
	fmt.Fprintln(os.Stderr, yellow.Render("⚠️  "+fmt.Sprintf(format, args...)))
}

func logError(format string, args ...any) {
	fmt.Fprintln(os.Stderr, red.Render("❌ "+fmt.Sprintf(format, args...)))
}

func logBold(format string, args ...any) {
	fmt.Fprintln(os.Stderr, bold.Render(fmt.Sprintf(format, args...)))
}

// spinner displays an animated spinner to the right of a message until stop() is called.
func spinner(message string) func() {
	return tui.Spinner(message)
}

// runShell runs a shell command in the given directory.
func runShell(dir, command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
