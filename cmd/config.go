package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/git"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "grove 설정 조회 및 변경",
	Long: `grove 설정을 조회하거나 변경합니다.

사용 예시:
  grove config                          # 전체 설정 조회
  grove config prefix PROJ            # 이슈 prefix 설정
  grove config jira                     # Jira API 토큰 설정 (대화형)
  grove config local-files              # 로컬 파일 패턴 목록
  grove config local-files --add ".env.local"
  grove config local-files --remove ".env.local"
  grove config post-checkout             # post-checkout 명령어 목록
  grove config post-checkout --add "pnpm run codegen"
  grove config post-checkout --remove "pnpm run codegen"
  grove config branch-types              # 레포 브랜칭 정책 조회
  grove config branch-types --scaffold   # 정책을 config.json에 풀어써서 직접 편집
  grove config base-branch -a feature develop   # 타입별 base 고정
  grove config branch-prefix -a feature feat    # 타입별 브랜치 prefix`,
	RunE: runConfigShow,
}

var configPrefixCmd = &cobra.Command{
	Use:   "prefix [VALUE]",
	Short: "이슈 prefix 조회/설정",
	Long:  `인자 없이 실행하면 현재 값을 표시하고, 인자를 주면 설정합니다.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConfigPrefix,
}

var configJiraCmd = &cobra.Command{
	Use:   "jira",
	Short: "Jira API 토큰 설정 (대화형)",
	RunE:  runConfigJira,
}

var configJiraWorkflowCmd = &cobra.Command{
	Use:   "jira-workflow",
	Short: "Jira 워크플로우(상태/전이 이름) 조회/초기화",
	Long: `grove의 이슈 파이프라인을 Jira 프로젝트의 실제 상태·전이 이름에 매핑합니다.
전역(~/.config/grove/config.json)에 저장되며, 레포별(.grove/config.json)로 덮어쓸 수 있습니다.

미설정 시 jira update/deploy/weekly 및 init 자동 전이가 비활성화됩니다
(워크트리 생성 등 나머지 기능은 그대로 동작).

매핑 항목:
  statuses      각 단계의 Jira 상태 이름 (inProgress, devComplete, inReview, ...)
  transitions   각 단계로의 전이(액션) 이름
  deployField   배포일자 커스텀 필드 id (예: customfield_11642)
  deployWeekday 배포일 제안 요일 (기본 Tuesday)

사용 예시:
  grove config jira-workflow             # 현재 매핑 조회
  grove config jira-workflow --scaffold  # 전역 config.json에 예시 템플릿 작성
  grove config jira-workflow --scaffold --repo  # 레포 config.json에 작성`,
	RunE: runConfigJiraWorkflow,
}

var configBranchTypesCmd = &cobra.Command{
	Use:   "branch-types",
	Short: "레포지토리 브랜칭 정책 조회/초기화",
	Long: `이슈 타입별 브랜치 생성 정책을 조회합니다. 각 타입은 이름·prefix·전략·base로
구성되며, 이 목록이 곧 레포지토리의 브랜칭 정책입니다.
설정은 레포지토리별로 <repo>/.grove/config.json의 branchTypes에 저장됩니다.
미설정 시 built-in 기본 정책을 사용합니다.

전략(strategy):
  from-default       기본 브랜치에서 딴다
  from-branch        지정/선택한 브랜치에서 딴다 (base 비면 검색 선택)
  from-tag           지정/선택한 태그에서 딴다 (base 비면 검색 선택)
  checkout-existing  기존 리모트 브랜치를 체크아웃 (새 브랜치 없음)

사용 예시:
  grove config branch-types             # 현재 정책 표로 조회
  grove config branch-types --scaffold  # 현재 정책을 config.json에 풀어써서 직접 편집`,
	RunE: runConfigBranchTypes,
}

