package cmd

import (
	"os"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/tui"
	"github.com/harudev/grove-cli/internal/worktree"
)

var worktreeCleanCmd = &cobra.Command{
	Use:   "clean [ISSUE-KEY]",
	Short: "완료된 이슈의 워크트리 정리",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		doneMode, _ := cmd.Flags().GetBool("done")
		force, _ := cmd.Flags().GetBool("force")

		repoDir, err := os.Getwd()
		if err != nil {
			return err
		}

		jiraClient := newJiraAdapter()
		prompter := tui.NewInteractivePrompter()

		opts := worktree.CleanOptions{
			DoneMode: doneMode,
			Force:    force,
		}

		if len(args) > 0 {
			arg := args[0]
			// Numeric only → prepend configured prefix
			if matched, _ := regexp.MatchString(`^\d+$`, arg); matched {
				arg = config.FormatIssueKey(arg)
			}
			opts.IssueKey = arg
		}

		return worktree.Clean(repoDir, jiraClient, prompter, opts)
	},
}

func init() {
	worktreeCleanCmd.Flags().Bool("done", false, "Jira 리뷰중 이후 상태인 워크트리 일괄 정리")
	worktreeCleanCmd.Flags().BoolP("force", "f", false, "미커밋 변경사항 무시하고 강제 정리")
	rootCmd.AddCommand(worktreeCleanCmd)
}
