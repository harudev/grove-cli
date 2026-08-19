package iterm

import (
	"bytes"
	"fmt"
	"os/exec"
)

// OpenTab opens a new iTerm2 tab and runs a command in it.
func OpenTab(worktreePath, command string) error {
	script := fmt.Sprintf(`tell application "iTerm"
  tell current window
    create tab with default profile
    tell current session
      write text "cd '%s' && %s"
    end tell
  end tell
end tell`, worktreePath, command)

	cmd := exec.Command("osascript", "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript: %w\n%s", err, stderr.String())
	}
	return nil
}
