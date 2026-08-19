package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// run executes a git command in the given directory and returns stdout.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// runSilent executes a git command and ignores stdout, returning only errors.
func runSilent(dir string, args ...string) error {
	_, err := run(dir, args...)
	return err
}
