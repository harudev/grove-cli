package config

import (
	"fmt"
	"strings"
	"time"
)

// JiraWorkflow maps grove's abstract issue pipeline onto a Jira project's
// concrete status and transition names. It is resolved from the global config
// (~/.config/grove/config.json) overlaid with per-repo overrides
// (<repo>/.grove/config.json).
//
// All fields are optional. When the workflow is not configured, grove's Jira
// status-transition features (jira update/deploy/weekly and the init
// auto-transition) are skipped with a hint, while worktree management keeps
// working. This keeps grove usable across different Jira setups without
// hardcoding any single organization's status names.
type JiraWorkflow struct {
	Statuses    JiraStatusNames     `json:"statuses,omitempty"`
	Transitions JiraTransitionNames `json:"transitions,omitempty"`
	// ExcludedStatuses are omitted from `jira weekly` (e.g. on-hold buckets).
	ExcludedStatuses []string `json:"excludedStatuses,omitempty"`
	// DeployField is the Jira custom field id holding the deploy date, e.g.
	// "customfield_11642". Empty disables all deploy-date features.
	DeployField string `json:"deployField,omitempty"`
	// DeployWeekday is the weekday the deploy-date selector suggests (English
	// name, e.g. "Tuesday"; Korean names also accepted). Empty defaults to
	// Tuesday.
	DeployWeekday string `json:"deployWeekday,omitempty"`
}

// JiraStatusNames holds the project's status names for each pipeline stage,
// listed in pipeline order. Any stage left empty is treated as "not part of
// this project's workflow" for ordering and comparison.
type JiraStatusNames struct {
	InProgress     string `json:"inProgress,omitempty"`
	DevComplete    string `json:"devComplete,omitempty"`
	InReview       string `json:"inReview,omitempty"`
	ReviewComplete string `json:"reviewComplete,omitempty"`
	ReviewPassed   string `json:"reviewPassed,omitempty"`
	// ResolvedClosed is the status an issue lands in after the resolved-close
	// transition (deployed/done). It is treated as terminal.
	ResolvedClosed string `json:"resolvedClosed,omitempty"`
	// Terminal lists additional statuses grove treats as closed/done. Grove
	// also always treats ResolvedClosed and Jira's statusCategory "done" as
	// terminal.
	Terminal []string `json:"terminal,omitempty"`
}

// JiraTransitionNames holds the transition (action) names as they appear in the
// project's Jira transition menu.
type JiraTransitionNames struct {
	InProgress     string `json:"inProgress,omitempty"`
	DevComplete    string `json:"devComplete,omitempty"`
	InReview       string `json:"inReview,omitempty"`
	ReviewComplete string `json:"reviewComplete,omitempty"`
	ResolvedClose  string `json:"resolvedClose,omitempty"`
	Close          string `json:"close,omitempty"`
}

// Configured reports whether any transition is set. When false, callers should
// skip Jira status-transition flows and hint the user to run
// `grove config jira-workflow`.
func (w JiraWorkflow) Configured() bool {
	t := w.Transitions
	return t.InProgress != "" || t.DevComplete != "" || t.InReview != "" ||
		t.ReviewComplete != "" || t.ResolvedClose != "" || t.Close != ""
}

// IsTerminal reports whether a status is closed/done. It matches the configured
// terminal status names and always treats Jira's statusCategory "done" as
// terminal, so it works even before the workflow is fully configured.
func (w JiraWorkflow) IsTerminal(status, statusCategory string) bool {
	if status != "" && status == w.Statuses.ResolvedClosed {
		return true
	}
	for _, s := range w.Statuses.Terminal {
		if s == status {
			return true
		}
	}
	return statusCategory == "done"
}

// statusRank maps each configured pipeline status to its position. Stages keep
// a stable rank even when unset, so comparisons stay consistent regardless of
// which optional stages a project uses.
func (w JiraWorkflow) statusRank() map[string]int {
	order := make(map[string]int)
	stages := []string{
		w.Statuses.InProgress,
		w.Statuses.DevComplete,
		w.Statuses.InReview,
		w.Statuses.ReviewComplete,
		w.Statuses.ReviewPassed,
	}
	for i, s := range stages {
		if s != "" {
			order[s] = i
		}
	}
	if w.Statuses.ResolvedClosed != "" {
		order[w.Statuses.ResolvedClosed] = len(stages)
	}
	for _, s := range w.Statuses.Terminal {
		order[s] = len(stages)
	}
	return order
}

// IsAtOrAfter reports whether current is at or after target in the pipeline.
// Returns false if either status is not part of the configured pipeline.
func (w JiraWorkflow) IsAtOrAfter(current, target string) bool {
	order := w.statusRank()
	c, ok1 := order[current]
	t, ok2 := order[target]
	if !ok1 || !ok2 {
		return false
	}
	return c >= t
}

// IsReviewOrAfter reports whether status is at or after the in-review stage.
func (w JiraWorkflow) IsReviewOrAfter(status string) bool {
	if w.Statuses.InReview == "" {
		return false
	}
	return w.IsAtOrAfter(status, w.Statuses.InReview)
}

