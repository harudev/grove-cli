package worktree

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/fileutil"
	"github.com/harudev/grove-cli/internal/git"
	"github.com/harudev/grove-cli/internal/github"
	"github.com/harudev/grove-cli/internal/issue"
	"github.com/harudev/grove-cli/internal/jira"
	"github.com/harudev/grove-cli/internal/tui"
)

// SelectBaseRef is the sentinel BaseRef value meaning "prompt me to pick a base
// ref interactively". It is set when --base is passed without an explicit value.
const SelectBaseRef = "\x00grove:select-base"

// InitOptions holds options for the init flow.
type InitOptions struct {
	Input      string // raw user input
	BranchType string // --type flag
	BaseRef    string // --base flag (pre-selected base branch/tag for from-branch/from-tag)
	NoInstall  bool   // --no-install flag
}

// InitResult holds the result of the init flow.
type InitResult struct {
	WorktreePath string
	BranchName   string
	BaseBranch   string
	BranchType   string
	IssueKey     string
}

// Engine orchestrates the init flow.
type Engine struct {
	RepoDir  string
	Jira     jira.Adapter
	GH       github.PRClient
	Prompter tui.Prompter
}

// Init runs the full init flow.
func (e *Engine) Init(opts InitOptions) (*InitResult, error) {
	// 1. Parse input
	parsed, err := issue.Parse(opts.Input)
	if err != nil {
		return nil, err
	}

	// 2. Resolve PR URL → issue key
	var prURL string
	if parsed.Type == issue.InputPRURL {
		prURL = parsed.PRURL
		if parsed.IssueKey == nil {
			prInfo, err := e.GH.View(prURL)
			if err != nil {
				return nil, fmt.Errorf("PR 정보를 가져올 수 없습니다: %w", err)
			}
			key, err := extractIssueFromBranch(prInfo.HeadRefName)
			if err != nil {
				return nil, fmt.Errorf("PR head branch에서 이슈 번호를 추출할 수 없습니다: %s", prInfo.HeadRefName)
			}
			parsed.IssueKey = key
			logInfo("PR에서 이슈 번호 자동 추출: %s", parsed.IssueKey)
		}
	}

	issueKey := parsed.IssueKey.String()
	issueNum := parsed.IssueKey.Number

	// 3. Setup worktree paths
	_, worktreePath, err := git.SetupWorktreePath(e.RepoDir, issueKey)
	if err != nil {
		return nil, err
	}

	// 4. Detect remote. The base branch is resolved lazily per branch type
	//    (config override → auto-detect → interactive setup) once the type is known.
	remote, err := git.DetectRemote(e.RepoDir)
	if err != nil {
		return nil, err
	}

	// 5. Check existing worktree (fast path)
	if opts.BranchType == "" {
		exists, err := git.WorktreeExists(e.RepoDir, worktreePath)
		if err == nil && exists {
			logInfo("기존 워크트리 발견: %s", worktreePath)
			fmt.Println(worktreePath)
			logInfo("")
			logInfo("or")
			logInfo("")
			logInfo("  ow %s", issueNum)
			return &InitResult{WorktreePath: worktreePath, IssueKey: issueKey}, nil
		}
	}

	logBold("🚀 Jira 워크트리 초기화")
	logInfo("이슈: %s", issueKey)

	var result *InitResult

	if prURL != "" {
		result, err = e.initFromPR(prURL, issueKey, worktreePath, remote)
	} else {
		result, err = e.initFromBranchType(opts, issueKey, issueNum, worktreePath, remote)
	}
	if err != nil {
		return nil, err
	}

	// 6. Post-setup
	if err := e.postSetup(result, remote, opts.NoInstall); err != nil {
		return nil, err
	}

	logSuccess("워크트리 초기화 완료!")
	logInfo("📁 워크트리: %s", result.WorktreePath)
	logInfo("🔀 브랜치: %s", result.BranchName)
	if result.BaseBranch != "" {
		logInfo("📋 Base: %s", result.BaseBranch)
	}

	fmt.Println(result.WorktreePath)
	logInfo("")
	logInfo("or")
	logInfo("")
	logInfo("  ow %s", issueNum)
	return result, nil
}

