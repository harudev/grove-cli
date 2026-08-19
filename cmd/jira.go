package cmd

import (
	"github.com/spf13/cobra"
)

var jiraCmd = &cobra.Command{
	Use:   "jira",
	Short: "Jira 이슈 관련 명령어",
	Long: `Jira 이슈 상태 조회, 업데이트, 코멘트 추가 등을 수행합니다.

사용 예시:
  grove jira status                    # 현재 워크트리 이슈 상태 조회
  grove jira update                    # PR 상태 → Jira 이슈 상태 동기화
  grove jira weekly                    # 주간 이슈 현황 조회
  grove jira link                      # Jira 이슈 URL 출력
  grove jira qa -m "QA 내용"           # QA 코멘트 추가
  grove jira noqa                      # GitHub PR에 noqa 라벨 추가
  grove jira e2e                       # E2E 테스트 케이스를 Jira 코멘트로 추가
  grove jira deploy                    # 배포일자 설정 (화요일 선택)`,
}

func init() {
	rootCmd.AddCommand(jiraCmd)
}
