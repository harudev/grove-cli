package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/github"
	"github.com/harudev/grove-cli/internal/tui"
	"github.com/harudev/grove-cli/internal/worktree"
)

var initCmd = &cobra.Command{
	Use:   "init [issue-number|jira-url|pr-url]",
	Short: "워크트리 생성",
	Long: `이슈 번호, Jira URL, 또는 GitHub PR URL을 기반으로 워크트리를 생성합니다.

사용 예시:
  grove init PROJ-12345                        # 풀 이슈키
  grove init 12345                             # 숫자만 (기본 prefix 사용)
  grove init PROJ-12345 -t feature             # 브랜치 타입 지정
  grove init PROJ-12345 -t bugfix -b develop   # base 브랜치 직접 지정
  grove init PROJ-12345 -t hotfix -b v1.2.0    # base 태그 직접 지정
  grove init PROJ-12345 -b                      # base 를 목록에서 검색·선택
  grove init https://your-jira.atlassian.net/browse/PROJ-12345
  grove init https://github.com/org/repo/pull/456`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringP("type", "t", "", "브랜치 타입 (config 정책에 정의된 타입)")
	initCmd.Flags().StringP("base", "b", "", "base 브랜치/태그 직접 지정 (값 없이 -b 만 주면 목록에서 검색·선택)")
	// 값 없이 -b/--base 만 주면 base 브랜치/태그를 대화형으로 선택한다.
	initCmd.Flags().Lookup("base").NoOptDefVal = worktree.SelectBaseRef
	initCmd.Flags().Bool("no-install", false, "의존성 설치 건너뛰기")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	branchType, _ := cmd.Flags().GetString("type")
	baseRef, _ := cmd.Flags().GetString("base")
	noInstall, _ := cmd.Flags().GetBool("no-install")

	repoDir, err := os.Getwd()
	if err != nil {
		return err
	}

	engine := &worktree.Engine{
		RepoDir:  repoDir,
		Jira:     newJiraAdapter(),
		GH:       github.NewGHCLIClient(),
		Prompter: tui.NewInteractivePrompter(),
	}

	opts := worktree.InitOptions{
		Input:      args[0],
		BranchType: branchType,
		BaseRef:    baseRef,
		NoInstall:  noInstall,
	}

	_, err = engine.Init(opts)
	return err
}
