package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/harudev/grove-cli/internal/config"
)

var (
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	dimStyle    = lipgloss.NewStyle().Faint(true)
)

type tool struct {
	Name     string
	Cmd      string
	Install  string
	Required bool
}

var tools = []tool{
	{"Git", "git", "brew install git", true},
	{"GitHub CLI (gh)", "gh", "brew install gh", true},
	{"jira-cli", "jira", "brew install ankitpokhrel/jira-cli/jira-cli", true},
	{"Node.js", "node", "brew install node", false},
	{"pnpm", "pnpm", "brew install pnpm", false},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "grove 설정 및 의존 도구 확인",
	Long: `grove 설정(이슈 prefix 등)을 구성하고, 의존 도구 설치 상태를 확인합니다.

사용 예시:
  grove setup              # 설정 + 도구 확인
  grove setup --install    # 미설치 필수 도구 자동 설치
  grove setup --prefix ABC # 이슈 prefix만 변경`,
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().Bool("install", false, "미설치 필수 도구 자동 설치")
	setupCmd.Flags().String("prefix", "", "이슈 prefix 설정 (e.g. PROJ, PROJ)")
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	autoInstall, _ := cmd.Flags().GetBool("install")
	prefixFlag, _ := cmd.Flags().GetString("prefix")

	// --- Issue prefix configuration (per-repo) ---
	repoRoot, repoErr := getRepoRoot()
	if repoErr != nil {
		fmt.Fprintln(os.Stderr, yellowStyle.Render("⚠ git 레포지토리 밖이라 이슈 prefix 설정은 건너뜁니다. (prefix는 레포마다 따로 설정됩니다)"))
	} else if prefixFlag != "" {
		// Non-interactive: set prefix directly
		if err := config.SetIssuePrefixForRepo(repoRoot, prefixFlag); err != nil {
			return fmt.Errorf("prefix 저장 실패: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s 이슈 prefix 설정: %s\n", "✅", config.GetIssuePrefixForRepo(repoRoot))
	} else {
		configurePrefix(repoRoot)
	}

	// --- Jira API token ---
	_, _, token := config.GetJiraConfig()
	if token != "" {
		masked := token[:4] + "..." + token[len(token)-4:]
		fmt.Fprintf(os.Stderr, "%s Jira API 토큰 설정됨: %s\n", "✅", masked)
	} else {
		fmt.Fprintf(os.Stderr, "%s Jira API 토큰 미설정 → %s\n", yellowStyle.Render("–"), "grove config jira")
	}

	// --- Local file patterns (per-repo) ---
	var localPatterns []string
	if repoErr == nil {
		localPatterns = config.GetLocalFilePatterns(repoRoot)
	}
	if len(localPatterns) > 0 {
		fmt.Fprintf(os.Stderr, "%s 로컬 파일 복사 패턴: %s\n", "✅", strings.Join(localPatterns, ", "))
	} else {
		fmt.Fprintf(os.Stderr, "%s 로컬 파일 복사 패턴 미설정\n", yellowStyle.Render("–"))
		fmt.Fprintln(os.Stderr, "  워크트리 생성 시 .env.local 등을 자동 복사하려면:")
		fmt.Fprintf(os.Stderr, "  %s\n", cyanStyle.Render("grove config local-files --add \".env.local\""))
	}

	fmt.Fprintln(os.Stderr)

	// --- Tool check ---
	fmt.Fprintln(os.Stderr, cyanStyle.Render("의존 도구 상태"))
	fmt.Fprintln(os.Stderr)

	var missing []tool
	for _, t := range tools {
		_, err := exec.LookPath(t.Cmd)
		if err == nil {
			req := ""
			if t.Required {
				req = " (필수)"
			}
			fmt.Fprintf(os.Stderr, "  %s %s%s\n", "✅", t.Name, req)
		} else {
			if t.Required {
				fmt.Fprintf(os.Stderr, "  %s %s (필수)\n", redStyle.Render("✗"), t.Name)
			} else {
				fmt.Fprintf(os.Stderr, "  %s %s (선택)\n", yellowStyle.Render("–"), t.Name)
			}
			missing = append(missing, t)
		}
	}

	// Auth status
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, cyanStyle.Render("인증 상태"))
	fmt.Fprintln(os.Stderr)

	if _, err := exec.LookPath("gh"); err == nil {
		if exec.Command("gh", "auth", "status").Run() == nil {
			fmt.Fprintf(os.Stderr, "  %s GitHub CLI 인증 완료\n", "✅")
		} else {
			fmt.Fprintf(os.Stderr, "  %s GitHub CLI 미인증 → %s\n", redStyle.Render("✗"), "gh auth login")
		}
	}

	if _, err := exec.LookPath("jira"); err == nil {
		if exec.Command("jira", "me").Run() == nil {
			fmt.Fprintf(os.Stderr, "  %s jira-cli 인증 완료\n", "✅")
		} else {
			fmt.Fprintf(os.Stderr, "  %s jira-cli 미인증 → %s\n", redStyle.Render("✗"), "jira init")
		}
	}

	// --- Global gitignore (.grove) ---
	configureGlobalGitignore()

	// --- Shell function (ow) ---
	installShellFunction()

	// Install missing
	var requiredMissing []tool
	for _, t := range missing {
		if t.Required {
			requiredMissing = append(requiredMissing, t)
		}
	}

	if len(requiredMissing) == 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, greenStyle.Render("✅ 설정 완료"))
		return nil
	}

	fmt.Fprintln(os.Stderr)

	if autoInstall {
		fmt.Fprintln(os.Stderr, cyanStyle.Render("미설치 필수 도구 설치 중..."))
		for _, t := range requiredMissing {
			fmt.Fprintf(os.Stderr, "  %s 설치 중: %s\n", cyanStyle.Render("⬇"), t.Name)
			installCmd := exec.Command("bash", "-c", t.Install)
			installCmd.Stdout = os.Stderr
			installCmd.Stderr = os.Stderr
			if err := installCmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "  %s %s 설치 실패: %v\n", redStyle.Render("✗"), t.Name, err)
			} else {
				fmt.Fprintf(os.Stderr, "  %s %s 설치 완료\n", "✅", t.Name)
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, yellowStyle.Render("미설치 필수 도구:"))
		for _, t := range requiredMissing {
			fmt.Fprintf(os.Stderr, "  %s\n", t.Install)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "자동 설치: grove setup --install")
	}

	return nil
}

