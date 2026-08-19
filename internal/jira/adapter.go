package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Adapter defines the interface for Jira operations.
type Adapter interface {
	ViewIssue(key string) (*Issue, error)
	GetStatus(key string) (string, error)
	ListChildren(parentKey string) ([]Issue, error)
	SearchIssues(jql string) ([]Issue, error)
	TransitionIssue(key, transitionName string) error
	GetDeployDate(key string) (string, error) // returns "YYYY-MM-DD" or ""
	SetDeployDate(key, date string) error     // date format: "YYYY-MM-DD"
	AddComment(key, body string) error
	AddLabel(key, label string) error
}

// CLIAdapter wraps the jira-cli command.
type CLIAdapter struct {
	DeployField string // deploy-date custom field id, e.g. "customfield_11642"; "" disables deploy features
}

// NewCLIAdapter creates a new CLIAdapter. deployField is the custom field id
// holding the deploy date ("" disables reading it from `issue view --raw`).
func NewCLIAdapter(deployField string) *CLIAdapter {
	return &CLIAdapter{DeployField: deployField}
}

// Available checks if jira-cli is installed.
func (a *CLIAdapter) Available() bool {
	_, err := exec.LookPath("jira")
	return err == nil
}

// ViewIssue retrieves a single issue using jira-cli.
func (a *CLIAdapter) ViewIssue(key string) (*Issue, error) {
	out, err := runJira("issue", "view", key, "--raw")
	if err != nil {
		return nil, fmt.Errorf("view issue %s: %w", key, err)
	}

	var raw struct {
		Fields struct {
			Summary     string          `json:"summary"`
			Description json.RawMessage `json:"description"`
			Status      struct {
				Name           string `json:"name"`
				StatusCategory struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"status"`
			Assignee *struct {
				EmailAddress string `json:"emailAddress"`
				Name         string `json:"name"`
			} `json:"assignee"`
			Parent *struct {
				Key string `json:"key"`
			} `json:"parent"`
			Updated string `json:"updated"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parse issue %s: %w", key, err)
	}

	issue := &Issue{
		Key:            key,
		Summary:        raw.Fields.Summary,
		Status:         raw.Fields.Status.Name,
		StatusCategory: raw.Fields.Status.StatusCategory.Key,
	}

	// Description can be a plain string (Jira Server v2) or ADF object (Jira Cloud v3).
	issue.Description = parseDescription(raw.Fields.Description)
	if raw.Fields.Assignee != nil {
		if raw.Fields.Assignee.EmailAddress != "" {
			issue.Assignee = raw.Fields.Assignee.EmailAddress
		} else {
			issue.Assignee = raw.Fields.Assignee.Name
		}
	}
	if raw.Fields.Parent != nil {
		issue.ParentKey = raw.Fields.Parent.Key
	}
	issue.DeployDate = deployDateFromBody([]byte(out), a.DeployField)
	issue.Updated = raw.Fields.Updated
	return issue, nil
}

// GetStatus retrieves only the status of an issue.
func (a *CLIAdapter) GetStatus(key string) (string, error) {
	issue, err := a.ViewIssue(key)
	if err != nil {
		return "", err
	}
	return issue.Status, nil
}

// ListChildren retrieves child issues of a parent.
func (a *CLIAdapter) ListChildren(parentKey string) ([]Issue, error) {
	jql := fmt.Sprintf("parent = %s", parentKey)
	out, err := runJira("issue", "list",
		"-q", jql,
		"--plain", "--no-headers",
		"--columns", "key,summary,status")
	if err != nil {
		return nil, fmt.Errorf("list children of %s: %w", parentKey, err)
	}

	if out == "" {
		return nil, nil
	}

	var issues []Issue
	for _, line := range splitLines(out) {
		parts := splitTabs(line)
		if len(parts) < 2 {
			continue
		}
		iss := Issue{Key: parts[0], Summary: parts[1]}
		if len(parts) >= 3 {
			iss.Status = parts[2]
		}
		issues = append(issues, iss)
	}
	return issues, nil
}

// SearchIssues searches issues using a JQL query.
func (a *CLIAdapter) SearchIssues(jql string) ([]Issue, error) {
	out, err := runJira("issue", "list",
		"-q", jql,
		"--plain", "--no-headers",
		"--columns", "key,summary,status")
	if err != nil {
		return nil, fmt.Errorf("search issues: %w", err)
	}

	if out == "" {
		return nil, nil
	}

	var issues []Issue
	for _, line := range splitLines(out) {
		parts := splitTabs(line)
		if len(parts) < 3 {
			continue
		}
		issues = append(issues, Issue{
			Key:     parts[0],
			Summary: parts[1],
			Status:  parts[2],
		})
	}
	return issues, nil
}

// TransitionIssue moves an issue to a new status using a transition name.
func (a *CLIAdapter) TransitionIssue(key, transitionName string) error {
	_, err := runJira("issue", "move", key, transitionName)
	if err != nil {
		return fmt.Errorf("transition %s → %s: %w", key, transitionName, err)
	}
	return nil
}

// GetDeployDate returns the deploy date field (customfield_11642) as "YYYY-MM-DD" or "".
// CLIAdapter cannot read custom fields, so it always returns empty.
func (a *CLIAdapter) GetDeployDate(key string) (string, error) {
	return "", fmt.Errorf("CLIAdapter does not support custom fields")
}

// SetDeployDate sets the deploy date field. CLIAdapter does not support custom fields.
func (a *CLIAdapter) SetDeployDate(key, date string) error {
	return fmt.Errorf("CLIAdapter does not support custom fields")
}

// AddLabel adds a label to an issue using jira-cli.
func (a *CLIAdapter) AddLabel(key, label string) error {
	// jira-cli의 edit --label은 기존 라벨을 대체하므로, 기존 라벨을 먼저 읽어서 추가
	issue, err := a.ViewIssue(key)
	if err != nil {
		return fmt.Errorf("add label to %s: %w", key, err)
	}
	// ViewIssue로는 라벨을 못 읽으므로 CLI에서는 불가 → 에러 반환하여 REST fallback
	_ = issue
	return fmt.Errorf("CLIAdapter does not support label operations")
}

// AddComment adds a comment to an issue using jira-cli.
func (a *CLIAdapter) AddComment(key, body string) error {
	_, err := runJira("issue", "comment", "add", key, "-b", body)
	if err != nil {
		return fmt.Errorf("add comment to %s: %w", key, err)
	}
	return nil
}

// parseDescription handles both plain string (Jira Server v2) and
// Atlassian Document Format object (Jira Cloud v3) for the description field.
func parseDescription(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	// Try plain string first (Jira Server v2).
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return plain
	}

	// Try ADF object (Jira Cloud v3).
	var adf adfNode
	if err := json.Unmarshal(raw, &adf); err == nil {
		var sb strings.Builder
		renderADFNode(&sb, &adf, 0)
		return strings.TrimSpace(sb.String())
	}

	return ""
}

// adfNode represents a node in the Atlassian Document Format tree.
type adfNode struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Marks   []adfMark       `json:"marks"`
	Content []adfNode       `json:"content"`
	Attrs   json.RawMessage `json:"attrs"`
}

