package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/github"
	"github.com/harudev/grove-cli/internal/jira"
	"github.com/harudev/grove-cli/internal/tui"
	"github.com/harudev/grove-cli/internal/worktree"
)

var jiraUpdateCmd = &cobra.Command{
	Use:   "update [ISSUE-KEY]",
	Short: "GitHub PR 상태에 따라 Jira 이슈 상태 업데이트",
	Long: `GitHub PR 상태를 확인하고 Jira 이슈 상태를 자동으로 전이합니다.

전이 규칙:
  PR open    + 개발완료 이전  → 개발완료
  PR merged  + 개발완료      → 리뷰중 (noQA 라벨 시 리뷰완료)
  리뷰통과 이후 + 배포일자 도래 → 해결종료

사용 예시:
  grove jira update                    # 현재 워크트리의 이슈 자동 감지
  grove jira update PROJ-12345       # 특정 이슈
  grove jira update 12345              # 숫자만 (prefix 자동 적용)
  grove jira update --all              # 모든 워크트리 대상으로 일괄 실행
  grove jira update --no-grove         # 워크트리 없는 내 이슈도 포함
  grove jira update --all --no-grove   # 워크트리 이슈 + 내 이슈 모두
  grove jira update --all --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: runJiraUpdate,
}

func init() {
	jiraUpdateCmd.Flags().Bool("dry-run", false, "실제 변경 없이 전이 내용만 확인")
	jiraUpdateCmd.Flags().Bool("all", false, "모든 워크트리 대상으로 일괄 실행")
	jiraUpdateCmd.Flags().Bool("no-grove", false, "워크트리 없는 내 이슈도 포함 (JQL: assignee = currentUser)")
	jiraUpdateCmd.Flags().BoolP("verbose", "v", false, "스킵된 이슈도 함께 표시")
	jiraCmd.AddCommand(jiraUpdateCmd)
}

func runJiraUpdate(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	allMode, _ := cmd.Flags().GetBool("all")
	noGrove, _ := cmd.Flags().GetBool("no-grove")
	verbose, _ := cmd.Flags().GetBool("verbose")

	repoDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if !config.ResolveJiraWorkflow(repoDir).Configured() {
		return errJiraWorkflowNotConfigured
	}

	jiraClient, err := requireJiraClient()
	if err != nil {
		return err
	}

	ghClient, err := requireGHClient()
	if err != nil {
		return err
	}

	// --all mode: process all worktrees (optionally also no-grove)
	if allMode {
		return runJiraUpdateAll(repoDir, jiraClient, ghClient, dryRun, verbose, noGrove)
	}

	// --no-grove only: process assigned issues without worktrees
	if noGrove {
		return runJiraUpdateMine(repoDir, jiraClient, ghClient, dryRun, verbose, nil)
	}

	// Single issue mode
	issueKey, err := resolveIssueKey(args)
	if err != nil {
		return err
	}

	opts := worktree.UpdateJiraOptions{
		IssueKey: issueKey,
		DryRun:   dryRun,
	}

	stop := tui.Spinner("Jira 상태 동기화 중")
	result, err := worktree.UpdateJira(repoDir, jiraClient, ghClient, opts)
	stop()
	if err != nil {
		return err
	}

	printUpdateResult(result, dryRun)

	if !dryRun {
		if len(result.PendingChildren) > 0 {
			if err := promptAndCloseChildren(jiraClient, result.IssueKey, result.PendingChildren); err != nil {
				return err
			}
		}
		if result.ParentResolved {
			child := jira.Issue{Key: result.IssueKey, Summary: result.IssueSummary, Status: result.CurrentStatus}
			if err := promptAndCloseChildren(jiraClient, result.ParentKey, []jira.Issue{child}); err != nil {
				return err
			}
		}
	}
	return nil
}

func runJiraUpdateAll(repoDir string, jiraClient jira.Adapter, ghClient *github.GHCLIClient, dryRun bool, verbose bool, noGrove bool) error {
	fmt.Fprintf(os.Stderr, "\n%s 모든 워크트리 Jira 상태 동기화\n\n", cyanStyle.Render("🔄"))

	stop := tui.Spinner("모든 워크트리 Jira 상태 동기화 중")
	results, errs := worktree.UpdateJiraAll(repoDir, jiraClient, ghClient, dryRun)
	stop()

	if noGrove {
		processedKeys := make(map[string]bool)
		for _, r := range results {
			processedKeys[r.IssueKey] = true
		}
		mineResults, mineErrs := worktree.UpdateJiraMine(repoDir, jiraClient, ghClient, processedKeys, dryRun)
		results = append(results, mineResults...)
		errs = append(errs, mineErrs...)
	}

	return printUpdateAllResults(results, errs, dryRun, verbose, jiraClient)
}

func runJiraUpdateMine(repoDir string, jiraClient jira.Adapter, ghClient *github.GHCLIClient, dryRun bool, verbose bool, skipKeys map[string]bool) error {
	fmt.Fprintf(os.Stderr, "\n%s 내 이슈 Jira 상태 동기화\n\n", cyanStyle.Render("🔄"))

	stop := tui.Spinner("내 이슈 Jira 상태 동기화 중")
	results, errs := worktree.UpdateJiraMine(repoDir, jiraClient, ghClient, skipKeys, dryRun)
	stop()

	return printUpdateAllResults(results, errs, dryRun, verbose, jiraClient)
}

func printUpdateAllResults(results []*worktree.UpdateJiraResult, errs []error, dryRun bool, verbose bool, jiraClient jira.Adapter) error {
	var transitioned, skipped int
	pendingMap := make(map[string][]jira.Issue)

	for _, r := range results {
		if r.Skipped {
			skipped++
			if verbose {
				printUpdateResultCompact(r, dryRun)
			}
		} else {
			transitioned++
			printUpdateResultCompact(r, dryRun)
		}

		// 1. Parent just resolved → collect all its non-terminal children
		if len(r.PendingChildren) > 0 {
			pendingMap[r.IssueKey] = append(pendingMap[r.IssueKey], r.PendingChildren...)
		}

		// 2. Child saw parent already resolved → collect this child
		if r.ParentResolved {
			pendingMap[r.ParentKey] = append(pendingMap[r.ParentKey], jira.Issue{
				Key:     r.IssueKey,
				Summary: r.IssueSummary,
				Status:  r.CurrentStatus,
			})
		}
	}

	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "  %s %s\n", redStyle.Render("✗"), e)
	}

	fmt.Fprintf(os.Stderr, "\n")
	if dryRun {
		fmt.Fprintf(os.Stderr, "  [dry-run] 전이 대상: %d, 스킵: %d, 오류: %d\n", transitioned, skipped, len(errs))
	} else {
		fmt.Fprintf(os.Stderr, "  전이 완료: %d, 스킵: %d, 오류: %d\n", transitioned, skipped, len(errs))
	}
	fmt.Fprintf(os.Stderr, "\n")

	if !dryRun && len(pendingMap) > 0 {
		// De-duplicate children per parent and prompt
		for parentKey, children := range pendingMap {
			unique := make(map[string]jira.Issue)
			for _, c := range children {
				unique[c.Key] = c
			}

			var sortedChildren []jira.Issue
			for _, c := range unique {
				sortedChildren = append(sortedChildren, c)
			}

			if err := promptAndCloseChildren(jiraClient, parentKey, sortedChildren); err != nil {
				return err
			}
		}
	}

	return nil
}

// jiraIssueHyperlink renders issueKey as a clickable terminal hyperlink (OSC 8) if Jira server is configured.
func jiraIssueHyperlink(issueKey string) string {
	server, _, _ := config.GetJiraConfig()
	if server == "" {
		return cyanStyle.Render(issueKey)
	}
	url := fmt.Sprintf("%s/browse/%s", server, issueKey)
	// OSC 8 terminal hyperlink: \e]8;;URL\e\\TEXT\e]8;;\e\\
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, cyanStyle.Render(issueKey))
}

func jiraIssueLabel(issueKey, summary string) string {
	link := jiraIssueHyperlink(issueKey)
	if summary != "" {
		return fmt.Sprintf("%s %s", link, summary)
	}
	return link
}

func printUpdateResult(result *worktree.UpdateJiraResult, dryRun bool) {
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  이슈:       %s\n", jiraIssueLabel(result.IssueKey, result.IssueSummary))
	fmt.Fprintf(os.Stderr, "  현재 상태:  %s\n", result.CurrentStatus)

	if result.PRNumber > 0 {
		fmt.Fprintf(os.Stderr, "  PR:         #%d (%s)\n", result.PRNumber, result.PRState)
	} else {
		fmt.Fprintf(os.Stderr, "  PR:         없음\n")
	}

	fmt.Fprintf(os.Stderr, "\n")

	if result.Skipped {
		fmt.Fprintf(os.Stderr, "  %s %s\n", yellowStyle.Render("⏭"), result.SkipReason)
	} else if dryRun {
		fmt.Fprintf(os.Stderr, "  %s [dry-run] %s\n", cyanStyle.Render("→"), result.Action)
	} else if result.Transitioned {
		fmt.Fprintf(os.Stderr, "  %s %s\n", "✅", result.Action)
	}

	fmt.Fprintf(os.Stderr, "\n")
}

// promptAndCloseChildren shows pending children and asks whether to close them.
func promptAndCloseChildren(jiraClient jira.Adapter, parentKey string, children []jira.Issue) error {
	fmt.Fprintf(os.Stderr, "  %s %s의 미종료 자식이슈:\n\n", yellowStyle.Render("📎"), jiraIssueHyperlink(parentKey))
	for _, c := range children {
		fmt.Fprintf(os.Stderr, "    %s %s [%s]\n", cyanStyle.Render("•"), jiraIssueLabel(c.Key, c.Summary), c.Status)
	}
	fmt.Fprintf(os.Stderr, "\n")

	wf := config.GetJiraWorkflow()
	if wf.Transitions.ResolvedClose == "" {
		return nil
	}
	resolvedName := wf.Statuses.ResolvedClosed
	if resolvedName == "" {
		resolvedName = wf.Transitions.ResolvedClose
	}

	confirmed, err := tui.RunConfirm(fmt.Sprintf("자식이슈도 %s로 전이할까요?", resolvedName))
	if err != nil || !confirmed {
		return err
	}

	for _, c := range children {
		label := jiraIssueLabel(c.Key, c.Summary)
		if err := jiraClient.TransitionIssue(c.Key, wf.Transitions.ResolvedClose); err != nil {
			fmt.Fprintf(os.Stderr, "    %s %s - 전이 실패: %v\n", redStyle.Render("✗"), label, err)
		} else {
			fmt.Fprintf(os.Stderr, "    %s %s → %s\n", "✅", label, resolvedName)
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
	return nil
}

func printUpdateResultCompact(result *worktree.UpdateJiraResult, dryRun bool) {
	prefix := "  "
	label := jiraIssueLabel(result.IssueKey, result.IssueSummary)

	if result.Skipped {
		fmt.Fprintf(os.Stderr, "%s%s %s - [%s] %s\n", prefix, yellowStyle.Render("⏭"), label, result.CurrentStatus, result.SkipReason)
	} else if dryRun {
		fmt.Fprintf(os.Stderr, "%s%s %s - [%s] %s\n", prefix, cyanStyle.Render("→"), label, result.CurrentStatus, result.Action)
	} else if result.Transitioned {
		fmt.Fprintf(os.Stderr, "%s%s %s - %s\n", prefix, "✅", label, result.Action)
	}
}
