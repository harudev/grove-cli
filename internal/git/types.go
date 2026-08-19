package git

// WorktreeInfo holds information about a single git worktree.
type WorktreeInfo struct {
	Path   string // absolute path
	Branch string // branch name (without refs/heads/)
	HEAD   string // HEAD commit hash
	Bare   bool
}
