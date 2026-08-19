package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// AIToolPatterns are file/directory patterns to copy from main repo to worktree.
var AIToolPatterns = []string{".claude", ".zed", ".cursorrules", "AGENTS.md"}

// DefaultBranchCandidates are tried in order to detect the default branch.
var DefaultBranchCandidates = []string{"mainline", "main"}

// Branch creation strategies. A type's strategy decides how its worktree is created.
const (
	StrategyFromDefault      = "from-default"      // base off the repo's default branch
	StrategyFromBranch       = "from-branch"       // base off a branch (pinned via Base, else picked at runtime)
	StrategyFromTag          = "from-tag"          // base off a git tag (pinned via Base, else picked at runtime)
	StrategyCheckoutExisting = "checkout-existing" // check out an existing remote branch (no new branch)
)

// BranchStrategies lists all valid strategy values.
var BranchStrategies = []string{
	StrategyFromDefault, StrategyFromBranch, StrategyFromTag, StrategyCheckoutExisting,
}

// BranchTypeDef defines a branch type's policy. The full list of these in a repo's
// .grove/config.json IS the repo's branching policy.
type BranchTypeDef struct {
	Name        string `json:"name"`                  // e.g. "feature"
	Description string `json:"description,omitempty"` // shown in the selection menu
	Prefix      string `json:"prefix,omitempty"`      // branch name prefix; defaults to Name
	Strategy    string `json:"strategy"`              // one of BranchStrategies
	Base        string `json:"base,omitempty"`        // pinned base ref (branch for from-branch, tag for from-tag)
}

// EffectivePrefix returns the branch name prefix, defaulting to Name when unset.
func (d BranchTypeDef) EffectivePrefix() string {
	if d.Prefix != "" {
		return d.Prefix
	}
	return d.Name
}

// DefaultBranchTypeDefs is the built-in policy used when a repo has no branchTypes config.
var DefaultBranchTypeDefs = []BranchTypeDef{
	{Name: "feature", Description: "일반 기능 개발", Strategy: StrategyFromDefault},
	{Name: "big-feature", Description: "대규모 기능 개발", Strategy: StrategyFromDefault},
	{Name: "sub-feature", Description: "하위 기능 개발", Strategy: StrategyFromBranch},
	{Name: "bugfix", Description: "버그 수정", Strategy: StrategyFromBranch},
	{Name: "hotfix", Description: "긴급 수정", Strategy: StrategyFromTag},
	{Name: "local-review", Description: "리모트 브랜치 로컬 리뷰", Strategy: StrategyCheckoutExisting},
}

// GroveConfig holds persistent grove configuration (global, ~/.config/grove/config.json).
// Only genuinely global settings live here. Repo-configurable settings (issue
// prefix, local file patterns, post-checkout commands, branching policy) live in
// RepoConfig and are no longer stored globally.
type GroveConfig struct {
	JiraServer   string `json:"jiraServer,omitempty"`   // e.g. "https://mycompany.atlassian.net"
	JiraLogin    string `json:"jiraLogin,omitempty"`    // e.g. "user@company.com"
	JiraAPIToken string `json:"jiraApiToken,omitempty"` // Atlassian API token
	// JiraWorkflow maps grove's pipeline onto this Jira instance's status and
	// transition names. Shared across repos; can be overridden per repo.
	JiraWorkflow *JiraWorkflow `json:"jiraWorkflow,omitempty"`
}

// RepoConfig holds per-repository configuration (<repo>/.grove/config.json).
type RepoConfig struct {
	IssuePrefix          string          `json:"issuePrefix,omitempty"`          // Jira/issue key prefix for this repo (e.g. "PROJ")
	LocalFilePatterns    []string        `json:"localFilePatterns,omitempty"`    // patterns to copy from main repo into worktrees (e.g. ".env.local")
	PostCheckoutCommands []string        `json:"postCheckoutCommands,omitempty"` // extra commands to run after worktree checkout
	BranchTypes          []BranchTypeDef `json:"branchTypes,omitempty"`          // per-repo branching policy (full source of truth when set)

	// JiraWorkflow overrides the global workflow for this repo (per-field).
	JiraWorkflow *JiraWorkflow `json:"jiraWorkflow,omitempty"`

	// Legacy per-type override maps. Honored as a read-only overlay onto
	// DefaultBranchTypeDefs when branchTypes is unset. No longer written.
	BaseBranches   map[string]string `json:"baseBranches,omitempty"`
	BranchPrefixes map[string]string `json:"branchPrefixes,omitempty"`
}

