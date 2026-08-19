package git

import (
	"testing"
)

func TestParseWorktreeListPorcelain(t *testing.T) {
	input := `worktree /Users/user/repo
HEAD abc1234567890
branch refs/heads/main

worktree /Users/user/repo-worktrees/PROJ-12345
HEAD def4567890123
branch refs/heads/feature/PROJ-12345

worktree /Users/user/repo-worktrees/PROJ-67890
HEAD ghi7890123456
branch refs/heads/bugfix/PROJ-67890

`

	worktrees := parseWorktreeListPorcelain(input)

	if len(worktrees) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(worktrees))
	}

	tests := []struct {
		idx    int
		path   string
		branch string
		head   string
	}{
		{0, "/Users/user/repo", "main", "abc1234567890"},
		{1, "/Users/user/repo-worktrees/PROJ-12345", "feature/PROJ-12345", "def4567890123"},
		{2, "/Users/user/repo-worktrees/PROJ-67890", "bugfix/PROJ-67890", "ghi7890123456"},
	}

	for _, tt := range tests {
		wt := worktrees[tt.idx]
		if wt.Path != tt.path {
			t.Errorf("[%d] Path = %q, want %q", tt.idx, wt.Path, tt.path)
		}
		if wt.Branch != tt.branch {
			t.Errorf("[%d] Branch = %q, want %q", tt.idx, wt.Branch, tt.branch)
		}
		if wt.HEAD != tt.head {
			t.Errorf("[%d] HEAD = %q, want %q", tt.idx, wt.HEAD, tt.head)
		}
	}
}

func TestParseWorktreeListPorcelainEmpty(t *testing.T) {
	worktrees := parseWorktreeListPorcelain("")
	if len(worktrees) != 0 {
		t.Errorf("expected 0 worktrees for empty input, got %d", len(worktrees))
	}
}

func TestParseWorktreeListPorcelainBare(t *testing.T) {
	input := `worktree /Users/user/repo.git
HEAD abc1234567890
bare

`
	worktrees := parseWorktreeListPorcelain(input)
	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktrees))
	}
	if !worktrees[0].Bare {
		t.Error("expected Bare = true")
	}
	if worktrees[0].Branch != "" {
		t.Errorf("expected empty Branch for bare, got %q", worktrees[0].Branch)
	}
}

func TestDeduplicateBranchName(t *testing.T) {
	// This test uses a real git repo in a temp dir
	dir := t.TempDir()
	if err := runSilent(dir, "init", "--initial-branch=main"); err != nil {
		t.Skipf("git init failed: %v", err)
	}
	// Create an initial commit
	if err := runSilent(dir, "commit", "--allow-empty", "-m", "init"); err != nil {
		t.Skipf("git commit failed: %v", err)
	}

	// Create a branch "feature/PROJ-12345"
	if err := runSilent(dir, "branch", "feature/PROJ-12345"); err != nil {
		t.Fatalf("create branch: %v", err)
	}

	got := DeduplicateBranchName(dir, "origin", "feature/PROJ-12345")
	if got != "feature/PROJ-12345-1" {
		t.Errorf("DeduplicateBranchName = %q, want %q", got, "feature/PROJ-12345-1")
	}

	// Create -1 too
	if err := runSilent(dir, "branch", "feature/PROJ-12345-1"); err != nil {
		t.Fatalf("create branch -1: %v", err)
	}
	got = DeduplicateBranchName(dir, "origin", "feature/PROJ-12345")
	if got != "feature/PROJ-12345-2" {
		t.Errorf("DeduplicateBranchName = %q, want %q", got, "feature/PROJ-12345-2")
	}

	// Non-existing branch should return as-is
	got = DeduplicateBranchName(dir, "origin", "feature/PROJ-99999")
	if got != "feature/PROJ-99999" {
		t.Errorf("DeduplicateBranchName = %q, want %q", got, "feature/PROJ-99999")
	}
}
