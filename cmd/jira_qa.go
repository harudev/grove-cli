package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/tui"
)

var jiraQACmd = &cobra.Command{
	Use:   "qa [ISSUE-KEY]",
	Short: "Jira 이슈에 QA 코멘트 추가",
	Long: `Jira 이슈에 QA 내용을 코멘트로 추가합니다.

사용 예시:
  grove jira qa -m "QA 완료. 정상 동작 확인."
  grove jira qa PROJ-12345 -m "로그인 플로우 테스트 완료"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runJiraQA,
}

func init() {
	jiraQACmd.Flags().StringP("message", "m", "", "QA 코멘트 내용 (필수)")
	jiraQACmd.MarkFlagRequired("message")
	jiraCmd.AddCommand(jiraQACmd)
}

func runJiraQA(cmd *cobra.Command, args []string) error {
	message, _ := cmd.Flags().GetString("message")

	issueKey, err := resolveIssueKey(args)
	if err != nil {
		return err
	}

	jiraClient, err := requireJiraClient()
	if err != nil {
		return err
	}

	comment := fmt.Sprintf("📋 QA\n\n%s", message)

	stop := tui.Spinner("QA 코멘트 추가 중")
	err = jiraClient.AddComment(issueKey, comment)
	stop()
	if err != nil {
		return fmt.Errorf("코멘트 추가 실패: %w", err)
	}

	fmt.Fprintf(os.Stderr, "%s %s에 QA 코멘트 추가 완료\n", "✅", cyanStyle.Render(issueKey))
	return nil
}
