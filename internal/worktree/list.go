package worktree

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/fileutil"
	"github.com/harudev/grove-cli/internal/git"
)

// WorktreeListEntry represents a single entry in the worktree list.
type WorktreeListEntry struct {
	Index    int    `json:"index"`
	IssueKey string `json:"issueKey"`
	Title    string `json:"title"`
	Phase    string `json:"phase"`
	Branch   string `json:"branch"`
	Path     string `json:"path"`
}

// List lists all worktrees with issue information.
func List(repoDir string, asJSON bool) error {
	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		return fmt.Errorf("워크트리 목록 조회 실패: %w", err)
	}

	mainRepo, err := git.GetRepoRoot(repoDir)
	if err != nil {
		return err
	}

	issueRe := config.IssueKeyExtractRegex()
	var entries []WorktreeListEntry
	counter := 0

	for _, wt := range worktrees {
		if wt.Path == mainRepo {
			continue
		}

		counter++
		issueKey := "-"
		if m := issueRe.FindString(wt.Branch); m != "" {
			issueKey = m
		}

		title := "-"
		if t, err := fileutil.ReadIssueTitle(wt.Path); err == nil && t != "" {
			title = truncate(t, 50)
		}

		phase := "-"
		if issueKey != "-" {
			if p, err := fileutil.ReadPhase(wt.Path, issueKey); err == nil && p != "" {
				phase = p
			}
		}

		entries = append(entries, WorktreeListEntry{
			Index:    counter,
			IssueKey: issueKey,
			Title:    title,
			Phase:    phase,
			Branch:   wt.Branch,
			Path:     wt.Path,
		})
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	fmt.Println("## 워크트리 목록")
	fmt.Println()
	fmt.Println("| # | 이슈 번호 | 이슈 제목 | 상태 |")
	fmt.Println("|---|-----------|-----------|------|")

	for _, e := range entries {
		fmt.Printf("| %d | %s | %s | %s |\n", e.Index, e.IssueKey, e.Title, e.Phase)
	}

	fmt.Println()
	fmt.Printf("총 워크트리: %d개 (메인 저장소 포함)\n", len(worktrees))

	return nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}