// initFromPR handles PR-based worktree creation.
func (e *Engine) initFromPR(prURL, issueKey, worktreePath, remote string) (*InitResult, error) {
	stop := spinner("📌 PR 기반 워크트리 생성")

	prInfo, err := e.GH.View(prURL)
	if err != nil {
		stop()
		return nil, fmt.Errorf("PR 정보를 가져올 수 없습니다: %w", err)
	}

	headBranch := prInfo.HeadRefName
	branchType := config.BranchTypeFromName(e.RepoDir, headBranch)

	stop()
	logInfo("PR Head: %s", headBranch)
	logInfo("PR Base: %s", prInfo.BaseRefName)

	stop = spinner("📌 워크트리 생성 중")

	if err := git.FetchAll(e.RepoDir); err != nil {
		stop()
		return nil, err
	}

	exists, _ := git.WorktreeExists(e.RepoDir, worktreePath)
	if exists {
		stop()
		logWarn("워크트리가 이미 존재합니다: %s", worktreePath)
	} else {
		// Create worktree from the PR head branch
		if git.BranchExists(e.RepoDir, headBranch) {
			if err := git.WorktreeAddExisting(e.RepoDir, worktreePath, headBranch); err != nil {
				stop()
				return nil, fmt.Errorf("워크트리 생성 실패: %w", err)
			}
		} else if git.BranchExistsRemote(e.RepoDir, remote, headBranch) {
			if err := git.WorktreeAddFromStartPoint(e.RepoDir, worktreePath, headBranch,
				fmt.Sprintf("%s/%s", remote, headBranch)); err != nil {
				stop()
				return nil, fmt.Errorf("워크트리 생성 실패: %w", err)
			}
		} else {
			stop()
			return nil, fmt.Errorf("브랜치를 찾을 수 없습니다: %s", headBranch)
		}
		stop()
		logSuccess("워크트리 생성 완료")
	}

	return &InitResult{
		WorktreePath: worktreePath,
		BranchName:   headBranch,
		BaseBranch:   prInfo.BaseRefName,
		BranchType:   branchType,
		IssueKey:     issueKey,
	}, nil
}

// initFromBranchType selects a branch type and dispatches on its strategy.
func (e *Engine) initFromBranchType(opts InitOptions, issueKey, issueNum, worktreePath, remote string) (*InitResult, error) {
	defs := config.ResolveBranchTypes(e.RepoDir)
	branchType := opts.BranchType

	// Interactive branch type selection
	if branchType == "" {
		infos := make([]tui.BranchTypeInfo, len(defs))
		for i, d := range defs {
			infos[i] = tui.BranchTypeInfo{Type: tui.BranchType(d.Name), Description: d.Description}
		}
		bt, err := e.Prompter.SelectBranchType(infos)
		if err != nil {
			return nil, err
		}
		branchType = string(bt)
	}

	def, ok := config.GetBranchTypeDef(e.RepoDir, branchType)
	if !ok {
		return nil, fmt.Errorf("알 수 없는 브랜치 타입: %s (가능: %s)",
			branchType, strings.Join(config.BranchTypeNames(e.RepoDir), ", "))
	}

	switch def.Strategy {
	case config.StrategyFromDefault:
		return e.initFromDefault(def, opts, issueKey, worktreePath, remote)
	case config.StrategyFromBranch:
		return e.initFromBranch(def, opts, issueKey, worktreePath, remote)
	case config.StrategyFromTag:
		return e.initFromTag(def, opts, issueKey, worktreePath, remote)
	case config.StrategyCheckoutExisting:
		return e.initLocalReview(issueKey, issueNum, worktreePath, remote)
	default:
		return nil, fmt.Errorf("알 수 없는 전략: %s (타입 %s)", def.Strategy, def.Name)
	}
}

// initFromDefault creates a worktree based off the repo's default branch
// (pinned config, --base override, or auto-detected default).
func (e *Engine) initFromDefault(def config.BranchTypeDef, opts InitOptions, issueKey, worktreePath, remote string) (*InitResult, error) {
	base := opts.BaseRef // explicit --base wins over the pinned config base
	if base == SelectBaseRef {
		b, err := e.selectBranchRef(fmt.Sprintf("%s: base 브랜치 선택", def.Name), remote)
		if err != nil {
			return nil, err
		}
		base = b
	}
	if base == "" {
		base = def.Base
	}
	if base == "" {
		b, err := e.resolveDefaultBranch(def.Name, remote)
		if err != nil {
			return nil, err
		}
		base = b
	}
	branchName := config.FormatBranchNameRepo(e.RepoDir, def.Name, issueKey)
	return e.createWorktreeFromBase(def.Name, base, branchName, issueKey, worktreePath, remote)
}

