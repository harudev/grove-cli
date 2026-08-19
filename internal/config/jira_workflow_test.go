package config

import (
	"testing"
	"time"
)

func sampleWorkflow() JiraWorkflow {
	return JiraWorkflow{
		Statuses: JiraStatusNames{
			InProgress:     "In Progress",
			DevComplete:    "In Review",
			InReview:       "In QA",
			ReviewComplete: "QA Passed",
			ReviewPassed:   "Ready to Deploy",
			ResolvedClosed: "Done",
			Terminal:       []string{"Closed", "Won't Do"},
		},
		Transitions: JiraTransitionNames{
			InProgress:    "Start",
			DevComplete:   "To Review",
			InReview:      "To QA",
			ResolvedClose: "Resolve",
		},
		ExcludedStatuses: []string{"On Hold"},
		DeployField:      "customfield_10001",
	}
}

func TestConfigured(t *testing.T) {
	if (JiraWorkflow{}).Configured() {
		t.Fatal("empty workflow should not be Configured()")
	}
	if !sampleWorkflow().Configured() {
		t.Fatal("sample workflow should be Configured()")
	}
}

func TestIsTerminal(t *testing.T) {
	w := sampleWorkflow()
	cases := []struct {
		status, category string
		want             bool
	}{
		{"Done", "", true},         // ResolvedClosed
		{"Closed", "", true},       // Terminal list
		{"Won't Do", "", true},     // Terminal list
		{"In QA", "", false},       // pipeline, not terminal
		{"Anything", "done", true}, // statusCategory fallback
		{"In Progress", "indeterminate", false},
	}
	for _, c := range cases {
		if got := w.IsTerminal(c.status, c.category); got != c.want {
			t.Errorf("IsTerminal(%q,%q) = %v, want %v", c.status, c.category, got, c.want)
		}
	}
}

func TestIsAtOrAfter(t *testing.T) {
	w := sampleWorkflow()
	cases := []struct {
		current, target string
		want            bool
	}{
		{"In QA", "In Review", true},         // rank 2 >= 1
		{"In Review", "In QA", false},        // rank 1 < 2
		{"In Progress", "In Progress", true}, // equal
		{"Done", "In QA", true},              // terminal after pipeline
		{"Unknown", "In QA", false},          // not in pipeline
		{"In QA", "Unknown", false},          // target not in pipeline
	}
	for _, c := range cases {
		if got := w.IsAtOrAfter(c.current, c.target); got != c.want {
			t.Errorf("IsAtOrAfter(%q,%q) = %v, want %v", c.current, c.target, got, c.want)
		}
	}
}

func TestIsReviewOrAfter(t *testing.T) {
	w := sampleWorkflow()
	if !w.IsReviewOrAfter("In QA") { // == InReview stage
		t.Error("In QA should be review-or-after")
	}
	if !w.IsReviewOrAfter("Done") { // terminal, after review
		t.Error("Done should be review-or-after")
	}
	if w.IsReviewOrAfter("In Progress") {
		t.Error("In Progress should not be review-or-after")
	}
	// An unconfigured InReview stage disables the check.
	if (JiraWorkflow{}).IsReviewOrAfter("anything") {
		t.Error("empty workflow should not report review-or-after")
	}
}

func TestDeployWeekdayValue(t *testing.T) {
	cases := map[string]time.Weekday{
		"":         time.Tuesday, // default
		"Tuesday":  time.Tuesday,
		"tuesday":  time.Tuesday,
		"화":        time.Tuesday,
		"화요일":      time.Tuesday,
		"Monday":   time.Monday,
		"금요일":      time.Friday,
		"nonsense": time.Tuesday, // fallback
	}
	for in, want := range cases {
		if got := (JiraWorkflow{DeployWeekday: in}).DeployWeekdayValue(); got != want {
			t.Errorf("DeployWeekdayValue(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMergeWorkflow(t *testing.T) {
	base := JiraWorkflow{
		Statuses:      JiraStatusNames{InProgress: "A", InReview: "B"},
		Transitions:   JiraTransitionNames{InReview: "toB"},
		DeployField:   "customfield_1",
		DeployWeekday: "Tuesday",
	}
	over := JiraWorkflow{
		Statuses:      JiraStatusNames{InReview: "B2"}, // override only InReview
		DeployWeekday: "Friday",
	}
	got := mergeWorkflow(base, over)

	if got.Statuses.InProgress != "A" {
		t.Errorf("InProgress = %q, want A (from base)", got.Statuses.InProgress)
	}
	if got.Statuses.InReview != "B2" {
		t.Errorf("InReview = %q, want B2 (overridden)", got.Statuses.InReview)
	}
	if got.Transitions.InReview != "toB" {
		t.Errorf("transition InReview = %q, want toB (from base)", got.Transitions.InReview)
	}
	if got.DeployField != "customfield_1" {
		t.Errorf("DeployField = %q, want customfield_1 (from base)", got.DeployField)
	}
	if got.DeployWeekday != "Friday" {
		t.Errorf("DeployWeekday = %q, want Friday (overridden)", got.DeployWeekday)
	}
}
