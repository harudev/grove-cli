package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const issueContextDir = ".grove"
const claudeLocalMD = "CLAUDE.local.md"

// IssueContext holds Jira issue information to save as markdown.
type IssueContext struct {
	Key         string
	Summary     string
	Description string
	Status      string
	ParentKey   string
}

// SaveIssueContext writes the issue context as a markdown file in .grove/{issueKey}.md
// and appends a reference to CLAUDE.local.md.
func SaveIssueContext(worktreePath string, ctx IssueContext) error {
	// 1. Write .grove/{issueKey}.md
	groveDir := filepath.Join(worktreePath, issueContextDir)
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		return fmt.Errorf("create .grove dir: %w", err)
	}

	filename := ctx.Key + ".local.md"
	mdPath := filepath.Join(groveDir, filename)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s: %s\n\n", ctx.Key, ctx.Summary))
	if ctx.Description != "" {
		sb.WriteString(ctx.Description)
		sb.WriteString("\n")
	}

	if err := os.WriteFile(mdPath, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write issue context: %w", err)
	}

	// 2. Append reference to CLAUDE.local.md
	relPath := filepath.Join(issueContextDir, filename)
	reference := fmt.Sprintf("\n## 이슈 컨텍스트\n\n작업 시작 전 [%s](%s)를 읽고 구현 계획을 세우세요.\n",
		relPath, relPath)

	return appendClaudeLocalRef(worktreePath, relPath, reference)
}

// SaveIssueContextPlanning initialises a local planning context file with issue title.
func SaveIssueContextPlanning(worktreePath string, ctx IssueContext) error {
	filename := ctx.Key + ".local.md"

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s: %s\n\n", ctx.Key, ctx.Summary))
	sb.WriteString("## 현재 상황 파악\n\n")
	sb.WriteString("- [ ] 관련 코드 위치 확인\n")
	sb.WriteString("- [ ] 재현 조건 정리\n")
	sb.WriteString("- [ ] 영향 범위 확인\n\n")
	sb.WriteString("## 구현 계획\n\n")
	sb.WriteString("### 접근 방식\n\n\n\n")
	sb.WriteString("### 변경 대상 파일\n\n-\n\n")
	sb.WriteString("### 확인이 필요한 사항\n\n-\n\n")
	sb.WriteString("## 하위 작업\n\n")
	sb.WriteString("1.\n")

	relPath := filepath.Join(issueContextDir, filename)
	reference := fmt.Sprintf(`
## 이슈 컨텍스트

작업 시작 전 함께 이슈 계획을 세우고 [%s](%s)에 저장합니다.

### 작업 흐름

1. 계획 수립: 현재 상황 파악 → 구현 계획 작성 → 사용자 확인
2. 구현

다수의 하위 작업으로 나뉠 경우 JIRA 이슈에 하위 이슈를 생성할지 묻습니다.
`, relPath, relPath)

	return writePlanningFile(worktreePath, filename, sb.String(), relPath, reference)
}

