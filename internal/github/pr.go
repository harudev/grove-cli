package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// PRInfo holds PR metadata.
type PRInfo struct {
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
}

// PRState represents the state of a pull request found by branch.
type PRState struct {
	Number int      `json:"number"`
	State  string   `json:"state"`   // OPEN, MERGED, CLOSED
	Labels []string `json:"-"`       // parsed from raw
	URL    string   `json:"url"`
}

// PRClient defines the interface for GitHub PR operations.
type PRClient interface {
	View(url string) (*PRInfo, error)
	CreateDraft(head, base, title, body string) error
	FindPRByBranch(repoDir, branch string) (*PRState, error)
}

// GHCLIClient wraps the gh CLI.
type GHCLIClient struct{}

// NewGHCLIClient creates a new GHCLIClient.
func NewGHCLIClient() *GHCLIClient {
	return &GHCLIClient{}
}

// Available checks if gh CLI is installed.
func (c *GHCLIClient) Available() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// View retrieves PR info from a URL.
func (c *GHCLIClient) View(url string) (*PRInfo, error) {
	out, err := runGH("pr", "view", url, "--json", "headRefName,baseRefName")
	if err != nil {
		return nil, fmt.Errorf("gh pr view %s: %w", url, err)
	}
	var info PRInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return nil, fmt.Errorf("parse pr info: %w", err)
	}
	return &info, nil
}

// CreateDraft creates a draft PR.
func (c *GHCLIClient) CreateDraft(head, base, title, body string) error {
	_, err := runGH("pr", "create",
		"--draft",
		"--title", title,
		"--body", body,
		"--head", head,
		"--base", base)
	return err
}

// FindPRByBranch finds the most recent PR for a given head branch.
func (c *GHCLIClient) FindPRByBranch(repoDir, branch string) (*PRState, error) {
	out, err := runGHInDir(repoDir, "pr", "list",
		"--head", branch,
		"--state", "all",
		"--limit", "1",
		"--json", "number,state,labels,url")
	if err != nil {
		return nil, fmt.Errorf("gh pr list --head %s: %w", branch, err)
	}

	var prs []struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, fmt.Errorf("parse pr list: %w", err)
	}

	if len(prs) == 0 {
		return nil, nil // no PR found
	}

	pr := prs[0]
	state := &PRState{
		Number: pr.Number,
		State:  pr.State,
		URL:    pr.URL,
	}
	for _, l := range pr.Labels {
		state.Labels = append(state.Labels, l.Name)
	}
	return state, nil
}

// ErrNoPRFound indicates no merged PR matched the search query.
var ErrNoPRFound = fmt.Errorf("머지된 PR을 찾을 수 없습니다")

// FindLastMergedPRDate searches for merged PRs matching the query and returns the merge date of the most recent one.
// Returns ErrNoPRFound when the query succeeds but no PRs match.
// Returns a wrapped error for API/network failures.
func (c *GHCLIClient) FindLastMergedPRDate(repoDir, query string) (time.Time, error) {
	out, err := runGHInDir(repoDir, "pr", "list",
		"--search", query+" sort:updated-desc",
		"--state", "merged",
		"--limit", "10",
		"--json", "mergedAt")
	if err != nil {
		return time.Time{}, fmt.Errorf("gh pr list --search %q: %w", query, err)
	}

	var prs []struct {
		MergedAt time.Time `json:"mergedAt"`
	}
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return time.Time{}, fmt.Errorf("parse pr list: %w", err)
	}

	if len(prs) == 0 {
		return time.Time{}, ErrNoPRFound
	}

	// find the latest mergedAt among all results
	latest := prs[0].MergedAt
	for _, pr := range prs[1:] {
		if pr.MergedAt.After(latest) {
			latest = pr.MergedAt
		}
	}

	return latest, nil
}

func runGHInDir(dir string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh %v: %w\n%s", args, err, stderr.String())
	}
	return stdout.String(), nil
}

func runGH(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh %v: %w\n%s", args, err, stderr.String())
	}
	return stdout.String(), nil
}