func configurePrefix(repoRoot string) {
	current := config.GetIssuePrefixForRepo(repoRoot)
	if current != "" {
		fmt.Fprintf(os.Stderr, "%s 이슈 prefix: %s\n", cyanStyle.Render("현재"), current)
		fmt.Fprint(os.Stderr, "변경하려면 새 prefix 입력 (Enter로 유지): ")
	} else {
		fmt.Fprint(os.Stderr, "이슈 prefix를 입력하세요 (e.g. PROJ, PROJ): ")
	}

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		if current == "" {
			fmt.Fprintln(os.Stderr, yellowStyle.Render("⚠ prefix가 설정되지 않았습니다. 숫자만 입력 시 이슈키 생성이 불가합니다."))
		}
		return
	}

	if err := config.SetIssuePrefixForRepo(repoRoot, input); err != nil {
		fmt.Fprintf(os.Stderr, "%s prefix 저장 실패: %v\n", redStyle.Render("✗"), err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s 이슈 prefix 설정: %s\n", "✅", config.GetIssuePrefixForRepo(repoRoot))
}

func configureJiraToken() {
	server, login, token := config.GetJiraConfig()

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, cyanStyle.Render("Jira API 토큰 (REST API fallback용)"))
	fmt.Fprintln(os.Stderr)

	if token != "" {
		masked := token[:4] + "..." + token[len(token)-4:]
		fmt.Fprintf(os.Stderr, "  %s 토큰 설정됨: %s (%s)\n", "✅", masked, server)
		fmt.Fprint(os.Stderr, "  재설정하려면 'y' 입력 (Enter로 유지): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.TrimSpace(input) != "y" {
			return
		}
	} else {
		fmt.Fprintln(os.Stderr, "  jira-cli API 제한 시 REST API로 자동 전환됩니다.")
		fmt.Fprintln(os.Stderr, "  토큰 발급: https://id.atlassian.com/manage-profile/security/api-tokens")
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, "  설정하시겠습니까? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(input)) != "y" {
			fmt.Fprintln(os.Stderr, yellowStyle.Render("  ⚠ 건너뜀 (jira-cli만 사용)"))
			return
		}
	}

	reader := bufio.NewReader(os.Stdin)

	// Server
	if server == "" {
		// Try to read from jira-cli config
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
		return
	}

	if err := config.SetJiraConfig(server, login, newToken); err != nil {
		fmt.Fprintf(os.Stderr, "  %s 저장 실패: %v\n", redStyle.Render("✗"), err)
		return
	}
	fmt.Fprintf(os.Stderr, "  %s Jira API 토큰 저장 완료\n", "✅")
}

// readJiraCLIServer reads the server from jira-cli config.
func readJiraCLIServer() string {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".config", ".jira", ".config.yml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "server:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "server:"))
		}
	}
	return ""
}