// IsExcluded reports whether status is in the excluded (e.g. on-hold) set.
func (w JiraWorkflow) IsExcluded(status string) bool {
	for _, s := range w.ExcludedStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// DeployWeekdayValue returns the configured deploy weekday, defaulting to
// Tuesday. Accepts English and Korean weekday names.
func (w JiraWorkflow) DeployWeekdayValue() time.Weekday {
	switch strings.ToLower(strings.TrimSpace(w.DeployWeekday)) {
	case "sunday", "sun", "일", "일요일":
		return time.Sunday
	case "monday", "mon", "월", "월요일":
		return time.Monday
	case "tuesday", "tue", "tues", "화", "화요일":
		return time.Tuesday
	case "wednesday", "wed", "수", "수요일":
		return time.Wednesday
	case "thursday", "thu", "thur", "thurs", "목", "목요일":
		return time.Thursday
	case "friday", "fri", "금", "금요일":
		return time.Friday
	case "saturday", "sat", "토", "토요일":
		return time.Saturday
	default:
		return time.Tuesday
	}
}

// GetJiraWorkflow resolves the workflow for the current repository.
func GetJiraWorkflow() JiraWorkflow {
	return ResolveJiraWorkflow(currentRepoDir())
}

// ResolveJiraWorkflow returns the global workflow overlaid with repoDir's
// per-field overrides (repo values win when non-empty).
func ResolveJiraWorkflow(repoDir string) JiraWorkflow {
	var wf JiraWorkflow
	if g := Load().JiraWorkflow; g != nil {
		wf = *g
	}
	if repoDir != "" {
		if r := LoadRepoConfig(repoDir).JiraWorkflow; r != nil {
			wf = mergeWorkflow(wf, *r)
		}
	}
	return wf
}

// ExampleJiraWorkflow returns a template workflow with placeholder names, used
// by scaffolding. Users edit these to match their own Jira project's status and
// transition names.
func ExampleJiraWorkflow() JiraWorkflow {
	return JiraWorkflow{
		Statuses: JiraStatusNames{
			InProgress:     "In Progress",
			DevComplete:    "In Review",
			InReview:       "In QA",
			ReviewComplete: "QA Passed",
			ReviewPassed:   "Ready to Deploy",
			ResolvedClosed: "Done",
			Terminal:       []string{"Done", "Closed", "Won't Do"},
		},
		Transitions: JiraTransitionNames{
			InProgress:     "In Progress",
			DevComplete:    "In Review",
			InReview:       "In QA",
			ReviewComplete: "QA Passed",
			ResolvedClose:  "Done",
			Close:          "Closed",
		},
		ExcludedStatuses: []string{"On Hold", "Blocked"},
		DeployField:      "customfield_XXXXX",
		DeployWeekday:    "Tuesday",
	}
}

// ScaffoldGlobalJiraWorkflow writes an example workflow into the global config
// (only if not already set) and returns the config file path.
func ScaffoldGlobalJiraWorkflow() (string, error) {
	cfg := Load()
	if cfg.JiraWorkflow == nil {
		ex := ExampleJiraWorkflow()
		cfg.JiraWorkflow = &ex
	}
	if err := Save(cfg); err != nil {
		return "", err
	}
	return configPath(), nil
}

// ScaffoldRepoJiraWorkflow writes an example workflow into repoDir's config
// (only if not already set) and returns the config file path.
func ScaffoldRepoJiraWorkflow(repoDir string) (string, error) {
	if repoDir == "" {
		return "", fmt.Errorf("git 레포지토리 안에서 실행해야 레포별 워크플로우를 설정할 수 있습니다")
	}
	rc := LoadRepoConfig(repoDir)
	if rc.JiraWorkflow == nil {
		ex := ExampleJiraWorkflow()
		rc.JiraWorkflow = &ex
	}
	if err := SaveRepoConfig(repoDir, rc); err != nil {
		return "", err
	}
	return repoConfigPath(repoDir), nil
}

// mergeWorkflow overlays non-empty fields of over onto base.
func mergeWorkflow(base, over JiraWorkflow) JiraWorkflow {
	out := base

	s, os := &out.Statuses, over.Statuses
	if os.InProgress != "" {
		s.InProgress = os.InProgress
	}
	if os.DevComplete != "" {
		s.DevComplete = os.DevComplete
	}
	if os.InReview != "" {
		s.InReview = os.InReview
	}
	if os.ReviewComplete != "" {
		s.ReviewComplete = os.ReviewComplete
	}
	if os.ReviewPassed != "" {
		s.ReviewPassed = os.ReviewPassed
	}
	if os.ResolvedClosed != "" {
		s.ResolvedClosed = os.ResolvedClosed
	}
	if len(os.Terminal) > 0 {
		s.Terminal = os.Terminal
	}

	t, ot := &out.Transitions, over.Transitions
	if ot.InProgress != "" {
		t.InProgress = ot.InProgress
	}
	if ot.DevComplete != "" {
		t.DevComplete = ot.DevComplete
	}
	if ot.InReview != "" {
		t.InReview = ot.InReview
	}
	if ot.ReviewComplete != "" {
		t.ReviewComplete = ot.ReviewComplete
	}
	if ot.ResolvedClose != "" {
		t.ResolvedClose = ot.ResolvedClose
	}
	if ot.Close != "" {
		t.Close = ot.Close
	}

	if len(over.ExcludedStatuses) > 0 {
		out.ExcludedStatuses = over.ExcludedStatuses
	}
	if over.DeployField != "" {
		out.DeployField = over.DeployField
	}
	if over.DeployWeekday != "" {
		out.DeployWeekday = over.DeployWeekday
	}
	return out
}