var configBranchPrefixCmd = &cobra.Command{
	Use:   "branch-prefix",
	Short: "이슈 타입별 브랜치 이름 prefix 설정 (레포지토리별)",
	Long: `브랜치 이름의 prefix를 이슈 타입별로 설정합니다.
브랜치 이름은 {prefix}/{이슈키} 형식입니다 (예: feat/PROJ-123).
설정은 <repo>/.grove/config.json의 branchTypes에 저장됩니다.

사용 예시:
  grove config branch-prefix                     # 설정 목록 조회
  grove config branch-prefix -a feature feat     # 추가: feature → feat/PROJ-123
  grove config branch-prefix -d feature          # 삭제: feature prefix를 기본값으로`,
	Args: cobra.MaximumNArgs(2),
	RunE: runConfigBranchPrefix,
}

var configBaseBranchCmd = &cobra.Command{
	Use:   "base-branch",
	Short: "이슈 타입별 base 브랜치/태그 고정 (레포지토리별)",
	Long: `이슈 타입이 워크트리를 생성할 기준(base) 브랜치/태그를 고정합니다.
고정하면 grove init 시 선택 프롬프트 없이 그 ref에서 바로 딴다.
설정은 <repo>/.grove/config.json의 branchTypes에 저장됩니다.

사용 예시:
  grove config base-branch                       # 설정 목록 조회
  grove config base-branch -a feature develop    # 고정: feature base를 develop으로
  grove config base-branch -a hotfix v1.2.0      # 고정: hotfix base를 태그 v1.2.0으로
  grove config base-branch -d feature            # 고정 해제`,
	Args: cobra.MaximumNArgs(2),
	RunE: runConfigBaseBranch,
}

var configLocalFilesCmd = &cobra.Command{
	Use:   "local-files",
	Short: "로컬 파일 복사 패턴 관리",
	Long: `grove init 시 메인 레포에서 워크트리로 복사할 로컬(untracked) 파일 패턴을 관리합니다.

사용 예시:
  grove config local-files                          # 목록 조회
  grove config local-files --add ".env.local"       # 패턴 추가
  grove config local-files --add ".env.development" # 패턴 추가
  grove config local-files --remove ".env.local"    # 패턴 제거

패턴은 filepath.Glob 형식입니다 (e.g. ".env*", ".env.local", "config/*.local.json").`,
	RunE: runConfigLocalFiles,
}

var configPostCheckoutCmd = &cobra.Command{
	Use:   "post-checkout",
	Short: "워크트리 생성 후 추가 실행할 명령어 관리",
	Long: `grove init 시 의존성 설치 이후 추가로 실행할 명령어를 관리합니다.
.husky/post-checkout 훅은 자동으로 실행되며, 여기서는 추가 명령어를 설정합니다.
설정은 레포지토리별로 <repo>/.grove/config.json에 저장됩니다.

사용 예시:
  grove config post-checkout                                # 목록 조회
  grove config post-checkout --add "pnpm run codegen"       # 명령어 추가
  grove config post-checkout --remove "pnpm run codegen"    # 명령어 제거

명령어는 워크트리 디렉토리에서 순서대로 실행됩니다.`,
	RunE: runConfigPostCheckout,
}

func init() {
	configLocalFilesCmd.Flags().String("add", "", "패턴 추가")
	configLocalFilesCmd.Flags().String("remove", "", "패턴 제거")
	configPostCheckoutCmd.Flags().String("add", "", "명령어 추가")
	configPostCheckoutCmd.Flags().String("remove", "", "명령어 제거")
	configBranchTypesCmd.Flags().Bool("scaffold", false, "현재 정책을 config.json에 풀어쓰기")
	configJiraWorkflowCmd.Flags().Bool("scaffold", false, "예시 워크플로우 템플릿을 config.json에 작성")
	configJiraWorkflowCmd.Flags().Bool("repo", false, "전역 대신 레포별 config.json에 작성")
	configBaseBranchCmd.Flags().BoolP("add", "a", false, "base 고정 (TYPE REF)")
	configBaseBranchCmd.Flags().BoolP("delete", "d", false, "base 고정 해제 (TYPE)")
	configBranchPrefixCmd.Flags().BoolP("add", "a", false, "브랜치 prefix 추가 (TYPE PREFIX)")
	configBranchPrefixCmd.Flags().BoolP("delete", "d", false, "브랜치 prefix 삭제 (TYPE)")

	configCmd.AddCommand(configPrefixCmd)
	configCmd.AddCommand(configJiraCmd)
	configCmd.AddCommand(configLocalFilesCmd)
	configCmd.AddCommand(configPostCheckoutCmd)
	configCmd.AddCommand(configJiraWorkflowCmd)
	configCmd.AddCommand(configBranchTypesCmd)
	configCmd.AddCommand(configBaseBranchCmd)
	configCmd.AddCommand(configBranchPrefixCmd)
	rootCmd.AddCommand(configCmd)
}