// configDir returns ~/.config/grove/
func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "grove")
}

// configPath returns ~/.config/grove/config.json
func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

// Load reads the config file. Returns defaults if not found.
func Load() *GroveConfig {
	cfg := &GroveConfig{}
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, cfg)
	return cfg
}

// Save writes the config file.
func Save(cfg *GroveConfig) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}

// currentRepoDir resolves the main repository root for the current working
// directory. It follows worktrees back to the primary checkout (via the shared
// git-common-dir) so that the per-repo config (.grove/config.json) is the same
// whether grove runs from the main repo, a subdirectory, or a worktree.
// Returns "" when not inside a git repository.
func currentRepoDir() string {
	out, err := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		// Older git without --path-format: fall back and resolve manually.
		out, err = exec.Command("git", "rev-parse", "--git-common-dir").Output()
		if err != nil {
			return ""
		}
	}
	common := strings.TrimSpace(string(out))
	if common == "" {
		return ""
	}
	if !filepath.IsAbs(common) {
		cwd, err := os.Getwd()
		if err != nil {
			return ""
		}
		common = filepath.Join(cwd, common)
	}
	// common points at <repoRoot>/.git; its parent is the repo root.
	return filepath.Dir(filepath.Clean(common))
}

// GetIssuePrefix returns the issue prefix for the current repository.
// Returns empty string if not configured.
func GetIssuePrefix() string {
	return GetIssuePrefixForRepo(currentRepoDir())
}

// GetIssuePrefixForRepo returns the issue prefix configured for repoDir.
// The prefix lives per-repo in <repo>/.grove/config.json; returns "" if unset
// or when not inside a repo.
func GetIssuePrefixForRepo(repoDir string) string {
	if repoDir == "" {
		return ""
	}
	return LoadRepoConfig(repoDir).IssuePrefix
}

// SetIssuePrefix saves the issue prefix to the current repository's config.
func SetIssuePrefix(prefix string) error {
	return SetIssuePrefixForRepo(currentRepoDir(), prefix)
}

// SetIssuePrefixForRepo saves the issue prefix to repoDir's config.
func SetIssuePrefixForRepo(repoDir, prefix string) error {
	if repoDir == "" {
		return fmt.Errorf("git 레포지토리 안에서 실행해야 prefix를 설정할 수 있습니다")
	}
	rc := LoadRepoConfig(repoDir)
	rc.IssuePrefix = strings.ToUpper(strings.TrimSpace(prefix))
	return SaveRepoConfig(repoDir, rc)
}

// SetJiraConfig saves Jira server, login, and API token.
func SetJiraConfig(server, login, token string) error {
	cfg := Load()
	cfg.JiraServer = strings.TrimSpace(server)
	cfg.JiraLogin = strings.TrimSpace(login)
	cfg.JiraAPIToken = strings.TrimSpace(token)
	return Save(cfg)
}

// GetJiraConfig returns Jira server, login, and API token.
func GetJiraConfig() (server, login, token string) {
	cfg := Load()
	return cfg.JiraServer, cfg.JiraLogin, cfg.JiraAPIToken
}

// ExportJiraToken sets JIRA_API_TOKEN env var if configured and not already set.
// This makes jira-cli use the grove-managed token.
func ExportJiraToken() {
	if os.Getenv("JIRA_API_TOKEN") != "" {
		return // env var already set, don't override
	}
	cfg := Load()
	if cfg.JiraAPIToken != "" {
		os.Setenv("JIRA_API_TOKEN", cfg.JiraAPIToken)
	}
}

// GetLocalFilePatterns returns the local file patterns configured for repoDir.
func GetLocalFilePatterns(repoDir string) []string {
	return LoadRepoConfig(repoDir).LocalFilePatterns
}

