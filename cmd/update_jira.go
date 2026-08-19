package cmd

import (
	"github.com/spf13/cobra"
)

// Deprecated: grove jira update를 사용하세요.
var updateJiraCmd = &cobra.Command{
	Use:        "update-jira [ISSUE-KEY]",
	Short:      "[deprecated] grove jira update를 사용하세요",
	Hidden:     true,
	Deprecated: "grove jira update를 사용하세요",
	Args:       cobra.MaximumNArgs(1),
	RunE:       runJiraUpdate,
}

func init() {
	updateJiraCmd.Flags().Bool("dry-run", false, "실제 변경 없이 전이 내용만 확인")
	updateJiraCmd.Flags().Bool("all", false, "모든 워크트리 대상으로 일괄 실행")
	rootCmd.AddCommand(updateJiraCmd)
}
