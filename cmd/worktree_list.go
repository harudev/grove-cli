package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/worktree"
)

var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "현재 워크트리 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		repoDir, err := os.Getwd()
		if err != nil {
			return err
		}
		return worktree.List(repoDir, asJSON)
	},
}

func init() {
	worktreeListCmd.Flags().Bool("json", false, "JSON 형식으로 출력")
	rootCmd.AddCommand(worktreeListCmd)
}
