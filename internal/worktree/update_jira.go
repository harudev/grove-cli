package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/git"
	"github.com/harudev/grove-cli/internal/github"
	"github.com/harudev/grove-cli/internal/jira"
)

// UpdateJiraOptions holds options for the update-jira flow.
type UpdateJiraOptions struct {
	IssueKey      string
	DryRun        bool
	All           bool // process all worktrees
	AllowNoBranch bool // skip branch lookup failure (PR=nil, worktree=false)
}

// UpdateJiraResult holds the result of an update-jira operation.
type UpdateJiraResult struct {
	IssueKey      string
	IssueSummary  string
	CurrentStatus string
	PRState       string // OPEN, MERGED, CLOSED, or "" (no PR)
	PRNumber      int
	PRURL         string
	Action        string // description of what was/would be done
	Transitioned  bool
	Skipped       bool
	SkipReason    string
	// PendingChildren holds non-terminal child issues when this issue was transitioned to 해결종료.
	PendingChildren []jira.Issue
	// ParentKey is the key of the parent issue if it exists.
	ParentKey string
	// ParentResolved indicates if the parent issue is already in a terminal/resolved state.
	ParentResolved bool
}

// UpdateJiraAll runs UpdateJira for all worktrees that have an associated issue key.
func UpdateJiraAll(repoDir string, jiraClient jira.Adapter, ghClient github.PRClient, dryRun bool) ([]*UpdateJiraResult, []error) {
	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		return nil, []error{fmt.Errorf("워크트리 목록 조회 실패: %w", err)}
	}

	mainRepo, err := git.GetRepoRoot(repoDir)
	if err != nil {
		return nil, []error{fmt.Errorf("메인 레포 경로 조회 실패: %w", err)}
	}

	_, login, _ := config.GetJiraConfig()
	wf := config.ResolveJiraWorkflow(repoDir)

	re := config.IssueKeyExtractRegex()
	var results []*UpdateJiraResult
	var errors []error

	for _, wt := range worktrees {
		// Skip main repo
		if wt.Path == mainRepo {
			continue
		}

		issueKey := re.FindString(wt.Branch)
		if issueKey == "" {
			continue
		}

		// Pre-check: skip terminal statuses early to avoid unnecessary PR lookups
		issue, err := jiraClient.ViewIssue(issueKey)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: Jira 조회 실패: %w", issueKey, err))
			continue
		}

		// Skip issues not assigned to the current user
		if login != "" && issue.Assignee != "" && !strings.EqualFold(issue.Assignee, login) {
			continue
		}

		if wf.IsTerminal(issue.Status, issue.StatusCategory) {
			r := &UpdateJiraResult{
				IssueKey:        issueKey,
				IssueSummary:    issue.Summary,
				CurrentStatus:   issue.Status,
				Skipped:         true,
				SkipReason:      fmt.Sprintf("이미 종료 상태: %s", issue.Status),
				PendingChildren: findPendingChildren(jiraClient, issueKey, wf),
			}
			results = append(results, r)
			continue
		}

		result, err := UpdateJira(repoDir, jiraClient, ghClient, UpdateJiraOptions{
			IssueKey: issueKey,
			DryRun:   dryRun,
		})
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", issueKey, err))
			continue
		}
		results = append(results, result)
	}

	return results, errors
}

// UpdateJiraMine searches for issues assigned to the current user (via JQL) and
// runs the update flow for each, skipping keys already handled by UpdateJiraAll.
func UpdateJiraMine(repoDir string, jiraClient jira.Adapter, ghClient github.PRClient, skipKeys map[string]bool, dryRun bool) ([]*UpdateJiraResult, []error) {
	_, login, _ := config.GetJiraConfig()
	if login == "" {
		return nil, []error{fmt.Errorf("Jira 로그인 정보 미설정 (grove init 또는 grove config로 설정)")}
	}
	wf := config.ResolveJiraWorkflow(repoDir)

	prefix := config.GetIssuePrefix()
	jql := fmt.Sprintf(`assignee = "%s" AND statusCategory != Done`, login)
	if prefix != "" {
		jql += fmt.Sprintf(` AND project = "%s"`, prefix)
	}
	jql += " ORDER BY updated DESC"

	issues, err := jiraClient.SearchIssues(jql)
	if err != nil {
		return nil, []error{fmt.Errorf("내 이슈 조회 실패: %w", err)}
	}

	var results []*UpdateJiraResult
	var errors []error

	for _, issue := range issues {
		if skipKeys[issue.Key] {
			continue
		}
		if wf.IsTerminal(issue.Status, issue.StatusCategory) {
			continue
		}

		result, err := UpdateJira(repoDir, jiraClient, ghClient, UpdateJiraOptions{
			IssueKey:      issue.Key,
			DryRun:        dryRun,
			AllowNoBranch: true,
		})
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", issue.Key, err))
			continue
		}
		results = append(results, result)
	}

	return results, errors
}