type adfMark struct {
	Type string `json:"type"`
}

// renderADFNode recursively renders an ADF node to plain text with basic markdown formatting.
func renderADFNode(sb *strings.Builder, node *adfNode, listDepth int) {
	switch node.Type {
	case "text":
		text := node.Text
		for _, m := range node.Marks {
			switch m.Type {
			case "code":
				text = "`" + text + "`"
			case "strong":
				text = "**" + text + "**"
			}
		}
		sb.WriteString(text)

	case "paragraph":
		for _, child := range node.Content {
			renderADFNode(sb, &child, listDepth)
		}
		sb.WriteString("\n")

	case "bulletList", "orderedList":
		for _, child := range node.Content {
			renderADFNode(sb, &child, listDepth+1)
		}

	case "listItem":
		indent := strings.Repeat("  ", listDepth-1)
		sb.WriteString(indent + "- ")
		for i, child := range node.Content {
			if i > 0 && child.Type != "bulletList" && child.Type != "orderedList" {
				sb.WriteString(indent + "  ")
			}
			renderADFNode(sb, &child, listDepth)
		}

	case "codeBlock":
		var lang string
		if len(node.Attrs) > 0 {
			var attrs struct {
				Language string `json:"language"`
			}
			json.Unmarshal(node.Attrs, &attrs)
			lang = attrs.Language
		}
		sb.WriteString("```" + lang + "\n")
		for _, child := range node.Content {
			sb.WriteString(child.Text)
		}
		sb.WriteString("\n```\n")

	case "heading":
		level := 1
		if len(node.Attrs) > 0 {
			var attrs struct {
				Level int `json:"level"`
			}
			if json.Unmarshal(node.Attrs, &attrs) == nil && attrs.Level > 0 {
				level = attrs.Level
			}
		}
		sb.WriteString(strings.Repeat("#", level) + " ")
		for _, child := range node.Content {
			renderADFNode(sb, &child, listDepth)
		}
		sb.WriteString("\n")

	default:
		for _, child := range node.Content {
			renderADFNode(sb, &child, listDepth)
		}
	}
}

// runJira executes a jira-cli command.
func runJira(args ...string) (string, error) {
	cmd := exec.Command("jira", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("jira %v: %w\n%s", args, err, stderr.String())
	}
	return stdout.String(), nil
}

func splitLines(s string) []string {
	var lines []string
	for _, l := range bytes.Split([]byte(s), []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			lines = append(lines, string(l))
		}
	}
	return lines
}

func splitTabs(s string) []string {
	var parts []string
	for _, p := range bytes.Split([]byte(s), []byte("\t")) {
		parts = append(parts, string(bytes.TrimSpace(p)))
	}
	return parts
}
