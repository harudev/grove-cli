package cmd

import (
	"github.com/harudev/grove-cli/internal/config"
	"github.com/spf13/cobra"
)

var appVersion = "dev"

// SetVersion sets the application version (called from main).
func SetVersion(v string) {
	appVersion = v
}

var rootCmd = &cobra.Command{
	Use:   "grove",
	Short: "Git worktree 관리 CLI",
	Long: `grove는 Git worktree를 생성하고 관리하는 CLI 도구입니다.
Jira 이슈, GitHub PR URL을 기반으로 워크트리를 자동 생성합니다.

사용 예시:
  grove setup                                  # 초기 설정 (도구 확인)
  grove config                                 # 설정 조회/변경
  grove config jira                            # Jira API 토큰 설정
  grove config local-files --add ".env.local"  # 로컬 파일 복사 패턴 추가
  grove init PROJ-12345                        # 풀 이슈키로 워크트리 생성
  grove init 12345 -t feature                  # 숫자만 (기본 prefix 사용)
  grove init https://github.com/org/repo/pull/456
  grove list
  grove clean --done
  grove jira status                            # Jira 이슈 상태 조회
  grove jira update                            # PR 상태 → Jira 이슈 상태 동기화
  grove jira link -o                           # Jira 이슈 브라우저에서 열기
  grove jira qa -m "QA 내용"                   # QA 코멘트 추가
  grove jira noqa                              # PR에 noqa 라벨 추가
  grove jira e2e                               # E2E TC를 Jira 코멘트로 추가`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// One-time migration of legacy global settings (issue prefix, local file
		// patterns) into the current repo's config. Safe no-op outside a repo or
		// once migrated.
		config.MigrateCurrentRepo()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if cmd.Name() == "update" || cmd.Name() == "check-update" {
			return
		}
		showUpdateNoticeIfNeeded()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