// UpdateJira checks GitHub PR status and transitions the Jira issue accordingly.
func UpdateJira(repoDir string, jiraClient jira.Adapter, ghClient github.PRClient, opts UpdateJiraOptions) (*UpdateJiraResult, error) {
	issueKey := opts.IssueKey
	result := &UpdateJiraResult{IssueKey: issueKey}
	wf := config.ResolveJiraWorkflow(repoDir)

	// 1. Detect branch for this issue
	branch, err := FindBranchForIssue(repoDir, issueKey)
	if err != nil {
		if !opts.AllowNoBranch {
			return nil, fmt.Errorf("브랜치를 찾을 수 없습니다: %w", err)
		}
		branch = ""
	}

	// 2. Get current Jira issue (status + summary in one call)
	issue, err := jiraClient.ViewIssue(issueKey)
	if err != nil {
		return nil, fmt.Errorf("Jira 조회 실패: %w", err)
	}
	result.CurrentStatus = issue.Status
	result.IssueSummary = issue.Summary

	// 3. Check if already in terminal state
	if wf.IsTerminal(issue.Status, issue.StatusCategory) {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("이미 종료 상태: %s", issue.Status)
		result.PendingChildren = findPendingChildren(jiraClient, issueKey, wf)
		return result, nil
	}

	// 4. Find PR by branch (skip if no branch)
	var pr *github.PRState
	if branch != "" {
		pr, err = ghClient.FindPRByBranch(repoDir, branch)
		if err != nil {
			return nil, fmt.Errorf("PR 조회 실패: %w", err)
		}
	}

	if pr != nil {
		result.PRState = pr.State
		result.PRNumber = pr.Number
		result.PRURL = pr.URL
	}

	// 5. Check if parent issue is resolved → follow parent (requires confirmation in CLI)
	if issue.ParentKey != "" && wf.Statuses.ResolvedClosed != "" {
		parent, err := jiraClient.ViewIssue(issue.ParentKey)
		if err == nil && parent.Status == wf.Statuses.ResolvedClosed {
			result.ParentKey = parent.Key
			result.ParentResolved = true
			result.Action = fmt.Sprintf("부모이슈 %s %s", parent.Key, wf.Statuses.ResolvedClosed)
			result.Skipped = true
			result.SkipReason = fmt.Sprintf("부모이슈 %s %s (확인 후 전이 가능)", parent.Key, wf.Statuses.ResolvedClosed)
			return result, nil
		}
	}

	// 6. Determine transition
	hasWt := hasWorktreeForIssue(issueKey)
	transition, action, skipReason := determineTransition(wf, issue.Status, pr, jiraClient, issueKey, hasWt)

	if skipReason != "" {
		result.Skipped = true
		result.SkipReason = skipReason
		return result, nil
	}

	if transition == "" {
		result.Skipped = true
		result.SkipReason = "전이할 상태가 없습니다"
		return result, nil
	}

	result.Action = action

	// 7. Execute transition
	if opts.DryRun {
		if isCloseTransition(wf, transition) {
			result.PendingChildren = findPendingChildren(jiraClient, issueKey, wf)
		}
		return result, nil
	}

	if err := jiraClient.TransitionIssue(issueKey, transition); err != nil {
		return nil, fmt.Errorf("Jira 전이 실패 (%s): %w", transition, err)
	}
	result.Transitioned = true

	if isCloseTransition(wf, transition) {
		result.PendingChildren = findPendingChildren(jiraClient, issueKey, wf)
	}

	return result, nil
}

// isCloseTransition returns true if the transition leads to a terminal state.
func isCloseTransition(wf config.JiraWorkflow, transition string) bool {
	if transition == "" {
		return false
	}
	return transition == wf.Transitions.Close || transition == wf.Transitions.ResolvedClose
}

// findPendingChildren returns non-terminal child issues of the given issue.
func findPendingChildren(jiraClient jira.Adapter, issueKey string, wf config.JiraWorkflow) []jira.Issue {
	children, err := jiraClient.ListChildren(issueKey)
	if err != nil {
		return nil
	}
	var pending []jira.Issue
	for _, child := range children {
		if !wf.IsTerminal(child.Status, child.StatusCategory) {
			pending = append(pending, child)
		}
	}
	return pending
}

