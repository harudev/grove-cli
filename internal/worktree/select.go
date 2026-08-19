package worktree

import (
	"fmt"
	"regexp"

	"github.com/harudev/grove-cli/internal/git"
	"github.com/harudev/grove-cli/internal/tui"
)

// SelectOptions holds options for the select flow.
type SelectOptions struct {
	Target string // issue number or key (e.g. "12345" or "PROJ-12345")
}

var numericTarget = regexp.MustCompile(`^\d+$`)

// matchRegex builds the branch-matching regex for a select target.
//
// A bare number ("169") matches any issue key ending in that number, regardless
// of the repo's prefix — so `ow 169` finds feature/TEAM-169, feature/PROJ-169, etc.
// A full key ("TEAM-169") matches that key directly, case-insensitively.
func matchRegex(target string) *regexp.Regexp {
	if numericTarget.MatchString(target) {
		// `-169` not followed by another digit (avoids matching TEAM-1690).
		return regexp.MustCompile(`-0*` + target + `(\D|$)`)
	}
	return regexp.MustCompile(`(?i)` + regexp.QuoteMeta(target))
}

// Select finds matching worktrees and returns the selected one's path.
// If multiple worktrees match, presents an interactive selection UI.
func Select(repoDir string, prompter tui.Prompter, opts SelectOptions) error {
	if opts.Target == "" {
		return fmt.Errorf("target 이슈를 지정해주세요")
	}

	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		return err
	}

	re := matchRegex(opts.Target)
	var matchedWorktrees []git.WorktreeInfo
	for _, wt := range worktrees {
		if re.MatchString(wt.Branch) {
			matchedWorktrees = append(matchedWorktrees, wt)
		}
	}

	if len(matchedWorktrees) == 0 {
		return fmt.Errorf("워크트리를 찾을 수 없습니다: %s", opts.Target)
	}

	selected := matchedWorktrees[0]
	if len(matchedWorktrees) > 1 {
		// Multiple worktrees found, let user select
		var branchOptions []string
		for _, wt := range matchedWorktrees {
			branchOptions = append(branchOptions, wt.Branch)
		}

		selectedBranch, err := prompter.SelectWorktree(branchOptions)
		if err != nil {
			return err
		}

		for _, wt := range matchedWorktrees {
			if wt.Branch == selectedBranch {
				selected = wt
				break
			}
		}
	}

	// Output the path to stdout for shell function to use
	fmt.Println(selected.Path)
	return nil
}