// AddLocalFilePattern adds a pattern to repoDir's localFilePatterns (no duplicates).
func AddLocalFilePattern(repoDir, pattern string) error {
	if repoDir == "" {
		return fmt.Errorf("git 레포지토리 안에서 실행해야 로컬 파일 패턴을 설정할 수 있습니다")
	}
	cfg := LoadRepoConfig(repoDir)
	pattern = strings.TrimSpace(pattern)
	for _, p := range cfg.LocalFilePatterns {
		if p == pattern {
			return nil // already exists
		}
	}
	cfg.LocalFilePatterns = append(cfg.LocalFilePatterns, pattern)
	return SaveRepoConfig(repoDir, cfg)
}

// RemoveLocalFilePattern removes a pattern from repoDir's localFilePatterns.
func RemoveLocalFilePattern(repoDir, pattern string) error {
	cfg := LoadRepoConfig(repoDir)
	pattern = strings.TrimSpace(pattern)
	var filtered []string
	for _, p := range cfg.LocalFilePatterns {
		if p != pattern {
			filtered = append(filtered, p)
		}
	}
	cfg.LocalFilePatterns = filtered
	return SaveRepoConfig(repoDir, cfg)
}

// legacyGlobalConfig reads global-config fields that have since moved to the
// per-repo config, so their existing values can be migrated once.
type legacyGlobalConfig struct {
	IssuePrefix       string   `json:"issuePrefix"`
	LocalFilePatterns []string `json:"localFilePatterns"`
}

func loadLegacyGlobal() legacyGlobalConfig {
	var lc legacyGlobalConfig
	data, err := os.ReadFile(configPath())
	if err != nil {
		return lc
	}
	json.Unmarshal(data, &lc)
	return lc
}

// repoConfigExists reports whether repoDir already has a .grove/config.json.
func repoConfigExists(repoDir string) bool {
	if repoDir == "" {
		return false
	}
	_, err := os.Stat(repoConfigPath(repoDir))
	return err == nil
}

// MigrateGlobalToRepo migrates settings that used to live in the global config
// (issuePrefix, localFilePatterns) into repoDir's config.
//
// To avoid silently seeding a brand-new repository with another project's
// settings, it only runs for repos grove already manages (those with an existing
// .grove/config.json) and only fills fields the repo has not set itself.
//
// Once the repo has absorbed the legacy values, the legacy keys are stripped
// from the global file (preserving the global Jira settings). The order matters:
// the global file is cleaned only AFTER a successful copy into the repo, so a
// not-yet-migrated repo never finds the global seed already gone.
func MigrateGlobalToRepo(repoDir string) {
	if !repoConfigExists(repoDir) {
		return
	}
	lg := loadLegacyGlobal()
	if lg.IssuePrefix == "" && len(lg.LocalFilePatterns) == 0 {
		return // global already clean
	}

	rc := LoadRepoConfig(repoDir)
	changed := false
	if rc.IssuePrefix == "" && lg.IssuePrefix != "" {
		rc.IssuePrefix = lg.IssuePrefix
		changed = true
	}
	if rc.LocalFilePatterns == nil && len(lg.LocalFilePatterns) > 0 {
		rc.LocalFilePatterns = append([]string(nil), lg.LocalFilePatterns...)
		changed = true
	}
	if changed {
		if err := SaveRepoConfig(repoDir, rc); err != nil {
			return
		}
	}

	// Repo now owns the legacy settings (either just copied, or it already had
	// its own values). Drop the legacy keys from the global file by rewriting it
	// through the Jira-only GroveConfig schema.
	_ = Save(Load())
}

// MigrateCurrentRepo migrates legacy global settings into the current repo.
func MigrateCurrentRepo() {
	MigrateGlobalToRepo(currentRepoDir())
}

// repoConfigPath returns <repoDir>/.grove/config.json
func repoConfigPath(repoDir string) string {
	return filepath.Join(repoDir, ".grove", "config.json")
}

// LoadRepoConfig reads the repo-local config. Returns defaults if not found.
func LoadRepoConfig(repoDir string) *RepoConfig {
	cfg := &RepoConfig{}
	data, err := os.ReadFile(repoConfigPath(repoDir))
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, cfg)
	return cfg
}

// SaveRepoConfig writes the repo-local config.
func SaveRepoConfig(repoDir string, cfg *RepoConfig) error {
	dir := filepath.Join(repoDir, ".grove")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(repoConfigPath(repoDir), data, 0o644)
}