// resolveDefaultBranch auto-detects the repo default branch, prompting the user to
// pick (and pinning the choice) when detection fails.
func (e *Engine) resolveDefaultBranch(typeName, remote string) (string, error) {
	if b, err := git.DetectDefaultBranch(e.RepoDir, remote); err == nil {
		return b, nil
	}
	logWarn("기본 브랜치를 자동 감지하지 못했습니다 (%s). 직접 선택해주세요.",
		strings.Join(config.DefaultBranchCandidates, "/"))
	base, err := e.selectBranchRef(fmt.Sprintf("%s: 기본 브랜치 선택", typeName), remote)
	if err != nil {
		return "", err
	}
	if err := config.SetBranchTypeBase(e.RepoDir, typeName, base); err != nil {
		logWarn("기본 브랜치 저장 실패: %v", err)
	}
	return base, nil
}

// initFromBranch creates a worktree based off a branch (pinned, --base, or picked).
func (e *Engine) initFromBranch(def config.BranchTypeDef, opts InitOptions, issueKey, worktreePath, remote string) (*InitResult, error) {
	base := opts.BaseRef // explicit --base wins over the pinned config base
	if base == "" {
		base = def.Base
	}
	if base == "" || base == SelectBaseRef {
		b, err := e.selectBranchRef(fmt.Sprintf("%s: base 브랜치 선택", def.Name), remote)
		if err != nil {
			return nil, err
		}
		base = b
	}
	branchName := config.FormatBranchNameRepo(e.RepoDir, def.Name, issueKey)
	return e.createWorktreeFromBase(def.Name, base, branchName, issueKey, worktreePath, remote)
}

// initFromTag creates a worktree based off a git tag (pinned, --base, or picked).
func (e *Engine) initFromTag(def config.BranchTypeDef, opts InitOptions, issueKey, worktreePath, remote string) (*InitResult, error) {
	tag := opts.BaseRef // explicit --base wins over the pinned config base
	if tag == "" {
		tag = def.Base
	}
	if tag == "" || tag == SelectBaseRef {
		if err := git.Fetch(e.RepoDir, "", true); err != nil {
			logWarn("태그 fetch 실패, 현재 알고 있는 태그로 진행합니다")
		}
		tags, err := git.ListTagsByDate(e.RepoDir)
		if err != nil {
			return nil, fmt.Errorf("태그 목록 조회 실패: %w", err)
		}
		t, err := e.Prompter.SelectRef(fmt.Sprintf("%s: base 태그 선택", def.Name), tags)
		if err != nil {
			return nil, err
		}
		tag = t
	}
	if !git.TagExists(e.RepoDir, tag) {
		return nil, fmt.Errorf("태그를 찾을 수 없습니다: %s", tag)
	}
	branchName := config.FormatBranchNameRepo(e.RepoDir, def.Name, issueKey)
	return e.createWorktreeFromTag(def.Name, tag, branchName, issueKey, worktreePath, remote)
}

// selectBranchRef fetches and prompts for a branch (newest-first, searchable).
func (e *Engine) selectBranchRef(title, remote string) (string, error) {
	if err := git.FetchAll(e.RepoDir); err != nil {
		logWarn("fetch 실패, 현재 알고 있는 브랜치로 진행합니다")
	}
	branches, err := git.ListBranchesByDate(e.RepoDir, remote)
	if err != nil {
		return "", fmt.Errorf("브랜치 목록 조회 실패: %w", err)
	}
	return e.Prompter.SelectRef(title, branches)
}

// createWorktreeFromBase checks out and pulls the base branch, then creates a worktree
// for a freshly created branch off it. Shared by feature/big-feature/bugfix/hotfix.
func (e *Engine) createWorktreeFromBase(branchType, baseBranch, branchName, issueKey, worktreePath, remote string) (*InitResult, error) {
	stop := spinner(fmt.Sprintf("📌 %s 브랜치 생성", branchType))

	if err := git.FetchAll(e.RepoDir); err != nil {
		stop()
		return nil, err
	}

	originalBranch, _ := git.GetCurrentBranch(e.RepoDir)

	// Update base branch
	if err := git.Checkout(e.RepoDir, baseBranch); err != nil {
		stop()
		return nil, fmt.Errorf("base 브랜치 체크아웃 실패 (%s): %w", baseBranch, err)
	}
	if err := git.Pull(e.RepoDir, remote, baseBranch); err != nil {
		logWarn("pull 실패, 현재 상태로 계속합니다")
	}

	// Deduplicate branch name
	branchName = git.DeduplicateBranchName(e.RepoDir, remote, branchName)

	// Create worktree
	exists, _ := git.WorktreeExists(e.RepoDir, worktreePath)
	if exists {
		stop()
		logWarn("워크트리가 이미 존재합니다: %s", worktreePath)
	} else {
		if err := git.WorktreeAdd(e.RepoDir, worktreePath, branchName); err != nil {
			stop()
			return nil, fmt.Errorf("워크트리 생성 실패: %w", err)
		}
		// Restore original branch
		if originalBranch != "" {
			git.Checkout(e.RepoDir, originalBranch)
		}
		stop()
		logSuccess("워크트리 생성 완료")
	}

	return &InitResult{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		BaseBranch:   baseBranch,
		BranchType:   branchType,
		IssueKey:     issueKey,
	}, nil
}

