package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeList parses `git worktree list --porcelain` output.
func WorktreeList(repoDir string) ([]WorktreeInfo, error) {
	out, err := run(repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeListPorcelain(out), nil
}

// WorktreeAdd creates a new worktree with a new branch.
func WorktreeAdd(repoDir, path, branch string) error {
	return runSilent(repoDir, "worktree", "add", path, "-b", branch, "--quiet")
}

// WorktreeAddFromStartPoint creates a new worktree with a new branch from startPoint.
func WorktreeAddFromStartPoint(repoDir, path, branch, startPoint string) error {
	if err := runSilent(repoDir, "worktree", "add", path, "-b", branch, startPoint, "--quiet"); err != nil {
		return err
	}
	// autoSetupMerge 설정에 관계없이 올바른 upstream을 명시적으로 설정
	// (원격 브랜치가 아직 없어도 config 직접 설정은 가능)
	if err := runSilent(repoDir, "config", "branch."+branch+".remote", "origin"); err != nil {
		return err
	}
	return runSilent(repoDir, "config", "branch."+branch+".merge", "refs/heads/"+branch)
}

// WorktreeAddExisting creates a worktree using an existing branch.
func WorktreeAddExisting(repoDir, path, branch string) error {
	return runSilent(repoDir, "worktree", "add", path, branch, "--quiet")
}

// WorktreeRemove removes a worktree.
func WorktreeRemove(repoDir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	return runSilent(repoDir, args...)
}

// WorktreePrune cleans up stale worktree refs.
func WorktreePrune(repoDir string) error {
	return runSilent(repoDir, "worktree", "prune")
}

// WorktreeExists checks if a worktree exists at the given absolute path.
func WorktreeExists(repoDir, absPath string) (bool, error) {
	worktrees, err := WorktreeList(repoDir)
	if err != nil {
		return false, err
	}
	for _, wt := range worktrees {
		if wt.Path == absPath {
			return true, nil
		}
	}
	return false, nil
}

// SetupWorktreePath computes the worktree base and full path for an issue.
func SetupWorktreePath(repoDir, issueKey string) (basePath, fullPath string, err error) {
	root, err := GetRepoRoot(repoDir)
	if err != nil {
		return "", "", err
	}
	repoName := filepath.Base(root)
	parentDir := filepath.Dir(root)
	basePath = filepath.Join(parentDir, repoName+"-worktrees")
	fullPath = filepath.Join(basePath, issueKey)
	return basePath, fullPath, nil
}

// HasUncommittedChanges checks if there are uncommitted changes in the worktree.
func HasUncommittedChanges(worktreePath string) (unstaged int, untracked int, err error) {
	// Unstaged changes
	out, err := run(worktreePath, "diff", "--name-only")
	if err != nil {
		return 0, 0, err
	}
	if out != "" {
		unstaged = len(strings.Split(out, "\n"))
	}

	// Untracked files
	out, err = run(worktreePath, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return 0, 0, err
	}
	if out != "" {
		untracked = len(strings.Split(out, "\n"))
	}

	return unstaged, untracked, nil
}

// AddExcludePattern adds a pattern to .git/info/exclude if not already present.
func AddExcludePattern(repoDir, pattern string) error {
	commonDir, err := GetGitCommonDir(repoDir)
	if err != nil {
		return err
	}
	// Make absolute if relative
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repoDir, commonDir)
	}

	excludePath := filepath.Join(commonDir, "info", "exclude")

	// Check if pattern already exists
	out, _ := run("", "grep", "-qF", pattern, excludePath)
	_ = out
	cmd := exec.Command("grep", "-qF", pattern, excludePath)
	if cmd.Run() == nil {
		return nil // already present
	}

	// Append pattern
	f, err := openFileAppend(excludePath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, pattern)
	return err
}

// parseWorktreeListPorcelain parses the porcelain output of git worktree list.
func parseWorktreeListPorcelain(output string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	var current WorktreeInfo

	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.Bare = true
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees
}
