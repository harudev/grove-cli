package worktree

import (
	"fmt"
	"os"
	"regexp"

	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/git"
	"github.com/harudev/grove-cli/internal/jira"
	"github.com/harudev/grove-cli/internal/tui"
)

// CleanOptions holds options for the clean flow.
type CleanOptions struct {
	IssueKey string // specific issue to clean (e.g. "PROJ-12345")
	DoneMode bool   // --done: batch cleanup by Jira status
	Force    bool   // --force: skip confirmations
}

type failedEntry struct {
	IssueKey string
	Reason   string
}

// Clean runs the worktree cleanup flow.
func Clean(repoDir string, jiraClient jira.Adapter, prompter tui.Prompter, opts CleanOptions) error {
	if opts.IssueKey != "" {
		return cleanSpecific(repoDir, prompter, opts)
	}
	if opts.DoneMode {
		return cleanDone(repoDir, jiraClient, prompter, opts)
	}
	return cleanShowStatus(repoDir, jiraClient)
}

// cleanSpecific cleans a specific issue's worktree.
func cleanSpecific(repoDir string, prompter tui.Prompter, opts CleanOptions) error {
	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(regexp.QuoteMeta(opts.IssueKey))
	var matchedWorktrees []git.WorktreeInfo
	for _, wt := range worktrees {
		if re.MatchString(wt.Branch) {
			matchedWorktrees = append(matchedWorktrees, wt)
		}
	}

	if len(matchedWorktrees) == 0 {
		return fmt.Errorf("워크트리를 찾을 수 없습니다: %s", opts.IssueKey)
	}

	selected := matchedWorktrees[0]
	if len(matchedWorktrees) > 1 {
		// Multiple worktrees found, let user select
		var branchOptions []string
		for _, wt := range matchedWorktrees {
			branchOptions = append(branchOptions, wt.Branch)
		}

		selectedBranch, err := prompter.SelectWorktree(branchOptions)
		if err != nil {
			return err
		}

		for _, wt := range matchedWorktrees {
			if wt.Branch == selectedBranch {
				selected = wt
				break
			}
		}
	}

	var failed []failedEntry
	cleanupWorktree(repoDir, prompter, opts.IssueKey, selected.Path, selected.Branch, opts.Force, &failed)
	printFailedList(failed)
	return nil
}

// cleanDone batch-cleans worktrees whose Jira status is in safe statuses.
func cleanDone(repoDir string, jiraClient jira.Adapter, prompter tui.Prompter, opts CleanOptions) error {
	if jiraClient == nil {
		return fmt.Errorf("jira-cli가 설치되어 있지 않습니다. brew install ankitpokhrel/jira-cli/jira-cli")
	}

	logInfo("Jira 상태를 확인하여 정리 가능한 워크트리를 찾고 있습니다...")
	logInfo("(정리 기준: Jira 완료(Done) 카테고리 상태)")

	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		return err
	}

	mainRepo, err := git.GetRepoRoot(repoDir)
	if err != nil {
		return err
	}

	cleanedCount := 0
	skippedCount := 0
	var failed []failedEntry

	for _, wt := range worktrees {
		if wt.Path == mainRepo {
			continue
		}

		issueKey := extractIssueKey(wt.Branch)
		if issueKey == "" {
			continue
		}

		issue, err := jiraClient.ViewIssue(issueKey)
		if err != nil || issue.Status == "" {
			logWarn("상태 조회 실패: %s", issueKey)
			skippedCount++
			continue
		}

		if isSafeToClean(issue.StatusCategory) {
			logSuccess("정리 대상: %s (Jira: %s)", issueKey, issue.Status)
			if cleanupWorktree(repoDir, prompter, issueKey, wt.Path, wt.Branch, opts.Force, &failed) {
				cleanedCount++
			}
		} else {
			logWarn("진행중: %s (Jira: %s)", issueKey, issue.Status)
			skippedCount++
		}
	}

	fmt.Fprintln(os.Stderr)
	logSuccess("정리 완료: %d개", cleanedCount)
	logWarn("건너뜀: %d개", skippedCount)
	printFailedList(failed)

	return nil
}

