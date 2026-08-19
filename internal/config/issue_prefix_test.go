package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIssuePrefixPerRepoIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate global config

	repoA := t.TempDir()
	repoB := t.TempDir()

	// Setting a prefix in one repo must not leak into another.
	if err := SetIssuePrefixForRepo(repoA, "proj"); err != nil {
		t.Fatalf("SetIssuePrefixForRepo(A): %v", err)
	}
	if got := GetIssuePrefixForRepo(repoA); got != "PROJ" {
		t.Fatalf("repoA prefix = %q, want PROJ (uppercased)", got)
	}
	if got := GetIssuePrefixForRepo(repoB); got == "PROJ" {
		t.Fatalf("repoB unexpectedly inherited repoA prefix %q", got)
	}

	if err := SetIssuePrefixForRepo(repoB, "PROJ"); err != nil {
		t.Fatalf("SetIssuePrefixForRepo(B): %v", err)
	}
	if got := GetIssuePrefixForRepo(repoA); got != "PROJ" {
		t.Fatalf("repoA prefix changed to %q after setting repoB", got)
	}
	if got := GetIssuePrefixForRepo(repoB); got != "PROJ" {
		t.Fatalf("repoB prefix = %q, want PROJ", got)
	}
}

func TestSetIssuePrefixIsReadOnlyAcrossReads(t *testing.T) {
	repo := t.TempDir()

	// Reading an unconfigured repo must not write anything to its config.
	_ = GetIssuePrefixForRepo(repo)
	if LoadRepoConfig(repo).IssuePrefix != "" {
		t.Fatal("reading prefix wrote into repo config")
	}

	// Setting must not clobber other repo-config fields.
	if err := AddPostCheckoutCommand(repo, "echo hi"); err != nil {
		t.Fatalf("AddPostCheckoutCommand: %v", err)
	}
	if err := SetIssuePrefixForRepo(repo, "ABC"); err != nil {
		t.Fatalf("SetIssuePrefixForRepo: %v", err)
	}
	rc := LoadRepoConfig(repo)
	if rc.IssuePrefix != "ABC" {
		t.Fatalf("prefix = %q, want ABC", rc.IssuePrefix)
	}
	if len(rc.PostCheckoutCommands) != 1 || rc.PostCheckoutCommands[0] != "echo hi" {
		t.Fatalf("postCheckoutCommands clobbered: %+v", rc.PostCheckoutCommands)
	}
}

func TestSetIssuePrefixRequiresRepo(t *testing.T) {
	if err := SetIssuePrefixForRepo("", "ABC"); err == nil {
		t.Fatal("SetIssuePrefixForRepo(\"\") should error")
	}
}

func TestLocalFilePatternsPerRepo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()

	if err := AddLocalFilePattern(repo, ".env.local"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := AddLocalFilePattern(repo, ".env.local"); err != nil { // dedupe
		t.Fatalf("Add dup: %v", err)
	}
	if got := GetLocalFilePatterns(repo); len(got) != 1 || got[0] != ".env.local" {
		t.Fatalf("patterns = %v, want [.env.local]", got)
	}
	if err := RemoveLocalFilePattern(repo, ".env.local"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := GetLocalFilePatterns(repo); len(got) != 0 {
		t.Fatalf("patterns after remove = %v, want empty", got)
	}
}

// writeGlobal writes a global ~/.config/grove/config.json under the test HOME.
func writeGlobal(t *testing.T, body string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", dir)
	cfgDir := filepath.Join(dir, ".config", "grove")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write global: %v", err)
	}
}

func TestMigrateGlobalToRepo(t *testing.T) {
	writeGlobal(t, `{"issuePrefix":"PROJ","localFilePatterns":[".env.local"],"jiraServer":"https://x"}`)

	repo := t.TempDir()
	// Repo must already be grove-managed for migration to run.
	if err := SaveRepoConfig(repo, &RepoConfig{PostCheckoutCommands: []string{"echo hi"}}); err != nil {
		t.Fatalf("seed repo: %v", err)
	}

	MigrateGlobalToRepo(repo)

	rc := LoadRepoConfig(repo)
	if rc.IssuePrefix != "PROJ" {
		t.Fatalf("migrated prefix = %q, want PROJ", rc.IssuePrefix)
	}
	if len(rc.LocalFilePatterns) != 1 || rc.LocalFilePatterns[0] != ".env.local" {
		t.Fatalf("migrated patterns = %v", rc.LocalFilePatterns)
	}
	if len(rc.PostCheckoutCommands) != 1 {
		t.Fatalf("existing repo fields clobbered: %v", rc.PostCheckoutCommands)
	}
	// Jira stays global, not migrated into the repo.
	var repoRaw map[string]any
	data, _ := os.ReadFile(repoConfigPath(repo))
	json.Unmarshal(data, &repoRaw)
	if _, ok := repoRaw["jiraServer"]; ok {
		t.Fatal("jiraServer should not be migrated into repo config")
	}

	// Global file: legacy keys stripped, Jira preserved.
	var globalRaw map[string]any
	gdata, _ := os.ReadFile(configPath())
	json.Unmarshal(gdata, &globalRaw)
	if _, ok := globalRaw["issuePrefix"]; ok {
		t.Fatal("issuePrefix should be removed from global config after migration")
	}
	if _, ok := globalRaw["localFilePatterns"]; ok {
		t.Fatal("localFilePatterns should be removed from global config after migration")
	}
	if globalRaw["jiraServer"] != "https://x" {
		t.Fatalf("jiraServer lost from global config: %v", globalRaw["jiraServer"])
	}
}

func TestMigrateSkipsUnmanagedRepo(t *testing.T) {
	writeGlobal(t, `{"issuePrefix":"PROJ"}`)

	repo := t.TempDir() // no .grove/config.json → not grove-managed
	MigrateGlobalToRepo(repo)

	if repoConfigExists(repo) {
		t.Fatal("migration created config for an unmanaged repo (would leak prefix into new repos)")
	}
}

func TestMigrateDoesNotOverrideRepoPrefix(t *testing.T) {
	writeGlobal(t, `{"issuePrefix":"PROJ"}`)

	repo := t.TempDir()
	if err := SetIssuePrefixForRepo(repo, "PROJ"); err != nil {
		t.Fatalf("set: %v", err)
	}
	MigrateGlobalToRepo(repo)

	if got := GetIssuePrefixForRepo(repo); got != "PROJ" {
		t.Fatalf("repo prefix overwritten by migration: %q", got)
	}
}
