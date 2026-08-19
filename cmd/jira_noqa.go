package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/tui"
)

var jiraNoQACmd = &cobra.Command{
	Use:   "noqa [ISSUE-KEY]",
	Short: "Jira 이슈에 noqa 라벨 추가",
	Long: `현재 이슈의 Jira에 noqa 라벨을 추가합니다.
noqa 라벨이 있으면 grove jira update 시 리뷰중을 건너뛰고 리뷰완료로 전이됩니다.

사용 예시:
  grove jira noqa                      # 현재 워크트리 이슈
  grove jira noqa PROJ-12345`,
	Args: cobra.MaximumNArgs(1),
	RunE: runJiraNoQA,
}

func init() {
	jiraCmd.AddCommand(jiraNoQACmd)
}

func runJiraNoQA(cmd *cobra.Command, args []string) error {
	issueKey, err := resolveIssueKey(args)
	if err != nil {
		return err
	}

	jiraClient, err := requireJiraClient()
	if err != nil {
		return err
	}

	stop := tui.Spinner("noqa 라벨 추가 중")
	err = jiraClient.AddLabel(issueKey, "noqa")
	stop()
	if err != nil {
		return fmt.Errorf("라벨 추가 실패: %w", err)
	}

	fmt.Fprintf(os.Stderr, "%s %s에 noqa 라벨 추가 완료\n", "✅", cyanStyle.Render(issueKey))
	return nil
}
