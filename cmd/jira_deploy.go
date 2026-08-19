package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/git"
	"github.com/harudev/grove-cli/internal/github"
	"github.com/harudev/grove-cli/internal/jira"
	"github.com/harudev/grove-cli/internal/tui"
)

var jiraDeployCmd = &cobra.Command{
	Use:   "deploy [ISSUE-KEY]",
	Short: "Jira 이슈에 배포일자 설정",
	Long: `Jira 이슈의 배포일자(설정된 커스텀 필드)를 설정합니다.
날짜를 지정하지 않으면 설정된 배포 요일(기본 화요일) 중 선택할 수 있습니다.

배포일자 커스텀 필드는 grove config jira-workflow의 deployField로 지정해야 합니다.

사용 예시:
  grove jira deploy                    # 현재 워크트리 이슈, 배포 요일 선택
  grove jira deploy PROJ-12345         # 특정 이슈
  grove jira deploy --date 2026-04-07  # 날짜 직접 지정
  grove jira deploy --all              # 리뷰 단계 이후 + 배포일자 미설정 이슈 일괄 설정`,
	Args: cobra.MaximumNArgs(1),
	RunE: runJiraDeploy,
}

func init() {
	jiraDeployCmd.Flags().String("date", "", "배포일자 직접 지정 (YYYY-MM-DD)")
	jiraDeployCmd.Flags().Bool("all", false, "리뷰중 이후 + 배포일자 미설정 이슈 일괄 설정")
	jiraCmd.AddCommand(jiraDeployCmd)
}

