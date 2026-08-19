package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/e2eparse"
	"github.com/harudev/grove-cli/internal/tui"
)

var jiraE2ECmd = &cobra.Command{
	Use:   "e2e [ISSUE-KEY]",
	Short: "E2E 테스트 케이스를 파싱하여 Jira 코멘트로 추가",
	Long: `현재 디렉토리에서 E2E 테스트 파일(*.e2e.ts, *.spec.ts, e2e/**/*.ts)을 찾아
describe/it/test 블록을 추출하고 Jira 이슈에 코멘트로 추가합니다.

사용 예시:
  grove jira e2e                       # 현재 워크트리 이슈
  grove jira e2e PROJ-12345
  grove jira e2e --dry-run             # 미리보기 (코멘트 안 남김)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runJiraE2E,
}

func init() {
	jiraE2ECmd.Flags().Bool("dry-run", false, "코멘트를 남기지 않고 내용만 미리보기")
	jiraCmd.AddCommand(jiraE2ECmd)
}

func runJiraE2E(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	issueKey, err := resolveIssueKey(args)
	if err != nil {
		return err
	}

	repoDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Parse e2e files
	files, err := e2eparse.ParseDir(repoDir)
	if err != nil {
		return fmt.Errorf("E2E 파일 파싱 실패: %w", err)
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "%s E2E 테스트 파일을 찾을 수 없습니다\n", yellowStyle.Render("⚠"))
		return nil
	}

	comment := e2eparse.FormatAsComment(files)

	if dryRun {
		fmt.Fprintf(os.Stderr, "%s [dry-run] %s 코멘트 미리보기:\n\n", cyanStyle.Render("→"), cyanStyle.Render(issueKey))
		fmt.Fprintln(os.Stderr, comment)
		return nil
	}

	jiraClient, err := requireJiraClient()
	if err != nil {
		return err
	}

	stop := tui.Spinner("E2E 코멘트 추가 중")
	err = jiraClient.AddComment(issueKey, comment)
	stop()
	if err != nil {
		return fmt.Errorf("코멘트 추가 실패: %w", err)
	}

	fmt.Fprintf(os.Stderr, "%s %s에 E2E 테스트 케이스 코멘트 추가 완료 (%d개 파일)\n",
		"✅", cyanStyle.Render(issueKey), len(files))
	return nil
}
