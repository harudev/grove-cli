package jira

// Issue represents a Jira issue.
type Issue struct {
	Key            string
	Summary        string
	Description    string
	Status         string
	StatusCategory string // Jira statusCategory.key: "new", "indeterminate", or "done"
	Assignee       string // email address of the assignee, or ""
	ParentKey      string
	DeployDate     string // "YYYY-MM-DD" or ""
	Updated        string // ISO 8601 timestamp or ""
}