func runJiraDeploy(cmd *cobra.Command, args []string) error {
	allMode, _ := cmd.Flags().GetBool("all")

	if config.GetJiraWorkflow().DeployField == "" {
		return errDeployFieldNotConfigured
	}

	jiraClient, err := requireJiraClient()
	if err != nil {
		return err
	}

	if allMode {
		repoDir, err := os.Getwd()
		if err != nil {
			return err
		}
		return runJiraDeployAll(repoDir, jiraClient, cmd)
	}

	issueKey, err := resolveIssueKey(args)
	if err != nil {
		return err
	}

	repoDir, _ := os.Getwd()
	deployDate, err := resolveDeployDate(cmd, repoDir, issueKey)
	if err != nil {
		return err
	}

	stop := tui.Spinner("배포일자 설정 중")
	err = jiraClient.SetDeployDate(issueKey, deployDate)
	stop()
	if err != nil {
		return fmt.Errorf("배포일자 설정 실패: %w", err)
	}

	suffix := ""
	if dd, err := time.Parse("2006-01-02", deployDate); err == nil {
		today := time.Now().Truncate(24 * time.Hour)
		if !dd.After(today) {
			issue, _ := jiraClient.ViewIssue(issueKey)
			if issue != nil {
				suffix = closeOnPastDeploy(jiraClient, issueKey, issue.Status, issue.StatusCategory)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "%s %s - 배포일자 → %s%s\n", "✅", jiraIssueHyperlink(issueKey), cyanStyle.Render(deployDate), suffix)
	return nil
}

// closeOnPastDeploy transitions a non-terminal issue to the resolved-close
// state (used when a deploy date has already passed). Returns a suffix string
// describing the outcome, or "" when nothing was done.
func closeOnPastDeploy(jiraClient jira.Adapter, issueKey, status, statusCategory string) string {
	wf := config.GetJiraWorkflow()
	if wf.Transitions.ResolvedClose == "" || wf.IsTerminal(status, statusCategory) {
		return ""
	}
	resolvedName := wf.Statuses.ResolvedClosed
	if resolvedName == "" {
		resolvedName = wf.Transitions.ResolvedClose
	}
	if err := jiraClient.TransitionIssue(issueKey, wf.Transitions.ResolvedClose); err != nil {
		return fmt.Sprintf(" (%s 전이 실패: %v)", resolvedName, err)
	}
	return " → " + resolvedName
}

// resolveDeployDate returns a deploy date from the --date flag or interactive selector.
// If repoDir and issueKey are provided, it tries to find the last merged PR by issue key
// and offers 3 Tuesdays from that merge date.
func resolveDeployDate(cmd *cobra.Command, repoDir, issueKey string) (string, error) {
	dateFlag, _ := cmd.Flags().GetString("date")
	if dateFlag != "" {
		if _, err := time.Parse("2006-01-02", dateFlag); err != nil {
			return "", fmt.Errorf("날짜 형식이 올바르지 않습니다 (YYYY-MM-DD): %s", dateFlag)
		}
		return dateFlag, nil
	}

	weekday := config.GetJiraWorkflow().DeployWeekdayValue()

	// PR 머지 날짜 기반 배포 요일 옵션 시도
	if repoDir != "" && issueKey != "" {
		ghClient := github.NewGHCLIClient()
		if ghClient.Available() {
			mergeDate, err := ghClient.FindLastMergedPRDate(repoDir, issueKey)
			if err == nil {
				options := tui.WeekdaysFromDate(mergeDate, weekday)
				fmt.Fprintf(os.Stderr, "%s 마지막 PR 머지: %s\n\n", cyanStyle.Render("🔍"), mergeDate.Local().Format("2006-01-02"))
				return tui.RunDeployDateSelector(options)
			}
			if errors.Is(err, github.ErrNoPRFound) {
				fmt.Fprintf(os.Stderr, "%s 머지된 PR을 찾을 수 없습니다\n\n", yellowStyle.Render("⚠"))
			} else {
				fmt.Fprintf(os.Stderr, "%s PR 조회 실패\n\n", yellowStyle.Render("⚠"))
			}
		}
	}

	// 폴백: 현재 날짜 기준 배포 요일
	options := tui.NextWeekdays(time.Now(), weekday)
	return tui.RunDeployDateSelector(options)
}

// runJiraDeployAll finds all worktree issues that are at or after "리뷰중" with no deploy date, then sets deploy dates.
func runJiraDeployAll(repoDir string, jiraClient jira.Adapter, cmd *cobra.Command) error {
	fmt.Fprintf(os.Stderr, "\n%s 배포일자 미설정 이슈 일괄 설정\n\n", cyanStyle.Render("📅"))

	stop := tui.Spinner("배포 대상 이슈 조회 중")
	targets, scanErrs := findDeployTargets(repoDir, jiraClient)
	stop()

	if len(scanErrs) > 0 {
		for _, e := range scanErrs {
			fmt.Fprintf(os.Stderr, "  %s %s\n", redStyle.Render("✗"), e)
		}
	}

	if len(targets) == 0 {
		fmt.Fprintf(os.Stderr, "  배포일자를 설정할 이슈가 없습니다.\n\n")
		return nil
	}

	// 부모이슈 배포일자가 있는 것과 없는 것 분리
	var autoTargets, manualTargets []deployTarget
	for _, t := range targets {
		if t.parentDeployDate != "" {
			autoTargets = append(autoTargets, t)
		} else {
			manualTargets = append(manualTargets, t)
		}
	}

	// 부모이슈 배포일자 자동 설정
	if len(autoTargets) > 0 {
		fmt.Fprintf(os.Stderr, "  %s 부모이슈 배포일자 적용:\n\n", yellowStyle.Render("📎"))
		if err := setDeployDates(jiraClient, autoTargets, func(t deployTarget) (string, error) {
			return t.parentDeployDate, nil
		}); err != nil {
			return err
		}
	}

	if len(manualTargets) == 0 {
		return nil
	}

	// 나머지 이슈 목록 표시
	for _, t := range manualTargets {
		fmt.Fprintf(os.Stderr, "  %s %s - [%s]\n", cyanStyle.Render("•"), jiraIssueLabel(t.issueKey, t.summary), t.status)
	}
	fmt.Fprintf(os.Stderr, "\n")

	// Ask: same date for all?
	sameDate, err := tui.RunConfirm("모든 이슈에 같은 배포일자를 설정할까요?")
	if err != nil {
		return err
	}

	if sameDate {
		deployDate, err := resolveDeployDate(cmd, repoDir, "")
		if err != nil {
			return err
		}
		return setDeployDates(jiraClient, manualTargets, func(_ deployTarget) (string, error) { return deployDate, nil })
	}

	// Per-issue date selection
	return setDeployDates(jiraClient, manualTargets, func(t deployTarget) (string, error) {
		fmt.Fprintf(os.Stderr, "\n%s\n%s\n", dimStyle.Render("────────────────────────────────────────"), jiraIssueLabel(t.issueKey, t.summary))
		return resolveDeployDate(cmd, repoDir, t.issueKey)
	})
}

type deployTarget struct {
	issueKey         string
	summary          string
	status           string
	statusCategory   string
	parentKey        string
	parentDeployDate string
}

// findDeployTargets scans all worktrees for issues at or after "리뷰중" with no deploy date.
// Uses ViewIssue to get status, summary, and deploy date in a single API call per issue.
func findDeployTargets(repoDir string, jiraClient jira.Adapter) ([]deployTarget, []error) {
	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		return nil, []error{fmt.Errorf("워크트리 목록 조회 실패: %w", err)}
	}

	mainRepo, err := git.GetRepoRoot(repoDir)
	if err != nil {
		return nil, []error{fmt.Errorf("메인 레포 경로 조회 실패: %w", err)}
	}

	wf := config.GetJiraWorkflow()
	re := config.IssueKeyExtractRegex()
	var targets []deployTarget
	var errs []error

	for _, wt := range worktrees {
		if wt.Path == mainRepo {
			continue
		}

		issueKey := re.FindString(wt.Branch)
		if issueKey == "" {
			continue
		}

		issue, err := jiraClient.ViewIssue(issueKey)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: 조회 실패: %w", issueKey, err))
			continue
		}

		if wf.IsTerminal(issue.Status, issue.StatusCategory) {
			continue
		}

		// 3개월 이상 업데이트되지 않은 이슈 제외
		if issue.Updated != "" {
			if updated, err := time.Parse("2006-01-02T15:04:05.000-0700", issue.Updated); err == nil {
				if time.Since(updated) > 90*24*time.Hour {
					continue
				}
			}
		}

		if !wf.IsReviewOrAfter(issue.Status) {
			continue
		}

		if issue.DeployDate != "" {
			continue
		}

		t := deployTarget{issueKey: issueKey, summary: issue.Summary, status: issue.Status, statusCategory: issue.StatusCategory, parentKey: issue.ParentKey}

		// 부모이슈의 배포일자 조회
		if issue.ParentKey != "" {
			parent, err := jiraClient.ViewIssue(issue.ParentKey)
			if err == nil && parent.DeployDate != "" {
				t.parentDeployDate = parent.DeployDate
			}
		}

		targets = append(targets, t)
	}

	return targets, errs
}

