package git

import (
	"fmt"
	"strings"
)

// BranchExists checks if a local branch exists.
func BranchExists(repoDir, name string) bool {
	return runSilent(repoDir, "show-ref", "--verify", "--quiet",
		fmt.Sprintf("refs/heads/%s", name)) == nil
}

// BranchExistsRemote checks if a remote branch exists.
func BranchExistsRemote(repoDir, remote, name string) bool {
	return runSilent(repoDir, "show-ref", "--verify", "--quiet",
		fmt.Sprintf("refs/remotes/%s/%s", remote, name)) == nil
}

// BranchExistsAny checks if a branch exists locally or on the remote.
func BranchExistsAny(repoDir, remote, name string) bool {
	return BranchExists(repoDir, name) || BranchExistsRemote(repoDir, remote, name)
}

// DeduplicateBranchName appends -1, -2, etc. if the branch name already exists.
func DeduplicateBranchName(repoDir, remote, baseName string) string {
	name := baseName
	counter := 1
	for BranchExistsAny(repoDir, remote, name) {
		name = fmt.Sprintf("%s-%d", baseName, counter)
		counter++
	}
	return name
}

// CreateBranch creates a new local branch from startPoint.
func CreateBranch(repoDir, name, startPoint string) error {
	return runSilent(repoDir, "branch", name, startPoint, "--quiet")
}

// DeleteBranch deletes a local branch.
func DeleteBranch(repoDir, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return runSilent(repoDir, "branch", flag, name)
}

// Checkout switches to a branch.
func Checkout(repoDir, branch string) error {
	return runSilent(repoDir, "checkout", branch, "--quiet")
}

// CheckoutNewBranch creates and switches to a new branch from startPoint.
func CheckoutNewBranch(repoDir, branch, startPoint string) error {
	return runSilent(repoDir, "checkout", "-b", branch, startPoint, "--quiet")
}

// Pull pulls from the remote.
func Pull(repoDir, remote, branch string) error {
	return runSilent(repoDir, "pull", remote, branch, "--quiet")
}

// ListRemoteBranches lists remote branches matching a pattern.
func ListRemoteBranches(repoDir, pattern string) ([]string, error) {
	out, err := run(repoDir, "branch", "-r")
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, pattern) {
			matches = append(matches, line)
		}
	}
	return matches, nil
}

// ListAllBranches returns all branch names (local + remote) with the remote prefix
// stripped and duplicates removed. HEAD pointers are skipped. Order is stable:
// remote-tracking branches first (alphabetical), then local-only branches.
func ListAllBranches(repoDir, remote string) ([]string, error) {
	out, err := run(repoDir, "branch", "-a", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var remoteNames, localNames []string
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.Contains(name, "->") || strings.HasSuffix(name, "/HEAD") {
			continue
		}
		if remote != "" && strings.HasPrefix(name, remote+"/") {
			name = strings.TrimPrefix(name, remote+"/")
			if !seen[name] {
				seen[name] = true
				remoteNames = append(remoteNames, name)
			}
			continue
		}
		// local branch: defer so remote-tracking ones rank first
		localNames = append(localNames, name)
	}
	result := append([]string{}, remoteNames...)
	for _, name := range localNames {
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result, nil
}

// ListBranchesByDate returns all branch names (local + remote) sorted by most
// recent commit date first, with the remote prefix stripped, duplicates removed,
// and HEAD pointers skipped.
func ListBranchesByDate(repoDir, remote string) ([]string, error) {
	out, err := run(repoDir, "for-each-ref", "--sort=-committerdate",
		"--format=%(refname:short)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var result []string
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.Contains(name, "->") || strings.HasSuffix(name, "/HEAD") {
			continue
		}
		if remote != "" && strings.HasPrefix(name, remote+"/") {
			name = strings.TrimPrefix(name, remote+"/")
		}
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result, nil
}

// ListTagsByDate returns all tags sorted by most recent creation date first.
func ListTagsByDate(repoDir string) ([]string, error) {
	out, err := run(repoDir, "for-each-ref", "--sort=-creatordate",
		"--format=%(refname:short)", "refs/tags")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, line := range strings.Split(out, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			result = append(result, name)
		}
	}
	return result, nil
}

// GetTags returns all tags.
func GetTags(repoDir string) ([]string, error) {
	out, err := run(repoDir, "tag")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// TagExists checks if a tag exists.
func TagExists(repoDir, tag string) bool {
	return runSilent(repoDir, "rev-parse", tag) == nil
}