// runConfigShow shows all config.
func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg := config.Load()
	repoDir, repoErr := getRepoRoot()

	fmt.Fprintln(os.Stderr, cyanStyle.Render("grove 설정"))
	fmt.Fprintln(os.Stderr)

	// Prefix (per-repo)
	if repoErr != nil {
		fmt.Fprintf(os.Stderr, "  이슈 prefix:    %s\n", yellowStyle.Render("(레포 밖)"))
	} else if p := config.GetIssuePrefixForRepo(repoDir); p != "" {
		fmt.Fprintf(os.Stderr, "  이슈 prefix:    %s\n", greenStyle.Render(p))
	} else {
		fmt.Fprintf(os.Stderr, "  이슈 prefix:    %s\n", yellowStyle.Render("(미설정)"))
	}

	// Jira (global)
	if cfg.JiraAPIToken != "" {
		masked := cfg.JiraAPIToken[:4] + "..." + cfg.JiraAPIToken[len(cfg.JiraAPIToken)-4:]
		fmt.Fprintf(os.Stderr, "  Jira 서버:      %s\n", cfg.JiraServer)
		fmt.Fprintf(os.Stderr, "  Jira 로그인:    %s\n", cfg.JiraLogin)
		fmt.Fprintf(os.Stderr, "  Jira API 토큰:  %s\n", masked)
	} else {
		fmt.Fprintf(os.Stderr, "  Jira API 토큰:  %s\n", yellowStyle.Render("(미설정)"))
	}

	// Local files (per-repo)
	fmt.Fprintln(os.Stderr)
	if patterns := config.GetLocalFilePatterns(repoDir); len(patterns) > 0 {
		fmt.Fprintln(os.Stderr, "  로컬 파일 패턴:")
		for _, p := range patterns {
			fmt.Fprintf(os.Stderr, "    - %s\n", p)
		}
	} else {
		fmt.Fprintf(os.Stderr, "  로컬 파일 패턴: %s\n", yellowStyle.Render("(없음)"))
	}

	// Post-checkout commands (per-repo)
	fmt.Fprintln(os.Stderr)
	if repoErr == nil {
		repoCfg := config.LoadRepoConfig(repoDir)
		fmt.Fprintln(os.Stderr, "  post-checkout (자동): .husky/post-checkout")
		if len(repoCfg.PostCheckoutCommands) > 0 {
			fmt.Fprintln(os.Stderr, "  post-checkout (추가):")
			for _, c := range repoCfg.PostCheckoutCommands {
				fmt.Fprintf(os.Stderr, "    - %s\n", c)
			}
		} else {
			fmt.Fprintf(os.Stderr, "  post-checkout (추가): %s\n", yellowStyle.Render("(없음)"))
		}

		// Branching policy (per-repo)
		fmt.Fprintln(os.Stderr)
		origin := "built-in 기본값"
		if config.HasBranchTypesConfig(repoDir) {
			origin = "config.json"
		}
		fmt.Fprintf(os.Stderr, "  브랜칭 정책 (%s):\n", origin)
		printBranchTypeTable(config.ResolveBranchTypes(repoDir), "    ")
	}

	return nil
}