// setDeployDates applies deploy dates to targets, calling dateFn for each to get the date.
func setDeployDates(jiraClient jira.Adapter, targets []deployTarget, dateFn func(deployTarget) (string, error)) error {
	var success, failed int
	fmt.Fprintf(os.Stderr, "\n")

	for _, t := range targets {
		date, err := dateFn(t)
		if err != nil {
			return err
		}

		label := jiraIssueLabel(t.issueKey, t.summary)
		if err := jiraClient.SetDeployDate(t.issueKey, date); err != nil {
			fmt.Fprintf(os.Stderr, "%s %s - 설정 실패: %v\n", redStyle.Render("✗"), label, err)
			failed++
			continue
		}

		suffix := ""
		if t.parentDeployDate != "" && date == t.parentDeployDate {
			suffix = fmt.Sprintf(" (← %s)", t.parentKey)
		}

		// 배포일자가 오늘 이전이면 해결종료(resolved-close)로 전이
		if deployDate, err := time.Parse("2006-01-02", date); err == nil {
			today := time.Now().Truncate(24 * time.Hour)
			if !deployDate.After(today) {
				suffix += closeOnPastDeploy(jiraClient, t.issueKey, t.status, t.statusCategory)
			}
		}

		fmt.Fprintf(os.Stderr, "%s %s - 배포일자 → %s%s\n", "✅", label, cyanStyle.Render(date), suffix)
		success++
	}

	fmt.Fprintf(os.Stderr, "\n설정 완료: %d, 실패: %d\n\n", success, failed)
	return nil
}
