package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/harudev/grove-cli/internal/fileutil"
	"github.com/harudev/grove-cli/internal/github"
	"github.com/harudev/grove-cli/internal/issue"
	"github.com/harudev/grove-cli/internal/jira"
)

// errJiraWorkflowNotConfigured is returned by Jira status-transition commands
// when no workflow (status/transition names) has been configured yet.
var errJiraWorkflowNotConfigured = errors.New(
	"Jira 워크플로우가 설정되지 않았습니다.\n" +
		"  상태/전이 이름을 매핑하려면: grove config jira-workflow --scaffold\n" +
		"  (전역: ~/.config/grove/config.json, 레포별: .grove/config.json)")

// errDeployFieldNotConfigured is returned by deploy-date commands when no
// deploy custom field id has been configured.
var errDeployFieldNotConfigured = errors.New(
	"배포일자 커스텀 필드가 설정되지 않았습니다.\n" +
		"  grove config jira-workflow --scaffold 후 deployField를 지정하세요 (예: customfield_11642)")

// resolveIssueKey resolves the issue key from arguments or current worktree.
func resolveIssueKey(args []string) (string, error) {
	if len(args) > 0 {
		parsed, err := issue.Parse(args[0])
		if err != nil {
			return "", err
		}
		if parsed.IssueKey != nil {
			return parsed.IssueKey.String(), nil
		}
		return args[0], nil
	}

	// Auto-detect from current worktree
	repoDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	issueKey, err := fileutil.ReadCurrentIssue(repoDir)
	if err == nil && issueKey != "" {
		return issueKey, nil
	}

	return "", fmt.Errorf("이슈키를 지정하거나 워크트리 내에서 실행하세요")
}

// requireJiraClient creates a Jira adapter or returns an error.
func requireJiraClient() (jira.Adapter, error) {
	client := newJiraAdapter()
	if client == nil {
		return nil, fmt.Errorf("Jira 어댑터를 사용할 수 없습니다. grove setup 또는 grove config jira를 실행하세요")
	}
	return client, nil
}

// requireGHClient creates a GitHub client or returns an error.
func requireGHClient() (*github.GHCLIClient, error) {
	client := github.NewGHCLIClient()
	if !client.Available() {
		return nil, fmt.Errorf("GitHub CLI(gh)가 설치되지 않았습니다")
	}
	return client, nil
}