// createWorktreeFromTag creates a worktree for a new branch based off a git tag.
// Tags are immutable, so there is no checkout/pull of the base.
func (e *Engine) createWorktreeFromTag(branchType, tag, branchName, issueKey, worktreePath, remote string) (*InitResult, error) {
	stop := spinner(fmt.Sprintf("📌 %s 브랜치 생성 (태그: %s)", branchType, tag))

	branchName = git.DeduplicateBranchName(e.RepoDir, remote, branchName)

	exists, _ := git.WorktreeExists(e.RepoDir, worktreePath)
	if exists {
		stop()
		logWarn("워크트리가 이미 존재합니다: %s", worktreePath)
	} else {
		if err := git.WorktreeAddFromStartPoint(e.RepoDir, worktreePath, branchName, tag); err != nil {
			stop()
			return nil, fmt.Errorf("워크트리 생성 실패: %w", err)
		}
		stop()
		logSuccess("워크트리 생성 완료")
	}

	return &InitResult{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		BaseBranch:   tag,
		BranchType:   branchType,
		IssueKey:     issueKey,
	}, nil
}

// initLocalReview handles local-review worktree creation by finding a remote branch with the same issue number.
func (e *Engine) initLocalReview(issueKey, issueNum, worktreePath, remote string) (*InitResult, error) {
	stop := spinner("📌 리모트에서 브랜치 검색 중")

	if err := git.FetchAll(e.RepoDir); err != nil {
		stop()
		return nil, err
	}

	// Search remote branches containing the issue key
	branches, err := git.ListRemoteBranches(e.RepoDir, issueKey)
	if err != nil {
		stop()
		return nil, err
	}

	// Filter: strip remote prefix, exclude HEAD pointers
	var candidates []string
	for _, b := range branches {
		b = strings.TrimSpace(b)
		if b == "" || strings.Contains(b, "->") {
			continue
		}
		// Strip "origin/" prefix for display
		name := strings.TrimPrefix(b, remote+"/")
		candidates = append(candidates, name)
	}

	stop()

	if len(candidates) == 0 {
		return nil, fmt.Errorf("리모트에서 이슈 %s에 해당하는 브랜치를 찾을 수 없습니다", issueKey)
	}

	var selectedBranch string
	if len(candidates) == 1 {
		selectedBranch = candidates[0]
		logInfo("리모트 브랜치 발견: %s", selectedBranch)
	} else {
		logInfo("이슈 %s에 해당하는 브랜치가 %d개 발견되었습니다", issueKey, len(candidates))
		selectedBranch, err = e.Prompter.SelectWorktree(candidates)
		if err != nil {
			return nil, err
		}
	}

	branchType := config.BranchTypeFromName(e.RepoDir, selectedBranch)

	stop = spinner("📌 워크트리 생성 중")

	// Create worktree from the remote branch
	exists, _ := git.WorktreeExists(e.RepoDir, worktreePath)
	if exists {
		stop()
		logWarn("워크트리가 이미 존재합니다: %s", worktreePath)
	} else {
		if git.BranchExists(e.RepoDir, selectedBranch) {
			if err := git.WorktreeAddExisting(e.RepoDir, worktreePath, selectedBranch); err != nil {
				stop()
				return nil, fmt.Errorf("워크트리 생성 실패: %w", err)
			}
		} else {
			if err := git.WorktreeAddFromStartPoint(e.RepoDir, worktreePath, selectedBranch,
				fmt.Sprintf("%s/%s", remote, selectedBranch)); err != nil {
				stop()
				return nil, fmt.Errorf("워크트리 생성 실패: %w", err)
			}
		}
		stop()
		logSuccess("워크트리 생성 완료")
	}

	// Detect base branch from PR if possible
	baseBranch := ""
	if e.GH != nil {
		// Try to find PR for this branch to get the base
		// Non-fatal if it fails
		baseBranch = ""
	}

	return &InitResult{
		WorktreePath: worktreePath,
		BranchName:   selectedBranch,
		BaseBranch:   baseBranch,
		BranchType:   branchType,
		IssueKey:     issueKey,
	}, nil
}