// readJiraCLILogin reads the login from jira-cli config.
func readJiraCLILogin() string {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".config", ".jira", ".config.yml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "login:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "login:"))
		}
	}
	return ""
}

func configureGlobalGitignore() {
	// 1. Get current core.excludesFile
	out, err := exec.Command("git", "config", "--global", "core.excludesFile").Output()
	excludesFile := strings.TrimSpace(string(out))

	if err != nil || excludesFile == "" {
		// Set default global gitignore path
		home, _ := os.UserHomeDir()
		excludesFile = filepath.Join(home, ".gitignore_global")
		if err := exec.Command("git", "config", "--global", "core.excludesFile", excludesFile).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "%s 글로벌 gitignore 설정 실패: %v\n", redStyle.Render("✗"), err)
			return
		}
	}

	// Expand ~ if present
	if strings.HasPrefix(excludesFile, "~/") {
		home, _ := os.UserHomeDir()
		excludesFile = filepath.Join(home, excludesFile[2:])
	}

	// 2. Check if .grove is already in the file
	data, _ := os.ReadFile(excludesFile)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == ".grove" {
			fmt.Fprintf(os.Stderr, "%s 글로벌 gitignore에 .grove 등록됨\n", "✅")
			return
		}
	}

	// 3. Append .grove
	f, err := os.OpenFile(excludesFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s 글로벌 gitignore 수정 실패: %v\n", redStyle.Render("✗"), err)
		return
	}
	defer f.Close()

	entry := ".grove\n"
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		entry = "\n" + entry
	}
	if _, err := f.WriteString(entry); err != nil {
		fmt.Fprintf(os.Stderr, "%s 글로벌 gitignore 수정 실패: %v\n", redStyle.Render("✗"), err)
		return
	}

	fmt.Fprintf(os.Stderr, "%s 글로벌 gitignore에 .grove 추가 완료 → %s\n", "✅", excludesFile)
}

const owFunctionMarker = "# grove: ow function"

const owFunction = `
# grove: ow function
# 워크트리로 이동: ow 12345 또는 ow PROJ-12345
ow() {
  local target="$1"
  if [ -z "$target" ]; then
    echo "Usage: ow <issue-number-or-key>" >&2
    return 1
  fi

  # 현재 디렉토리 기준으로 git 레포지토리 루트 찾기
  local repo_root
  repo_root=$(git rev-parse --show-toplevel 2>/dev/null)
  if [ $? -ne 0 ]; then
    echo "git 레포지토리를 찾을 수 없습니다" >&2
    return 1
  fi

  # grove select를 레포지토리 루트에서 호출해서 선택된 경로를 받음
  local selected_path
  selected_path=$(cd "$repo_root" && grove select "$target" 2>&1)
  if [ $? -eq 0 ]; then
    cd "$selected_path"
  else
    # stderr로 에러 메시지 출력
    echo "$selected_path" >&2
    return 1
  fi
}
`

func installShellFunction() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	zshrc := filepath.Join(home, ".zshrc")

	data, err := os.ReadFile(zshrc)

	// 이미 최신 내용으로 등록된 경우 skip
	if err == nil && strings.Contains(string(data), strings.TrimLeft(owFunction, "\n")) {
		fmt.Fprintf(os.Stderr, "%s shell function (ow) 등록됨\n", "✅")
		return
	}

	// 구버전이 있으면 최신 내용으로 교체
	if err == nil && strings.Contains(string(data), owFunctionMarker) {
		re := regexp.MustCompile(`(?s)# grove: ow function.*?\n}\n`)
		newContent := re.ReplaceAllString(string(data), strings.TrimLeft(owFunction, "\n"))
		if err := os.WriteFile(zshrc, []byte(newContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "%s shell function 업데이트 실패: %v\n", redStyle.Render("✗"), err)
			return
		}
		fmt.Fprintf(os.Stderr, "%s shell function (ow) 업데이트 완료 → %s\n", "✅", "source ~/.zshrc 또는 새 터미널에서 사용 가능")
		return
	}

	// 없으면 새로 append
	f, err := os.OpenFile(zshrc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s shell function 등록 실패: %v\n", redStyle.Render("✗"), err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(owFunction); err != nil {
		fmt.Fprintf(os.Stderr, "%s shell function 등록 실패: %v\n", redStyle.Render("✗"), err)
		return
	}

	fmt.Fprintf(os.Stderr, "%s shell function (ow) 등록 완료 → %s\n", "✅", "source ~/.zshrc 또는 새 터미널에서 사용 가능")
	fmt.Fprintln(os.Stderr, "  사용법: ow 12345 또는 ow PROJ-12345")
}
