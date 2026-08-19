package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/tui"
)

var jiraStatusCmd = &cobra.Command{
	Use:   "status [ISSUE-KEY]",
	Short: "Jira 이슈 상태 조회",
	Long: `Jira 이슈의 현재 상태와 요약 정보를 조회합니다.

사용 예시:
  grove jira status                    # 현재 워크트리 이슈
  grove jira status PROJ-12345       # 특정 이슈
  grove jira status 12345              # 숫자만`,
	Args: cobra.MaximumNArgs(1),
	RunE: runJiraStatus,
}

func init() {
	jiraCmd.AddCommand(jiraStatusCmd)
}

func runJiraStatus(cmd *cobra.Command, args []string) error {
	issueKey, err := resolveIssueKey(args)
	if err != nil {
		return err
	}

	jiraClient, err := requireJiraClient()
	if err != nil {
		return err
	}

	stop := tui.Spinner("이슈 조회 중")
	issue, err := jiraClient.ViewIssue(issueKey)
	stop()
	if err != nil {
		return fmt.Errorf("이슈 조회 실패: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  이슈:   %s\n", cyanStyle.Render(issue.Key))
	fmt.Fprintf(os.Stderr, "  제목:   %s\n", issue.Summary)
	fmt.Fprintf(os.Stderr, "  상태:   %s\n", issue.Status)
	if issue.ParentKey != "" {
		fmt.Fprintf(os.Stderr, "  부모:   %s\n", issue.ParentKey)
	}
	fmt.Fprintf(os.Stderr, "\n")

	return nil
}