// postSetup runs post-worktree-creation setup.
func (e *Engine) postSetup(result *InitResult, remote string, noInstall bool) error {
	// Copy AI agent tools
	mainRepo, err := git.GetRepoRoot(e.RepoDir)
	if err != nil {
		return err
	}
	logInfo("📦 AI & Agent Tools 파일 복사 중...")
	if err := fileutil.CopyAIAgentTools(mainRepo, result.WorktreePath, config.AIToolPatterns); err != nil {
		logWarn("AI tools 복사 실패: %v", err)
	}

	// Copy local files (e.g. .env.local)
	localPatterns := config.GetLocalFilePatterns(mainRepo)
	if len(localPatterns) > 0 {
		logInfo("📄 로컬 파일 복사 중...")
		copied, warns := fileutil.CopyLocalFiles(mainRepo, result.WorktreePath, localPatterns)
		for _, c := range copied {
			logInfo("   복사: %s", c)
		}
		for _, w := range warns {
			logWarn("   %s", w)
		}
	}

	// Add .claude/.claude/ to exclude
	git.AddExcludePattern(e.RepoDir, ".claude/.claude/")

	// Save issue info
	logInfo("📝 이슈 정보 저장 중...")
	fileutil.SaveCurrentIssue(result.WorktreePath, result.IssueKey)
	fileutil.SaveBranchType(result.WorktreePath, result.BranchType)

	// Try to save issue title from Jira, and move the issue into progress.
	if e.Jira != nil {
		if iss, err := e.Jira.ViewIssue(result.IssueKey); err == nil {
			fileutil.SaveIssueTitle(result.WorktreePath, iss.Summary)
			// 워크트리·브랜치 생성 성공 → 이슈를 진행 상태로 전이 시도(실패해도 조용히 무시)
			e.tryTransitionInProgress(iss)
		}
	}

	// Ask to save issue context (Jira issue → markdown)
	if e.Jira != nil {
		e.promptIssueContext(result)
	}

	// Install dependencies
	if !noInstall {
		installDependencies(mainRepo, result.WorktreePath)
	}

	return nil
}

// tryTransitionInProgress attempts to move a freshly-created issue into the
// in-progress state right after its worktree/branch has been set up. It is
// best-effort: any failure (transition not available, network error, unknown
// status) is silently ignored since the worktree is already created.
//
// 아직 시작 전(statusCategory "new", 예: 할일)인 이슈만 전이한다. 이미 진행중
// 이거나 완료된 이슈를 되돌리지 않도록 statusCategory 기준으로 가드한다.
func (e *Engine) tryTransitionInProgress(iss *jira.Issue) {
	if iss.StatusCategory != "" && iss.StatusCategory != "new" {
		return
	}
	wf := config.ResolveJiraWorkflow(e.RepoDir)
	if wf.Transitions.InProgress == "" {
		return // workflow not configured for this transition
	}
	if err := e.Jira.TransitionIssue(iss.Key, wf.Transitions.InProgress); err != nil {
		return
	}
	logInfo("📌 Jira 상태 전이: %s → %s", iss.Key, wf.Statuses.InProgress)
}

