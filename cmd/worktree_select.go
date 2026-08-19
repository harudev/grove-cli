package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/tui"
	"github.com/harudev/grove-cli/internal/worktree"
)

var worktreeSelectCmd = &cobra.Command{
	Use:   "select <ISSUE-KEY>",
	Short: "워크트리 선택 및 경로 출력 (shell function용)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Target may be a bare issue number ("169") or a full key with an
		// explicit prefix ("TEAM-169"). Matching is handled in worktree.Select.
		repoDir, err := os.Getwd()
		if err != nil {
			return err
		}

		prompter := tui.NewInteractivePrompter()
		opts := worktree.SelectOptions{
			Target: args[0],
		}

		return worktree.Select(repoDir, prompter, opts)
	},
}

func init() {
	rootCmd.AddCommand(worktreeSelectCmd)
}