// cleanShowStatus shows worktree status table (default mode).
func cleanShowStatus(repoDir string, jiraClient jira.Adapter) error {
	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		return err
	}

	mainRepo, err := git.GetRepoRoot(repoDir)
	if err != nil {
		return err
	}

	fmt.Println("워크트리 목록 (Jira 상태 포함):")
	fmt.Println()
	fmt.Println("| 이슈 번호 | Jira 상태 | 정리 가능 | 브랜치 | 경로 |")
	fmt.Println("|-----------|-----------|-----------|--------|------|")

	found := false
	for _, wt := range worktrees {
		if wt.Path == mainRepo {
			continue
		}

		issueKey := extractIssueKey(wt.Branch)
		if issueKey == "" {
			continue
		}

		status := "조회실패"
		statusCategory := ""
		cleanable := "-"
		if jiraClient != nil {
			if issue, err := jiraClient.ViewIssue(issueKey); err == nil && issue.Status != "" {
				status = issue.Status
				statusCategory = issue.StatusCategory
			}
		}
		if isSafeToClean(statusCategory) {
			cleanable = "O"
		}

		fmt.Printf("| %s | %s | %s | %s | %s |\n", issueKey, status, cleanable, wt.Branch, wt.Path)
		found = true
	}

	if !found {
		logSuccess("정리할 워크트리가 없습니다.")
	} else {
		fmt.Println()
		fmt.Println("사용법:")
		fmt.Println("  grove clean PROJ-12345         # 특정 이슈 정리")
		fmt.Println("  grove clean --done             # Jira 완료(Done) 카테고리 상태 일괄 정리")
	}

	return nil
}

// cleanupWorktree handles the actual cleanup of a single worktree.
func cleanupWorktree(repoDir string, prompter tui.Prompter, issueKey, worktreePath, branchName string, force bool, failed *[]failedEntry) bool {
	// Check if running inside the target worktree
	cwd, _ := os.Getwd()
	if cwd == worktreePath {
		logError("현재 워크트리 내에서는 해당 워크트리를 삭제할 수 없습니다.")
		*failed = append(*failed, failedEntry{issueKey, "현재 워크트리 내에서 실행 중"})
		return false
	}

	// Check uncommitted changes
	unstaged, untracked, err := git.HasUncommittedChanges(worktreePath)
	if err == nil && (unstaged > 0 || untracked > 0) {
		logWarn("미커밋 변경사항이 있습니다:")
		fmt.Fprintf(os.Stderr, "  수정된 파일: %d개\n", unstaged)
		fmt.Fprintf(os.Stderr, "  추적되지 않은 파일: %d개\n", untracked)

		if !force {
			confirmed, err := prompter.Confirm("정말 삭제하시겠습니까?")
			if err != nil || !confirmed {
				*failed = append(*failed, failedEntry{issueKey, "미커밋 변경사항으로 사용자 취소"})
				return false
			}
		} else {
			logWarn("--force: 미커밋 변경사항을 무시하고 강제 삭제합니다.")
		}
	}

	// Remove worktree
	stop := spinner(fmt.Sprintf("🗑️  워크트리 삭제 중: %s", issueKey))
	if err := git.WorktreeRemove(repoDir, worktreePath, force); err != nil {
		stop()
		logError("워크트리 제거 실패")
		*failed = append(*failed, failedEntry{issueKey, fmt.Sprintf("워크트리 제거 실패: %v", err)})
		return false
	}

	// Delete local branch
	if err := git.DeleteBranch(repoDir, branchName, force); err != nil {
		stop()
		logSuccess("워크트리 제거 완료")
		logWarn("브랜치 삭제 실패: %s", branchName)
		*failed = append(*failed, failedEntry{issueKey, fmt.Sprintf("브랜치 삭제 실패: %v", err)})
	} else {
		// Prune
		git.WorktreePrune(repoDir)
		stop()
		logSuccess("워크트리 및 브랜치 삭제 완료: %s", branchName)
	}

	return true
}

// isSafeToClean reports whether a worktree can be safely cleaned up, based on
// the issue's Jira statusCategory ("new", "indeterminate", "done") rather than
// a specific status name — every status a project classifies as its "완료"
// category counts, regardless of project-specific naming (e.g. TEAM's "완료").
func isSafeToClean(statusCategory string) bool {
	return statusCategory == "done"
}

func extractIssueKey(branch string) string {
	return config.IssueKeyExtractRegex().FindString(branch)
}

func printFailedList(failed []failedEntry) {
	if len(failed) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr)
	logError("── 삭제 실패 목록 ──")
	for _, f := range failed {
		fmt.Fprintf(os.Stderr, "  ✗ %s: %s\n", f.IssueKey, f.Reason)
	}
}
