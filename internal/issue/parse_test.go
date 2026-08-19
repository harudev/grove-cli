package issue

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestConfig creates an isolated temp git repo, switches into it, and writes
// its per-repo .grove/config.json with the given issue prefix. The issue prefix
// is resolved per-repo from the current working directory's repo, so tests must
// run inside a real repo rather than relying on global config.
func setupTestConfig(t *testing.T, prefix string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // isolate any global config

	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}
	t.Chdir(repo)

	groveDir := filepath.Join(repo, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatalf("mkdir .grove: %v", err)
	}
	if err := os.WriteFile(filepath.Join(groveDir, "config.json"),
		[]byte(`{"issuePrefix":"`+prefix+`"}`), 0o644); err != nil {
		t.Fatalf("write repo config: %v", err)
	}
}

func TestParse(t *testing.T) {
	// Set default prefix for tests
	setupTestConfig(t, "PROJ")

	tests := []struct {
		name      string
		input     string
		wantType  InputType
		wantKey   string
		wantPRURL string
		wantErr   bool
	}{
		{
			name:     "full issue key",
			input:    "PROJ-12345",
			wantType: InputIssueKey,
			wantKey:  "PROJ-12345",
		},
		{
			name:     "lowercase issue key (with prefix configured)",
			input:    "proj-12345",
			wantType: InputIssueKey,
			wantKey:  "PROJ-12345",
		},
		{
			name:     "numeric only (uses default prefix)",
			input:    "12345",
			wantType: InputNumeric,
			wantKey:  "PROJ-12345",
		},
		{
			name:     "jira URL",
			input:    "https://example.atlassian.net/browse/PROJ-28451",
			wantType: InputJiraURL,
			wantKey:  "PROJ-28451",
		},
		{
			name:      "PR URL without issue key",
			input:     "https://github.com/org/repo/pull/456",
			wantType:  InputPRURL,
			wantPRURL: "https://github.com/org/repo/pull/456",
		},
		{
			name:      "PR URL with issue key in path",
			input:     "https://github.com/org/repo/pull/456?head=feature/PROJ-12345",
			wantType:  InputPRURL,
			wantKey:   "PROJ-12345",
			wantPRURL: "https://github.com/org/repo/pull/456?head=feature/PROJ-12345",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid input",
			input:   "not-a-valid-input",
			wantErr: true,
		},
		{
			name:     "input with whitespace",
			input:    "  PROJ-99999  ",
			wantType: InputIssueKey,
			wantKey:  "PROJ-99999",
		},
		{
			name:     "different prefix full key",
			input:    "PROJ-999",
			wantType: InputIssueKey,
			wantKey:  "PROJ-999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}
			if got.Type != tt.wantType {
				t.Errorf("Parse(%q).Type = %v, want %v", tt.input, got.Type, tt.wantType)
			}
			if tt.wantKey != "" {
				if got.IssueKey == nil {
					t.Fatalf("Parse(%q).IssueKey is nil, want %q", tt.input, tt.wantKey)
				}
				if got.IssueKey.String() != tt.wantKey {
					t.Errorf("Parse(%q).IssueKey = %q, want %q", tt.input, got.IssueKey.String(), tt.wantKey)
				}
			}
			if tt.wantPRURL != "" && got.PRURL != tt.wantPRURL {
				t.Errorf("Parse(%q).PRURL = %q, want %q", tt.input, got.PRURL, tt.wantPRURL)
			}
		})
	}
}

func TestParseNumericWithoutPrefix(t *testing.T) {
	// No prefix configured → numeric input should fail
	setupTestConfig(t, "")

	_, err := Parse("12345")
	if err == nil {
		t.Error("Parse(\"12345\") with no prefix should return error")
	}
}