// printBranchTypeTable prints the resolved branch type policy as an aligned table.
func printBranchTypeTable(defs []config.BranchTypeDef, indent string) {
	for _, d := range defs {
		base := d.Base
		if base == "" {
			switch d.Strategy {
			case config.StrategyFromDefault:
				base = dimText("(기본 브랜치)")
			case config.StrategyFromBranch:
				base = dimText("(선택)")
			case config.StrategyFromTag:
				base = dimText("(태그 선택)")
			default:
				base = dimText("-")
			}
		} else {
			base = greenStyle.Render(base)
		}
		fmt.Fprintf(os.Stderr, "%s%-13s %-12s  %-18s %s\n",
			indent, d.Name, cyanStyle.Render(d.EffectivePrefix()+"/"), d.Strategy, base)
	}
}

func dimText(s string) string { return yellowStyle.Render(s) }

// runConfigPrefix gets or sets the issue prefix.
func runConfigPrefix(cmd *cobra.Command, args []string) error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		current := config.GetIssuePrefixForRepo(repoRoot)
		if current == "" {
			fmt.Fprintln(os.Stderr, yellowStyle.Render("(미설정)"))
		} else {
			fmt.Fprintln(os.Stderr, current)
		}
		return nil
	}

	if err := config.SetIssuePrefixForRepo(repoRoot, args[0]); err != nil {
		return fmt.Errorf("prefix 저장 실패: %w", err)
	}
	fmt.Fprintf(os.Stderr, "%s 이슈 prefix 설정: %s\n", "✅", config.GetIssuePrefixForRepo(repoRoot))
	return nil
}

// runConfigJira runs interactive Jira config (moved from setup).
func runConfigJira(cmd *cobra.Command, args []string) error {
	server, login, token := config.GetJiraConfig()
	reader := bufio.NewReader(os.Stdin)

	fmt.Fprintln(os.Stderr, cyanStyle.Render("Jira API 토큰 설정"))
	fmt.Fprintln(os.Stderr)

	if token != "" {
		masked := token[:4] + "..." + token[len(token)-4:]
		fmt.Fprintf(os.Stderr, "  현재 토큰: %s (%s)\n", masked, server)
		fmt.Fprintln(os.Stderr)
	} else {
		fmt.Fprintln(os.Stderr, "  jira-cli API 제한 시 REST API로 자동 전환됩니다.")
		fmt.Fprintln(os.Stderr, "  토큰 발급: https://id.atlassian.com/manage-profile/security/api-tokens")
		fmt.Fprintln(os.Stderr)
	}

	// Server
	if server == "" {
		server = readJiraCLIServer()
	}
	if server != "" {
		fmt.Fprintf(os.Stderr, "  Jira 서버 [%s]: ", server)
	} else {
		fmt.Fprint(os.Stderr, "  Jira 서버 URL (e.g. https://mycompany.atlassian.net): ")
	}
	input, _ := reader.ReadString('\n')
	if s := strings.TrimSpace(input); s != "" {
		server = s
	}

	// Login
	if login == "" {
		login = readJiraCLILogin()
	}
	if login != "" {
		fmt.Fprintf(os.Stderr, "  Jira 로그인 [%s]: ", login)
	} else {
		fmt.Fprint(os.Stderr, "  Jira 로그인 (이메일): ")
	}
	input, _ = reader.ReadString('\n')
	if s := strings.TrimSpace(input); s != "" {
		login = s
	}

	// API Token
	fmt.Fprint(os.Stderr, "  API 토큰: ")
	input, _ = reader.ReadString('\n')
	newToken := strings.TrimSpace(input)
	if newToken == "" {
		fmt.Fprintln(os.Stderr, yellowStyle.Render("  ⚠ 토큰이 입력되지 않았습니다"))
		return nil
	}

	if err := config.SetJiraConfig(server, login, newToken); err != nil {
		return fmt.Errorf("저장 실패: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  %s Jira API 토큰 저장 완료\n", "✅")
	return nil
}

// getRepoRoot returns the repo root for the current directory.
func getRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("현재 디렉토리를 찾을 수 없습니다: %w", err)
	}
	root, err := git.GetRepoRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("git 레포지토리가 아닙니다: %w", err)
	}
	return root, nil
}

