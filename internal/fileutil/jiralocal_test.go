package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndReadCurrentIssue(t *testing.T) {
	dir := t.TempDir()
	if err := SaveCurrentIssue(dir, "PROJ-12345"); err != nil {
		t.Fatalf("SaveCurrentIssue: %v", err)
	}
	got, err := ReadCurrentIssue(dir)
	if err != nil {
		t.Fatalf("ReadCurrentIssue: %v", err)
	}
	if got != "PROJ-12345" {
		t.Errorf("ReadCurrentIssue = %q, want %q", got, "PROJ-12345")
	}
}

func TestSaveAndReadBranchType(t *testing.T) {
	dir := t.TempDir()
	if err := SaveBranchType(dir, "feature"); err != nil {
		t.Fatalf("SaveBranchType: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".claude/jira-local/branch-type"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := string(data); got != "feature\n" {
		t.Errorf("branch-type content = %q, want %q", got, "feature\n")
	}
}

func TestSaveAndReadIssueTitle(t *testing.T) {
	dir := t.TempDir()
	if err := SaveIssueTitle(dir, "프린트허브 신규 기능"); err != nil {
		t.Fatalf("SaveIssueTitle: %v", err)
	}
	got, err := ReadIssueTitle(dir)
	if err != nil {
		t.Fatalf("ReadIssueTitle: %v", err)
	}
	if got != "프린트허브 신규 기능" {
		t.Errorf("ReadIssueTitle = %q, want %q", got, "프린트허브 신규 기능")
	}
}

func TestReadPhase(t *testing.T) {
	dir := t.TempDir()
	handoffDir := filepath.Join(dir, ".claude/jira-local/handoff")
	os.MkdirAll(handoffDir, 0o755)

	content := `---
issueKey: PROJ-12345
branch: feature/PROJ-12345
phase: dev
---
Some content here.
`
	os.WriteFile(filepath.Join(handoffDir, "PROJ-12345.local.md"), []byte(content), 0o644)

	got, err := ReadPhase(dir, "PROJ-12345")
	if err != nil {
		t.Fatalf("ReadPhase: %v", err)
	}
	if got != "dev" {
		t.Errorf("ReadPhase = %q, want %q", got, "dev")
	}
}

func TestReadPhaseNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadPhase(dir, "PROJ-99999")
	if err == nil {
		t.Error("ReadPhase expected error for missing file")
	}
}

func TestCopyAIAgentTools(t *testing.T) {
	mainRepo := t.TempDir()
	worktree := t.TempDir()

	// Create source files
	os.MkdirAll(filepath.Join(mainRepo, ".claude", "rules"), 0o755)
	os.WriteFile(filepath.Join(mainRepo, ".claude", "rules", "test.md"), []byte("rule"), 0o644)
	os.WriteFile(filepath.Join(mainRepo, "AGENTS.md"), []byte("agents"), 0o644)

	patterns := []string{".claude", "AGENTS.md"}
	if err := CopyAIAgentTools(mainRepo, worktree, patterns); err != nil {
		t.Fatalf("CopyAIAgentTools: %v", err)
	}

	// Verify copies
	data, err := os.ReadFile(filepath.Join(worktree, ".claude", "rules", "test.md"))
	if err != nil {
		t.Fatalf("rule file not copied: %v", err)
	}
	if string(data) != "rule" {
		t.Errorf("rule content = %q, want %q", string(data), "rule")
	}

	data, err = os.ReadFile(filepath.Join(worktree, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not copied: %v", err)
	}
	if string(data) != "agents" {
		t.Errorf("AGENTS.md content = %q, want %q", string(data), "agents")
	}
}
