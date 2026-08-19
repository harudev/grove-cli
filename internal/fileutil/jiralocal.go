package fileutil

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const jiraLocalDir = ".claude/jira-local"

// SaveCurrentIssue writes the issue key to .claude/jira-local/current-issue.
func SaveCurrentIssue(worktreePath, issueKey string) error {
	return writeFile(worktreePath, "current-issue", issueKey+"\n")
}

// SaveBranchType writes the branch type to .claude/jira-local/branch-type.
func SaveBranchType(worktreePath, branchType string) error {
	return writeFile(worktreePath, "branch-type", branchType+"\n")
}

// SaveIssueTitle writes the issue title to .claude/jira-local/issue-title.
func SaveIssueTitle(worktreePath, title string) error {
	return writeFile(worktreePath, "issue-title", title+"\n")
}

// ReadCurrentIssue reads the issue key from .claude/jira-local/current-issue.
func ReadCurrentIssue(worktreePath string) (string, error) {
	return readFirstLine(worktreePath, "current-issue")
}

// ReadIssueTitle reads the issue title from .claude/jira-local/issue-title.
func ReadIssueTitle(worktreePath string) (string, error) {
	return readFirstLine(worktreePath, "issue-title")
}

// ReadPhase reads the phase from .claude/jira-local/handoff/{issueKey}.local.md.
func ReadPhase(worktreePath, issueKey string) (string, error) {
	handoffPath := filepath.Join(worktreePath, jiraLocalDir, "handoff", issueKey+".local.md")
	f, err := os.Open(handoffPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "phase:") {
			phase := strings.TrimSpace(strings.TrimPrefix(line, "phase:"))
			phase = strings.Trim(phase, "\"")
			return phase, nil
		}
	}
	return "", fmt.Errorf("phase not found in %s", handoffPath)
}

// CopyAIAgentTools copies AI/agent tool files from mainRepo to worktreePath.
func CopyAIAgentTools(mainRepo, worktreePath string, patterns []string) error {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(mainRepo, pattern))
		if err != nil {
			return fmt.Errorf("glob %s: %w", pattern, err)
		}
		for _, src := range matches {
			relPath, err := filepath.Rel(mainRepo, src)
			if err != nil {
				return err
			}
			dst := filepath.Join(worktreePath, relPath)
			if err := copyRecursive(src, dst); err != nil {
				return fmt.Errorf("copy %s: %w", relPath, err)
			}
		}
	}

	// Remove .claude/.claude/ nesting if present
	nested := filepath.Join(worktreePath, ".claude", ".claude")
	os.RemoveAll(nested)

	return nil
}

// CopyLocalFiles copies local (untracked) files matching patterns from mainRepo to worktreePath.
// Unlike CopyAIAgentTools, failures on individual patterns are warnings, not errors.
// Patterns may contain ** to match across directory boundaries (e.g. **/.env.local).
func CopyLocalFiles(mainRepo, worktreePath string, patterns []string) (copied []string, warnings []string) {
	for _, pattern := range patterns {
		var matches []string
		var err error
		if strings.Contains(pattern, "**") {
			matches, err = globDoublestar(mainRepo, pattern)
		} else if !strings.Contains(pattern, "/") {
			// bare filename: search recursively across all directories
			matches, err = globDoublestar(mainRepo, "**/"+pattern)
		} else {
			matches, err = filepath.Glob(filepath.Join(mainRepo, pattern))
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("glob %s: %v", pattern, err))
			continue
		}
		if len(matches) == 0 {
			continue
		}
		for _, src := range matches {
			relPath, err := filepath.Rel(mainRepo, src)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("rel %s: %v", src, err))
				continue
			}
			dst := filepath.Join(worktreePath, relPath)
			// Skip if destination already exists
			if _, err := os.Stat(dst); err == nil {
				continue
			}
			if err := copyRecursive(src, dst); err != nil {
				warnings = append(warnings, fmt.Sprintf("copy %s: %v", relPath, err))
				continue
			}
			copied = append(copied, relPath)
		}
	}
	return
}

// globDoublestar walks root and returns absolute paths where the relative path matches pattern.
// Supports ** to match zero or more path components.
func globDoublestar(root, pattern string) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil || relPath == "." {
			return nil
		}
		ok, matchErr := matchDoublestar(pattern, filepath.ToSlash(relPath))
		if matchErr != nil || !ok {
			return nil
		}
		matches = append(matches, path)
		return nil
	})
	return matches, err
}

// matchDoublestar reports whether path matches the glob pattern with ** support.
func matchDoublestar(pattern, path string) (bool, error) {
	return matchParts(
		strings.Split(filepath.ToSlash(pattern), "/"),
		strings.Split(filepath.ToSlash(path), "/"),
	)
}

func matchParts(patParts, pathParts []string) (bool, error) {
	for i, p := range patParts {
		if p == "**" {
			rest := patParts[i+1:]
			for j := 0; j <= len(pathParts); j++ {
				if ok, _ := matchParts(rest, pathParts[j:]); ok {
					return true, nil
				}
			}
			return false, nil
		}
		if len(pathParts) == 0 {
			return false, nil
		}
		ok, err := filepath.Match(p, pathParts[0])
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		pathParts = pathParts[1:]
	}
	return len(pathParts) == 0, nil
}

// CopySyncTokens copies the sync-tokens.sh script to the worktree.
func CopySyncTokens(scriptDir, worktreePath string) error {
	src := filepath.Join(scriptDir, "sync-tokens.sh")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil // not an error if script doesn't exist
	}
	dst := filepath.Join(worktreePath, jiraLocalDir, "sync-tokens.sh")
	return copyFile(src, dst, 0o755)
}

// EnsureJiraLocalDir creates the .claude/jira-local directory if it doesn't exist.
func EnsureJiraLocalDir(worktreePath string) error {
	dir := filepath.Join(worktreePath, jiraLocalDir)
	return os.MkdirAll(dir, 0o755)
}

// --- helpers ---

func writeFile(worktreePath, filename, content string) error {
	dir := filepath.Join(worktreePath, jiraLocalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644)
}

func readFirstLine(worktreePath, filename string) (string, error) {
	data, err := os.ReadFile(filepath.Join(worktreePath, jiraLocalDir, filename))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func copyFile(src, dst string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, perm)
}

func copyRecursive(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return copyFile(src, dst, info.Mode())
	}

	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcChild := filepath.Join(src, entry.Name())
		dstChild := filepath.Join(dst, entry.Name())
		if err := copyRecursive(srcChild, dstChild); err != nil {
			return err
		}
	}

	return nil
}
