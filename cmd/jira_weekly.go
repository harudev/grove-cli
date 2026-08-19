package cmd

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/jira"
	"github.com/harudev/grove-cli/internal/tui"
	"github.com/spf13/cobra"
)

var jiraWeeklyCmd = &cobra.Command{
	Use:   "weekly",
	Short: "주간 이슈 현황 조회",
	Long: `지난주에 진행한 이슈와 이번주에 진행중/할일 이슈를 테이블 형태로 보여줍니다.

사용 예시:
  grove jira weekly`,
	RunE: runJiraWeekly,
}

func init() {
	jiraCmd.AddCommand(jiraWeeklyCmd)
}

// jqlStatusList renders status names as a JQL quoted list, e.g. `"a", "b"`.
func jqlStatusList(statuses []string) string {
	parts := make([]string, 0, len(statuses))
	for _, s := range statuses {
		parts = append(parts, fmt.Sprintf("%q", s))
	}
	return strings.Join(parts, ", ")
}

// jqlNotIn returns ` AND status NOT IN (...)` for the given statuses, or "" when empty.
func jqlNotIn(statuses []string) string {
	if len(statuses) == 0 {
		return ""
	}
	return fmt.Sprintf(" AND status NOT IN (%s)", jqlStatusList(statuses))
}

func runJiraWeekly(cmd *cobra.Command, args []string) error {
	jiraClient, err := requireJiraClient()
	if err != nil {
		return err
	}

	wf := config.GetJiraWorkflow()

	// 완료(terminal) 상태 목록: 명시적 terminal + 해결종료
	var terminalStatuses []string
	terminalStatuses = append(terminalStatuses, wf.Statuses.Terminal...)
	if wf.Statuses.ResolvedClosed != "" {
		terminalStatuses = append(terminalStatuses, wf.Statuses.ResolvedClosed)
	}
	excludeClause := jqlNotIn(wf.ExcludedStatuses)
	excludeAndTerminalClause := jqlNotIn(append(append([]string{}, wf.ExcludedStatuses...), terminalStatuses...))

	now := time.Now()

	// 이번주 월요일 (startOfWeek)
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	thisMonday := now.AddDate(0, 0, -int(weekday-time.Monday))
	lastMonday := thisMonday.AddDate(0, 0, -7)

	thisMondayStr := thisMonday.Format("2006-01-02")
	lastMondayStr := lastMonday.Format("2006-01-02")

	// 지난주 이슈: 지난주에 업데이트된 내 이슈 (제외 상태 제외)
	lastWeekJQL := fmt.Sprintf(
		"assignee = currentUser() AND updatedDate >= \"%s\" AND updatedDate < \"%s\"%s ORDER BY status ASC",
		lastMondayStr, thisMondayStr, excludeClause,
	)

	// 이번주 이슈: 종료되지 않은 내 이슈 + 이번주 업데이트된 이슈
	thisWeekJQL := fmt.Sprintf(
		"assignee = currentUser() AND ((1=1%s) OR (updatedDate >= \"%s\"%s)) ORDER BY status ASC",
		excludeAndTerminalClause, thisMondayStr, excludeClause,
	)

	fmt.Fprintf(os.Stderr, "\n")

	// 지난주 이슈
	fmt.Fprintf(os.Stderr, "%s  %s ~ %s\n\n",
		cyanStyle.Render("📋 지난주 진행 이슈"),
		lastMondayStr,
		thisMonday.AddDate(0, 0, -1).Format("2006-01-02"),
	)

	stop := tui.Spinner("지난주 이슈 조회 중")
	lastWeekIssues, err := jiraClient.SearchIssues(url.QueryEscape(lastWeekJQL))
	stop()
	if err != nil {
		return fmt.Errorf("지난주 이슈 조회 실패: %w", err)
	}

	printWeeklyIssues(lastWeekIssues, wf)

	fmt.Fprintf(os.Stderr, "\n")

	// 이번주 이슈
	fmt.Fprintf(os.Stderr, "%s  %s ~\n\n",
		cyanStyle.Render("📋 이번주 이슈"),
		thisMondayStr,
	)

	stop = tui.Spinner("이번주 이슈 조회 중")
	thisWeekIssues, err := jiraClient.SearchIssues(url.QueryEscape(thisWeekJQL))
	stop()
	if err != nil {
		return fmt.Errorf("이번주 이슈 조회 실패: %w", err)
	}

	printWeeklyIssues(thisWeekIssues, wf)

	fmt.Fprintf(os.Stderr, "\n")

	return nil
}