// GetPostCheckoutCommands returns the configured post-checkout commands for a repo.
func GetPostCheckoutCommands(repoDir string) []string {
	return LoadRepoConfig(repoDir).PostCheckoutCommands
}

// AddPostCheckoutCommand adds a command to the repo's postCheckoutCommands (no duplicates).
func AddPostCheckoutCommand(repoDir, command string) error {
	cfg := LoadRepoConfig(repoDir)
	command = strings.TrimSpace(command)
	for _, c := range cfg.PostCheckoutCommands {
		if c == command {
			return nil
		}
	}
	cfg.PostCheckoutCommands = append(cfg.PostCheckoutCommands, command)
	return SaveRepoConfig(repoDir, cfg)
}

// RemovePostCheckoutCommand removes a command from the repo's postCheckoutCommands.
func RemovePostCheckoutCommand(repoDir, command string) error {
	cfg := LoadRepoConfig(repoDir)
	command = strings.TrimSpace(command)
	var filtered []string
	for _, c := range cfg.PostCheckoutCommands {
		if c != command {
			filtered = append(filtered, c)
		}
	}
	cfg.PostCheckoutCommands = filtered
	return SaveRepoConfig(repoDir, cfg)
}

// ResolveBranchTypes returns the effective branching policy for a repo:
//   - if branchTypes is set in .grove/config.json, it is the full source of truth
//   - otherwise the built-in DefaultBranchTypeDefs, with the legacy
//     baseBranches/branchPrefixes maps applied as a read-only overlay
func ResolveBranchTypes(repoDir string) []BranchTypeDef {
	cfg := LoadRepoConfig(repoDir)

	if len(cfg.BranchTypes) > 0 {
		out := make([]BranchTypeDef, len(cfg.BranchTypes))
		copy(out, cfg.BranchTypes)
		for i := range out {
			if out[i].Prefix == "" {
				out[i].Prefix = out[i].Name
			}
			if out[i].Strategy == "" {
				out[i].Strategy = StrategyFromDefault
			}
		}
		return out
	}

	out := make([]BranchTypeDef, len(DefaultBranchTypeDefs))
	copy(out, DefaultBranchTypeDefs)
	for i := range out {
		out[i].Prefix = out[i].Name
		if p := strings.TrimSpace(cfg.BranchPrefixes[out[i].Name]); p != "" {
			out[i].Prefix = p
		}
		if b := strings.TrimSpace(cfg.BaseBranches[out[i].Name]); b != "" {
			// A pinned base on a from-default type means "base off this specific branch".
			out[i].Base = b
			if out[i].Strategy == StrategyFromDefault {
				out[i].Strategy = StrategyFromBranch
			}
		}
	}
	return out
}

// GetBranchTypeDef returns the resolved definition for a branch type name.
func GetBranchTypeDef(repoDir, name string) (BranchTypeDef, bool) {
	for _, d := range ResolveBranchTypes(repoDir) {
		if d.Name == name {
			return d, true
		}
	}
	return BranchTypeDef{}, false
}

// BranchTypeNames returns the resolved branch type names in order.
func BranchTypeNames(repoDir string) []string {
	defs := ResolveBranchTypes(repoDir)
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}

// HasBranchTypesConfig reports whether the repo defines an explicit branchTypes policy.
func HasBranchTypesConfig(repoDir string) bool {
	return len(LoadRepoConfig(repoDir).BranchTypes) > 0
}

// materializeBranchTypes ensures cfg.BranchTypes holds the full resolved policy
// (dropping the legacy overlay maps), so individual edits write a complete policy.
func materializeBranchTypes(repoDir string, cfg *RepoConfig) {
	if len(cfg.BranchTypes) == 0 {
		cfg.BranchTypes = ResolveBranchTypes(repoDir)
		cfg.BaseBranches = nil
		cfg.BranchPrefixes = nil
	}
}

// SetBranchTypeBase sets the pinned base ref for a branch type (materializing the
// full policy on first write). Empty base clears the pin.
func SetBranchTypeBase(repoDir, name, base string) error {
	cfg := LoadRepoConfig(repoDir)
	materializeBranchTypes(repoDir, cfg)
	for i := range cfg.BranchTypes {
		if cfg.BranchTypes[i].Name == name {
			cfg.BranchTypes[i].Base = strings.TrimSpace(base)
			return SaveRepoConfig(repoDir, cfg)
		}
	}
	return fmt.Errorf("알 수 없는 타입: %s", name)
}

