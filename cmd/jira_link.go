package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/config"
)

var jiraLinkCmd = &cobra.Command{
	Use:   "link [ISSUE-KEY]",
	Short: "Jira 이슈 URL 출력 또는 브라우저에서 열기",
	Long: `Jira 이슈 URL을 출력하거나 브라우저에서 엽니다.

사용 예시:
  grove jira link                      # URL 출력
  grove jira link -o                   # 브라우저에서 열기
  grove jira link PROJ-12345 -o`,
	Args: cobra.MaximumNArgs(1),
	RunE: runJiraLink,
}

func init() {
	jiraLinkCmd.Flags().BoolP("open", "o", false, "브라우저에서 열기")
	jiraCmd.AddCommand(jiraLinkCmd)
}

func runJiraLink(cmd *cobra.Command, args []string) error {
	openBrowser, _ := cmd.Flags().GetBool("open")

	issueKey, err := resolveIssueKey(args)
	if err != nil {
		return err
	}

	server, _, _ := config.GetJiraConfig()
	if server == "" {
		return fmt.Errorf("Jira 서버가 설정되지 않았습니다. grove config jira를 실행하세요")
	}

	url := fmt.Sprintf("%s/browse/%s", server, issueKey)

	if openBrowser {
		fmt.Fprintf(os.Stderr, "%s %s\n", greenStyle.Render("🔗"), url)
		return exec.Command("open", url).Run()
	}

	fmt.Println(url)
	return nil
}
