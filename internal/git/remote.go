package git

import (
	"fmt"
	"strings"

	"github.com/harudev/grove-cli/internal/config"
)

// DetectRemote returns the preferred remote name (origin first, then first available).
func DetectRemote(repoDir string) (string, error) {
	out, err := run(repoDir, "remote")
	if err != nil {
		return "", fmt.Errorf("detect remote: %w", err)
	}
	remotes := strings.Split(out, "\n")
	for _, r := range remotes {
		if strings.TrimSpace(r) == "origin" {
			return "origin", nil
		}
	}
	if len(remotes) > 0 && strings.TrimSpace(remotes[0]) != "" {
		return strings.TrimSpace(remotes[0]), nil
	}
	return "", fmt.Errorf("no git remote configured")
}

// DetectDefaultBranch detects the default branch (mainline > main > fallback).
func DetectDefaultBranch(repoDir, remote string) (string, error) {
	for _, candidate := range config.DefaultBranchCandidates {
		// Check remote ref
		if err := runSilent(repoDir, "show-ref", "--verify", "--quiet",
			fmt.Sprintf("refs/remotes/%s/%s", remote, candidate)); err == nil {
			return candidate, nil
		}
		// Check local ref
		if err := runSilent(repoDir, "show-ref", "--verify", "--quiet",
			fmt.Sprintf("refs/heads/%s", candidate)); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot detect default branch (tried: %s)", strings.Join(config.DefaultBranchCandidates, ", "))
}

// Fetch runs git fetch for the given remote.
func Fetch(repoDir, remote string, tags bool) error {
	args := []string{"fetch"}
	if tags {
		args = append(args, "--tags", "-f")
	}
	if remote != "" {
		args = append(args, remote)
	} else {
		args = append(args, "--all")
	}
	args = append(args, "--quiet")
	return runSilent(repoDir, args...)
}

// FetchAll runs git fetch --all --quiet.
func FetchAll(repoDir string) error {
	return runSilent(repoDir, "fetch", "--all", "--quiet")
}

// Push pushes a branch to the remote.
func Push(repoDir, remote, branch string) error {
	return runSilent(repoDir, "push", "-u", remote, branch, "--quiet")
}

// GetCurrentBranch returns the current branch name.
func GetCurrentBranch(repoDir string) (string, error) {
	return run(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
}

// GetRepoRoot returns the top-level directory of the git repository.
func GetRepoRoot(repoDir string) (string, error) {
	return run(repoDir, "rev-parse", "--show-toplevel")
}

// GetGitCommonDir returns the git common directory (shared across worktrees).
func GetGitCommonDir(repoDir string) (string, error) {
	return run(repoDir, "rev-parse", "--git-common-dir")
}