// printWeeklyIssues prints issues split into two groups:
// 1. 진행중 이슈 (before 리뷰중) — plain table
// 2. 리뷰중 이후 이슈 — grouped by deploy date
func printWeeklyIssues(issues []jira.Issue, wf config.JiraWorkflow) {
	if len(issues) == 0 {
		fmt.Fprintf(os.Stderr, "  (이슈 없음)\n")
		return
	}

	var inProgress []jira.Issue
	deployGroups := make(map[string][]jira.Issue) // deployDate -> issues

	for _, issue := range issues {
		if wf.IsExcluded(issue.Status) {
			continue
		}
		if wf.IsReviewOrAfter(issue.Status) {
			dateKey := issue.DeployDate
			if dateKey == "" {
				dateKey = "(미정)"
			}
			deployGroups[dateKey] = append(deployGroups[dateKey], issue)
		} else {
			inProgress = append(inProgress, issue)
		}
	}

	if len(inProgress) > 0 {
		printIssueTable(inProgress)
	}

	if len(deployGroups) > 0 {
		// Sort deploy dates
		var dates []string
		for d := range deployGroups {
			dates = append(dates, d)
		}
		sort.Strings(dates)

		for _, date := range dates {
			fmt.Fprintf(os.Stderr, "\n  %s\n\n", yellowStyle.Render("🚀 배포일: "+date))
			printIssueTable(deployGroups[date])
		}
	}

	if len(inProgress) == 0 && len(deployGroups) == 0 {
		fmt.Fprintf(os.Stderr, "  (이슈 없음)\n")
	}
}

// printIssueTable prints issues in a formatted table.
func printIssueTable(issues []jira.Issue) {
	// 컬럼 헤더
	headers := []string{"#", "이슈 키", "상태", "제목"}

	// 각 컬럼의 최대 너비 계산
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = displayWidth(h)
	}

	type row struct {
		num, key, status, summary string
	}
	rows := make([]row, len(issues))

	for i, issue := range issues {
		r := row{
			num:     fmt.Sprintf("%d", i+1),
			key:     issue.Key,
			status:  issue.Status,
			summary: truncate(issue.Summary, 50),
		}
		rows[i] = r

		if w := displayWidth(r.num); w > colWidths[0] {
			colWidths[0] = w
		}
		if w := displayWidth(r.key); w > colWidths[1] {
			colWidths[1] = w
		}
		if w := displayWidth(r.status); w > colWidths[2] {
			colWidths[2] = w
		}
		if w := displayWidth(r.summary); w > colWidths[3] {
			colWidths[3] = w
		}
	}

	// 헤더 출력
	printRow(headers, colWidths)
	// 구분선
	seps := make([]string, len(colWidths))
	for i, w := range colWidths {
		seps[i] = strings.Repeat("-", w)
	}
	printRow(seps, colWidths)

	// 데이터 출력 (이슈 키에 터미널 하이퍼링크 적용)
	for _, r := range rows {
		printRow([]string{r.num, wrapHyperlink(r.key), r.status, r.summary}, colWidths)
	}
}

func printRow(cols []string, widths []int) {
	parts := make([]string, len(cols))
	for i, col := range cols {
		pad := widths[i] - displayWidth(col)
		if pad < 0 {
			pad = 0
		}
		parts[i] = col + strings.Repeat(" ", pad)
	}
	fmt.Fprintf(os.Stderr, "  | %s |\n", strings.Join(parts, " | "))
}

// wrapHyperlink wraps an issue key with OSC 8 terminal hyperlink if Jira server is configured.
// Unlike jiraIssueHyperlink, this does not apply color styles (used in table output).
func wrapHyperlink(issueKey string) string {
	server, _, _ := config.GetJiraConfig()
	if server == "" {
		return issueKey
	}
	url := fmt.Sprintf("%s/browse/%s", server, issueKey)
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, issueKey)
}

// stripOSC removes OSC 8 hyperlink sequences (\x1b]8;;...\x1b\\) from a string,
// keeping only the visible text.
func stripOSC(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		// Detect OSC 8 start: \x1b]8;
		if i+3 < len(s) && s[i] == 0x1b && s[i+1] == ']' && s[i+2] == '8' && s[i+3] == ';' {
			// Skip until ST (\x1b\\)
			j := i + 4
			for j < len(s)-1 {
				if s[j] == 0x1b && s[j+1] == '\\' {
					j += 2
					break
				}
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// displayWidth returns the display width considering CJK characters (width 2).
// OSC 8 hyperlink sequences are stripped before measuring.
func displayWidth(s string) int {
	s = stripOSC(s)
	w := 0
	for _, r := range s {
		if r >= 0x1100 && isWideRune(r) {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

func isWideRune(r rune) bool {
	return (r >= 0x1100 && r <= 0x115F) ||
		(r >= 0x2E80 && r <= 0x303E) ||
		(r >= 0x3040 && r <= 0x33BF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0xA4CF) ||
		(r >= 0xA960 && r <= 0xA97C) ||
		(r >= 0xAC00 && r <= 0xD7AF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE30 && r <= 0xFE6F) ||
		(r >= 0xFF01 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x2FFFD) ||
		(r >= 0x30000 && r <= 0x3FFFD)
}

// truncate truncates a string to maxWidth display width, appending "..." if truncated.
func truncate(s string, maxWidth int) string {
	if displayWidth(s) <= maxWidth {
		return s
	}
	w := 0
	for i, r := range s {
		rw := 1
		if r >= 0x1100 && isWideRune(r) {
			rw = 2
		}
		if w+rw > maxWidth-3 {
			return s[:i] + "..."
		}
		w += rw
		_ = utf8.RuneLen(r)
	}
	return s
}