// runConfigPostCheckout manages post-checkout commands (per-repo).
func runConfigPostCheckout(cmd *cobra.Command, args []string) error {
	repoDir, err := getRepoRoot()
	if err != nil {
		return err
	}

	addCmd, _ := cmd.Flags().GetString("add")
	removeCmd, _ := cmd.Flags().GetString("remove")

	if addCmd != "" {
		if err := config.AddPostCheckoutCommand(repoDir, addCmd); err != nil {
			return fmt.Errorf("명령어 추가 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s 명령어 추가: %s\n", "✅", addCmd)
		fmt.Fprintf(os.Stderr, "  저장 위치: %s/.grove/config.json\n", repoDir)
		return nil
	}

	if removeCmd != "" {
		if err := config.RemovePostCheckoutCommand(repoDir, removeCmd); err != nil {
			return fmt.Errorf("명령어 제거 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s 명령어 제거: %s\n", "✅", removeCmd)
		return nil
	}

	// List
	fmt.Fprintln(os.Stderr, cyanStyle.Render("post-checkout 설정"))
	fmt.Fprintln(os.Stderr, "  (자동) .husky/post-checkout 있으면 실행")
	fmt.Fprintln(os.Stderr)

	commands := config.GetPostCheckoutCommands(repoDir)
	if len(commands) == 0 {
		fmt.Fprintf(os.Stderr, "  추가 명령어: %s\n", yellowStyle.Render("(없음)"))
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "명령어 추가:")
		fmt.Fprintln(os.Stderr, "  grove config post-checkout --add \"pnpm run codegen\"")
		fmt.Fprintf(os.Stderr, "\n설정은 레포지토리별로 저장됩니다: <repo>/.grove/config.json\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "  추가 명령어 (%s/.grove/config.json):\n", repoDir)
	for _, c := range commands {
		fmt.Fprintf(os.Stderr, "    - %s\n", c)
	}
	return nil
}

// runConfigBranchTypes shows the resolved branching policy, or scaffolds it.
func runConfigBranchTypes(cmd *cobra.Command, args []string) error {
	repoDir, err := getRepoRoot()
	if err != nil {
		return err
	}

	if scaffold, _ := cmd.Flags().GetBool("scaffold"); scaffold {
		defs, err := config.ScaffoldBranchTypes(repoDir)
		if err != nil {
			return fmt.Errorf("정책 저장 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s 브랜칭 정책을 %s/.grove/config.json에 작성했습니다\n", "✅", repoDir)
		printBranchTypeTable(defs, "  ")
		fmt.Fprintln(os.Stderr, "\n이제 config.json의 branchTypes를 직접 편집할 수 있습니다.")
		return nil
	}

	origin := "built-in 기본값"
	if config.HasBranchTypesConfig(repoDir) {
		origin = "config.json"
	}
	fmt.Fprintf(os.Stderr, "%s (%s)\n", cyanStyle.Render("브랜칭 정책"), origin)
	printBranchTypeTable(config.ResolveBranchTypes(repoDir), "  ")
	if origin != "config.json" {
		fmt.Fprintln(os.Stderr, "\n직접 편집하려면: grove config branch-types --scaffold")
	}
	return nil
}

// runConfigJiraWorkflow shows the resolved Jira workflow, or scaffolds a template.
func runConfigJiraWorkflow(cmd *cobra.Command, args []string) error {
	if scaffold, _ := cmd.Flags().GetBool("scaffold"); scaffold {
		toRepo, _ := cmd.Flags().GetBool("repo")
		var path string
		var err error
		if toRepo {
			repoDir, rerr := getRepoRoot()
			if rerr != nil {
				return rerr
			}
			path, err = config.ScaffoldRepoJiraWorkflow(repoDir)
		} else {
			path, err = config.ScaffoldGlobalJiraWorkflow()
		}
		if err != nil {
			return fmt.Errorf("워크플로우 저장 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s Jira 워크플로우 템플릿을 %s에 작성했습니다\n", "✅", path)
		fmt.Fprintln(os.Stderr, "  이제 statuses/transitions 이름을 실제 Jira 프로젝트에 맞게 편집하세요.")
		fmt.Fprintln(os.Stderr, "  전이 이름은 Jira 이슈의 전이(액션) 메뉴에 표시되는 이름과 정확히 일치해야 합니다.")
		return nil
	}

	wf := config.ResolveJiraWorkflow(currentRepoDirSafe())
	if !wf.Configured() {
		fmt.Fprintln(os.Stderr, cyanStyle.Render("Jira 워크플로우"), "(미설정)")
		fmt.Fprintln(os.Stderr, "\n템플릿 작성: grove config jira-workflow --scaffold")
		return nil
	}

	fmt.Fprintln(os.Stderr, cyanStyle.Render("Jira 워크플로우"))
	fmt.Fprintln(os.Stderr, "  [상태]")
	printKV("    inProgress", wf.Statuses.InProgress)
	printKV("    devComplete", wf.Statuses.DevComplete)
	printKV("    inReview", wf.Statuses.InReview)
	printKV("    reviewComplete", wf.Statuses.ReviewComplete)
	printKV("    reviewPassed", wf.Statuses.ReviewPassed)
	printKV("    resolvedClosed", wf.Statuses.ResolvedClosed)
	printKV("    terminal", strings.Join(wf.Statuses.Terminal, ", "))
	fmt.Fprintln(os.Stderr, "  [전이]")
	printKV("    inProgress", wf.Transitions.InProgress)
	printKV("    devComplete", wf.Transitions.DevComplete)
	printKV("    inReview", wf.Transitions.InReview)
	printKV("    reviewComplete", wf.Transitions.ReviewComplete)
	printKV("    resolvedClose", wf.Transitions.ResolvedClose)
	printKV("    close", wf.Transitions.Close)
	fmt.Fprintln(os.Stderr, "  [배포]")
	printKV("    deployField", wf.DeployField)
	printKV("    deployWeekday", wf.DeployWeekday)
	printKV("    excludedStatuses", strings.Join(wf.ExcludedStatuses, ", "))
	return nil
}

// printKV prints a "key: value" line, showing "-" for empty values.
func printKV(key, value string) {
	if value == "" {
		value = dimStyle.Render("-")
	}
	fmt.Fprintf(os.Stderr, "  %s: %s\n", key, value)
}

// currentRepoDirSafe returns the repo root, or "" when not inside a repo.
func currentRepoDirSafe() string {
	repoDir, err := getRepoRoot()
	if err != nil {
		return ""
	}
	return repoDir
}

// runConfigBranchPrefix manages per-type branch name prefixes.
func runConfigBranchPrefix(cmd *cobra.Command, args []string) error {
	repoDir, err := getRepoRoot()
	if err != nil {
		return err
	}

	add, _ := cmd.Flags().GetBool("add")
	del, _ := cmd.Flags().GetBool("delete")
	if add && del {
		return fmt.Errorf("-a 와 -d 는 함께 사용할 수 없습니다")
	}

	if add {
		if len(args) != 2 {
			return fmt.Errorf("사용법: grove config branch-prefix -a TYPE PREFIX (예: -a feature feat)")
		}
		branchType, prefix := args[0], args[1]
		if err := validateBranchType(repoDir, branchType); err != nil {
			return err
		}
		if err := config.SetBranchTypePrefix(repoDir, branchType, prefix); err != nil {
			return fmt.Errorf("설정 저장 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s %s 브랜치 prefix 설정: %s/ (예: %s/%s)\n",
			"✅", branchType, prefix, prefix, config.FormatIssueKey("12345"))
		fmt.Fprintf(os.Stderr, "  저장 위치: %s/.grove/config.json\n", repoDir)
		return nil
	}

	if del {
		if len(args) != 1 {
			return fmt.Errorf("사용법: grove config branch-prefix -d TYPE (예: -d feature)")
		}
		branchType := args[0]
		if err := validateBranchType(repoDir, branchType); err != nil {
			return err
		}
		if err := config.SetBranchTypePrefix(repoDir, branchType, ""); err != nil {
			return fmt.Errorf("설정 제거 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s %s 브랜치 prefix 삭제 (기본값: %s)\n", "✅", branchType, branchType)
		return nil
	}

	// No flag: list resolved prefixes.
	fmt.Fprintln(os.Stderr, cyanStyle.Render("브랜치 prefix (타입별)"))
	for _, d := range config.ResolveBranchTypes(repoDir) {
		fmt.Fprintf(os.Stderr, "  %-13s %s\n", d.Name, greenStyle.Render(d.EffectivePrefix()+"/"))
	}
	fmt.Fprintln(os.Stderr, "\n추가: grove config branch-prefix -a feature feat")
	return nil
}

// runConfigBaseBranch pins/unpins the base ref (branch or tag) per branch type.
func runConfigBaseBranch(cmd *cobra.Command, args []string) error {
	repoDir, err := getRepoRoot()
	if err != nil {
		return err
	}

	add, _ := cmd.Flags().GetBool("add")
	del, _ := cmd.Flags().GetBool("delete")
	if add && del {
		return fmt.Errorf("-a 와 -d 는 함께 사용할 수 없습니다")
	}

	if add {
		if len(args) != 2 {
			return fmt.Errorf("사용법: grove config base-branch -a TYPE REF (예: -a feature develop)")
		}
		branchType, ref := args[0], args[1]
		if err := validateBranchType(repoDir, branchType); err != nil {
			return err
		}
		if err := config.SetBranchTypeBase(repoDir, branchType, ref); err != nil {
			return fmt.Errorf("설정 저장 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s %s base 고정: %s\n", "✅", branchType, ref)
		fmt.Fprintf(os.Stderr, "  저장 위치: %s/.grove/config.json\n", repoDir)
		return nil
	}

	if del {
		if len(args) != 1 {
			return fmt.Errorf("사용법: grove config base-branch -d TYPE (예: -d feature)")
		}
		branchType := args[0]
		if err := validateBranchType(repoDir, branchType); err != nil {
			return err
		}
		if err := config.SetBranchTypeBase(repoDir, branchType, ""); err != nil {
			return fmt.Errorf("설정 제거 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s %s base 고정 해제\n", "✅", branchType)
		return nil
	}

	// No flag: list resolved bases.
	fmt.Fprintln(os.Stderr, cyanStyle.Render("base 고정 (타입별)"))
	printBranchTypeTable(config.ResolveBranchTypes(repoDir), "  ")
	fmt.Fprintln(os.Stderr, "\n고정: grove config base-branch -a feature develop")
	return nil
}

// validateBranchType returns an error if name is not a known branch type for the repo.
func validateBranchType(repoDir, name string) error {
	names := config.BranchTypeNames(repoDir)
	for _, t := range names {
		if t == name {
			return nil
		}
	}
	return fmt.Errorf("알 수 없는 타입: %s (가능: %s)", name, strings.Join(names, ", "))
}

// runConfigLocalFiles manages local file patterns.
func runConfigLocalFiles(cmd *cobra.Command, args []string) error {
	repoDir, err := getRepoRoot()
	if err != nil {
		return err
	}

	addPattern, _ := cmd.Flags().GetString("add")
	removePattern, _ := cmd.Flags().GetString("remove")

	if addPattern != "" {
		if err := config.AddLocalFilePattern(repoDir, addPattern); err != nil {
			return fmt.Errorf("패턴 추가 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s 패턴 추가: %s\n", "✅", addPattern)
		return nil
	}

	if removePattern != "" {
		if err := config.RemoveLocalFilePattern(repoDir, removePattern); err != nil {
			return fmt.Errorf("패턴 제거 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s 패턴 제거: %s\n", "✅", removePattern)
		return nil
	}

	// List
	patterns := config.GetLocalFilePatterns(repoDir)
	if len(patterns) == 0 {
		fmt.Fprintln(os.Stderr, yellowStyle.Render("등록된 로컬 파일 패턴이 없습니다."))
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "패턴 추가:")
		fmt.Fprintln(os.Stderr, "  grove config local-files --add \".env.local\"")
		return nil
	}

	fmt.Fprintln(os.Stderr, cyanStyle.Render("로컬 파일 패턴"))
	for _, p := range patterns {
		fmt.Fprintf(os.Stderr, "  - %s\n", p)
	}
	return nil
}