// SaveIssueContextPlanningTDD initialises a local planning context file with issue title
// and TDD workflow (TC first, then implement).
func SaveIssueContextPlanningTDD(worktreePath string, ctx IssueContext) error {
	filename := ctx.Key + ".local.md"

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s: %s\n\n", ctx.Key, ctx.Summary))
	sb.WriteString("## 현재 상황 파악\n\n")
	sb.WriteString("- [ ] 관련 코드 위치 확인\n")
	sb.WriteString("- [ ] 재현 조건 정리\n")
	sb.WriteString("- [ ] 영향 범위 확인\n\n")
	sb.WriteString("## 구현 계획\n\n")
	sb.WriteString("### 접근 방식\n\n\n\n")
	sb.WriteString("### 변경 대상 파일\n\n-\n\n")
	sb.WriteString("### 확인이 필요한 사항\n\n-\n\n")
	sb.WriteString("## 테스트 케이스\n\n")
	sb.WriteString("각 작업 항목에 대한 TC를 먼저 작성한 뒤 구현을 시작합니다.\n\n")
	sb.WriteString("-\n\n")
	sb.WriteString("## 하위 작업\n\n")
	sb.WriteString("1.\n")

	relPath := filepath.Join(issueContextDir, filename)
	reference := fmt.Sprintf(`
## 이슈 컨텍스트

작업 시작 전 함께 이슈 계획을 세우고 [%s](%s)에 저장합니다.

### 작업 흐름

1. 계획 수립: 현재 상황 파악 → 구현 계획 작성 → 사용자 확인
2. TC 작성: 각 작업 항목에 대한 테스트 케이스를 먼저 작성
3. TC 리뷰: 사용자에게 TC 리뷰 요청, TC가 승인되면 계획 완료
4. 구현: TC를 통과하도록 코드 작성

다수의 하위 작업으로 나뉠 경우 JIRA 이슈에 하위 이슈를 생성할지 묻습니다.
`, relPath, relPath)

	return writePlanningFile(worktreePath, filename, sb.String(), relPath, reference)
}

// SaveIssueContextPlanningPostTC initialises a local planning context file with issue title;
// test cases are written after implementation for verification.
func SaveIssueContextPlanningPostTC(worktreePath string, ctx IssueContext) error {
	filename := ctx.Key + ".local.md"

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s: %s\n\n", ctx.Key, ctx.Summary))
	sb.WriteString("## 현재 상황 파악\n\n")
	sb.WriteString("- [ ] 관련 코드 위치 확인\n")
	sb.WriteString("- [ ] 재현 조건 정리\n")
	sb.WriteString("- [ ] 영향 범위 확인\n\n")
	sb.WriteString("## 구현 계획\n\n")
	sb.WriteString("### 접근 방식\n\n\n\n")
	sb.WriteString("### 변경 대상 파일\n\n-\n\n")
	sb.WriteString("### 확인이 필요한 사항\n\n-\n\n")
	sb.WriteString("## 하위 작업\n\n")
	sb.WriteString("1.\n")

	relPath := filepath.Join(issueContextDir, filename)
	reference := fmt.Sprintf(`
## 이슈 컨텍스트

작업 시작 전 함께 이슈 계획을 세우고 [%s](%s)에 저장합니다.

### 작업 흐름

1. 계획 수립: 현재 상황 파악 → 구현 계획 작성 → 사용자 확인
2. 구현
3. TC 작성: 구현 완료 후 동작 확인용 테스트 케이스 작성

다수의 하위 작업으로 나뉠 경우 JIRA 이슈에 하위 이슈를 생성할지 묻습니다.
`, relPath, relPath)

	return writePlanningFile(worktreePath, filename, sb.String(), relPath, reference)
}

// writePlanningFile writes the planning markdown and appends reference to CLAUDE.local.md.
func writePlanningFile(worktreePath, filename, content, relPath, reference string) error {
	groveDir := filepath.Join(worktreePath, issueContextDir)
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		return fmt.Errorf("create .grove dir: %w", err)
	}

	mdPath := filepath.Join(groveDir, filename)
	if err := os.WriteFile(mdPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write issue context: %w", err)
	}

	return appendClaudeLocalRef(worktreePath, relPath, reference)
}

// appendClaudeLocalRef appends a reference block to CLAUDE.local.md if not already present.
func appendClaudeLocalRef(worktreePath, relPath, reference string) error {
	claudePath := filepath.Join(worktreePath, claudeLocalMD)

	// Check if CLAUDE.local.md already references this issue
	existing, _ := os.ReadFile(claudePath)
	if strings.Contains(string(existing), relPath) {
		return nil // already referenced
	}

	f, err := os.OpenFile(claudePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open CLAUDE.local.md: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(reference); err != nil {
		return fmt.Errorf("write CLAUDE.local.md: %w", err)
	}

	return nil
}