// SetBranchTypePrefix sets the branch name prefix for a branch type (materializing
// the full policy on first write). Empty/identity prefix resets to the type name.
func SetBranchTypePrefix(repoDir, name, prefix string) error {
	cfg := LoadRepoConfig(repoDir)
	materializeBranchTypes(repoDir, cfg)
	prefix = strings.TrimSpace(prefix)
	for i := range cfg.BranchTypes {
		if cfg.BranchTypes[i].Name == name {
			if prefix == "" || prefix == name {
				cfg.BranchTypes[i].Prefix = ""
			} else {
				cfg.BranchTypes[i].Prefix = prefix
			}
			return SaveRepoConfig(repoDir, cfg)
		}
	}
	return fmt.Errorf("알 수 없는 타입: %s", name)
}

// ScaffoldBranchTypes writes the current effective policy into branchTypes so the
// user can edit it by hand. Returns the written definitions.
func ScaffoldBranchTypes(repoDir string) ([]BranchTypeDef, error) {
	cfg := LoadRepoConfig(repoDir)
	defs := ResolveBranchTypes(repoDir)
	cfg.BranchTypes = defs
	cfg.BaseBranches = nil
	cfg.BranchPrefixes = nil
	if err := SaveRepoConfig(repoDir, cfg); err != nil {
		return nil, err
	}
	return defs, nil
}

// IsConfigured returns true if the issue prefix has been set.
func IsConfigured() bool {
	return GetIssuePrefix() != ""
}

// IssueKeyRegex returns a compiled regex for matching issue keys with the configured prefix.
// e.g. for prefix "PROJ" → `(?i)(PROJ)-(\d+)`
func IssueKeyRegex() *regexp.Regexp {
	prefix := GetIssuePrefix()
	if prefix == "" {
		// Match any UPPERCASE-digits pattern
		return regexp.MustCompile(`([A-Z][A-Z0-9]+)-(\d+)`)
	}
	return regexp.MustCompile(fmt.Sprintf(`(?i)(%s)-(\d+)`, regexp.QuoteMeta(prefix)))
}

// IssueKeyExtractRegex returns a regex for extracting full issue keys (PREFIX-NUMBER)
// from existing text such as branch names. It always matches any prefix, not just the
// repo's configured one, since a repo's worktrees commonly mix issue keys from several
// projects (e.g. PROJ, TEAM, QA) while only one prefix can be configured per repo.
func IssueKeyExtractRegex() *regexp.Regexp {
	return regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)
}

// FormatIssueKey formats an issue key from a number using the configured prefix.
func FormatIssueKey(number string) string {
	prefix := GetIssuePrefix()
	if prefix == "" {
		return number
	}
	return fmt.Sprintf("%s-%s", prefix, number)
}

// FormatBranchName formats a branch name: {branchType}/{PREFIX}-{number}.
// Note: this uses the branch type verbatim as the prefix. Prefer the repo-aware
// FormatBranchNameRepo when a repoDir is available so per-repo prefix overrides apply.
func FormatBranchName(branchType, issueKey string) string {
	return fmt.Sprintf("%s/%s", branchType, issueKey)
}

// GetBranchPrefix returns the resolved branch name prefix for a branch type.
// Falls back to the branch type name when the type is unknown.
func GetBranchPrefix(repoDir, branchType string) string {
	if d, ok := GetBranchTypeDef(repoDir, branchType); ok {
		return d.EffectivePrefix()
	}
	return branchType
}

// FormatBranchNameRepo formats {prefix}/{issueKey} using the repo's resolved prefix.
func FormatBranchNameRepo(repoDir, branchType, issueKey string) string {
	return fmt.Sprintf("%s/%s", GetBranchPrefix(repoDir, branchType), issueKey)
}

// BranchTypeFromName maps a branch name's prefix back to its branch type name,
// honoring the repo's resolved policy. Returns the raw prefix if no type matches.
func BranchTypeFromName(repoDir, branchName string) string {
	prefix := strings.SplitN(branchName, "/", 2)[0]
	for _, d := range ResolveBranchTypes(repoDir) {
		if d.EffectivePrefix() == prefix {
			return d.Name
		}
	}
	return prefix
}