// determineTransition decides which Jira transition to apply, using the
// resolved workflow's status/transition names. A stage with an empty transition
// name is skipped (that stage is not part of this project's workflow).
//
// 배포일자가 설정된 경우 PR 상태와 무관하게 배포일자만으로 결정한다.
// 배포일자 미설정 시 PR 상태 기반 전이 로직으로 폴백한다.
func determineTransition(wf config.JiraWorkflow, status string, pr *github.PRState, jiraClient jira.Adapter, issueKey string, hasWorktree bool) (transition, action, skipReason string) {
	// 배포일자 설정 여부 먼저 확인 → PR 상태 무관하게 배포일자로 결정
	if wf.DeployField != "" && wf.Transitions.ResolvedClose != "" {
		deployDate, err := jiraClient.GetDeployDate(issueKey)
		if err == nil && deployDate != "" {
			deploy, parseErr := time.Parse("2006-01-02", deployDate)
			if parseErr != nil {
				return "", "", fmt.Sprintf("배포일자 파싱 실패: %s", deployDate)
			}
			today := time.Now().Truncate(24 * time.Hour)
			if deploy.After(today) {
				return "", "", fmt.Sprintf("배포일자 미도래: %s (현재: %s)", deployDate, status)
			}
			return wf.Transitions.ResolvedClose, fmt.Sprintf("배포일자 %s 도래 → %s", deployDate, wf.Statuses.ResolvedClosed), ""
		}
	}

	// 배포일자 미설정: PR 상태로 전이 결정
	hasPR := pr != nil
	switch {
	// PR이 머지됐고 아직 리뷰중 이전 → 리뷰중 (noQA면 리뷰완료)
	case hasPR && pr.State == "MERGED" && !wf.IsAtOrAfter(status, wf.Statuses.InReview):
		if hasNoQALabel(pr) && wf.Transitions.ReviewComplete != "" {
			return wf.Transitions.ReviewComplete, fmt.Sprintf("PR #%d merged (noQA) → %s", pr.Number, wf.Statuses.ReviewComplete), ""
		}
		if wf.Transitions.InReview != "" {
			return wf.Transitions.InReview, fmt.Sprintf("PR #%d merged → %s", pr.Number, wf.Statuses.InReview), ""
		}

	// PR이 열려있고 아직 개발완료 전 → 개발완료
	case hasPR && pr.State == "OPEN" && !wf.IsAtOrAfter(status, wf.Statuses.DevComplete):
		if wf.Transitions.DevComplete != "" {
			return wf.Transitions.DevComplete, fmt.Sprintf("PR #%d open → %s", pr.Number, wf.Statuses.DevComplete), ""
		}

	// PR 없고 워크트리가 있고 아직 진행중 전 → 진행중
	case !hasPR && hasWorktree && !wf.IsAtOrAfter(status, wf.Statuses.InProgress):
		if wf.Transitions.InProgress != "" {
			return wf.Transitions.InProgress, fmt.Sprintf("워크트리 존재 → %s", wf.Statuses.InProgress), ""
		}

	// PR 없음
	case !hasPR:
		return "", "", "PR이 없습니다"
	}

	if !hasPR {
		return "", "", "PR이 없습니다"
	}
	return "", "", fmt.Sprintf("PR 상태(%s)와 Jira 상태(%s)가 일치합니다", pr.State, status)
}

// FindBranchForIssue finds the git branch associated with an issue key.
func FindBranchForIssue(repoDir string, issueKey string) (string, error) {
	// First try: current branch
	branch, err := git.GetCurrentBranch(repoDir)
	if err == nil && strings.Contains(strings.ToUpper(branch), strings.ToUpper(issueKey)) {
		return branch, nil
	}

	// Second try: scan all worktrees
	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		return "", err
	}

	re := config.IssueKeyExtractRegex()
	for _, wt := range worktrees {
		key := re.FindString(wt.Branch)
		if strings.EqualFold(key, issueKey) {
			return wt.Branch, nil
		}
	}

	// Third try: common branch patterns (honoring the repo's resolved policy)
	for _, t := range config.BranchTypeNames(repoDir) {
		candidate := config.FormatBranchNameRepo(repoDir, t, issueKey)
		if git.BranchExists(repoDir, candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("이슈 %s에 해당하는 브랜치를 찾을 수 없습니다", issueKey)
}

// hasWorktreeForIssue checks if a worktree directory exists for the given issue key
// by scanning for *-worktrees/<issueKey> directories. This works regardless of
// whether the current directory is inside a git repository.
func hasWorktreeForIssue(issueKey string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}

	// Walk up from cwd to find a parent that contains *-worktrees/<issueKey>
	dir := cwd
	for {
		matches, _ := filepath.Glob(filepath.Join(dir, "*-worktrees", issueKey))
		for _, m := range matches {
			info, err := os.Stat(m)
			if err == nil && info.IsDir() {
				return true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}

// hasNoQALabel checks if the PR has a noQA label.
func hasNoQALabel(pr *github.PRState) bool {
	for _, label := range pr.Labels {
		lower := strings.ToLower(label)
		if lower == "noqa" || lower == "no-qa" || lower == "no_qa" {
			return true
		}
	}
	return false
}