// promptIssueContext asks the user how to handle Jira issue context.
func (e *Engine) promptIssueContext(result *InitResult) {
	mode, err := e.Prompter.SelectIssueContextMode(result.IssueKey)
	if err != nil || mode == tui.IssueContextSkip {
		return
	}

	// If planning mode selected, ask for TDD strategy
	if mode == tui.IssueContextPlanning {
		tddMode, tddErr := e.Prompter.SelectTDDMode()
		if tddErr != nil {
			return
		}
		switch tddMode {
		case tui.TDDModeTDD:
			mode = tui.IssueContextPlanningTDD
		case tui.TDDModePostTC:
			mode = tui.IssueContextPlanningPostTC
		case tui.TDDModeNone:
			// keep as IssueContextPlanning
		}
	}

	iss, err := e.Jira.ViewIssue(result.IssueKey)
	if err != nil {
		logWarn("이슈 조회 실패: %v", err)
		return
	}

	ctx := fileutil.IssueContext{
		Key:         iss.Key,
		Summary:     iss.Summary,
		Description: iss.Description,
		Status:      iss.Status,
		ParentKey:   iss.ParentKey,
	}

	var saveErr error
	switch mode {
	case tui.IssueContextFull:
		saveErr = fileutil.SaveIssueContext(result.WorktreePath, ctx)
	case tui.IssueContextPlanning:
		saveErr = fileutil.SaveIssueContextPlanning(result.WorktreePath, ctx)
	case tui.IssueContextPlanningTDD:
		saveErr = fileutil.SaveIssueContextPlanningTDD(result.WorktreePath, ctx)
	case tui.IssueContextPlanningPostTC:
		saveErr = fileutil.SaveIssueContextPlanningPostTC(result.WorktreePath, ctx)
	}
	if saveErr != nil {
		logWarn("이슈 컨텍스트 저장 실패: %v", saveErr)
		return
	}

	logSuccess("이슈 컨텍스트 저장 완료: .grove/%s.local.md", result.IssueKey)
	logInfo("CLAUDE.local.md에 참조 추가됨")
}

// installDependencies detects package manager and installs deps.
func installDependencies(repoDir, worktreePath string) {
	logInfo("⬇️  의존성 설치 중...")

	type lockEntry struct {
		file string
		mgr  string
		cmd  string
	}
	lockFiles := []lockEntry{
		{"pnpm-lock.yaml", "pnpm", "pnpm install --frozen-lockfile"},
		{"package-lock.json", "npm", "npm ci"},
		{"poetry.lock", "poetry", "poetry install"},
		{"Pipfile.lock", "pipenv", "pipenv install"},
		{"requirements.txt", "pip", "pip install -r requirements.txt"},
		{"go.sum", "go", "go mod download"},
		{"Gemfile.lock", "bundler", "bundle install"},
		{"Cargo.lock", "cargo", "cargo build"},
	}

	var pkgManager, installCmd string
	for _, e := range lockFiles {
		if _, err := os.Stat(worktreePath + "/" + e.file); err == nil {
			pkgManager = e.mgr
			installCmd = e.cmd
			break
		}
	}

	if pkgManager == "" {
		logInfo("   lock 파일 없음 → 의존성 설치 skip")
		runPostCheckout(repoDir, worktreePath)
		return
	}

	logInfo("   사용 패키지 매니저: %s", pkgManager)
	if err := runShell(worktreePath, installCmd); err != nil {
		logWarn("%s install 실패. 워크트리에서 수동으로 실행해주세요.", pkgManager)
	}

	runPostCheckout(repoDir, worktreePath)
}

// runPostCheckout runs .husky/post-checkout (if exists) and any extra commands from repo config.
func runPostCheckout(repoDir, worktreePath string) {
	// 1. .husky/post-checkout 자동 실행
	huskyHook := worktreePath + "/.husky/post-checkout"
	if info, err := os.Stat(huskyHook); err == nil && !info.IsDir() {
		logInfo("🪝 .husky/post-checkout 실행 중...")
		if err := runShell(worktreePath, huskyHook); err != nil {
			logWarn(".husky/post-checkout 실패. 워크트리에서 수동으로 실행해주세요.")
		}
	}

	// 2. 레포별 추가 post-checkout 명령어
	postCmds := config.GetPostCheckoutCommands(repoDir)
	for _, cmd := range postCmds {
		logInfo("   post-checkout: %s", cmd)
		if err := runShell(worktreePath, cmd); err != nil {
			logWarn("post-checkout 명령 실패: %s (워크트리에서 수동으로 실행해주세요)", cmd)
		}
	}
}

func extractIssueFromBranch(branch string) (*issue.IssueKey, error) {
	re := config.IssueKeyRegex()
	m := re.FindStringSubmatch(branch)
	if m == nil {
		// Fallback: try generic pattern
		generic := regexp.MustCompile(`([A-Z][A-Z0-9]+)-(\d+)`)
		m = generic.FindStringSubmatch(branch)
		if m == nil {
			return nil, fmt.Errorf("no issue key found in branch: %s", branch)
		}
	}
	return &issue.IssueKey{Prefix: strings.ToUpper(m[1]), Number: m[2]}, nil
}
