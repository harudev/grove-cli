package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RestAdapter calls the Jira REST API directly (fallback for jira-cli rate limits).
type RestAdapter struct {
	Server      string // e.g. "https://mycompany.atlassian.net"
	Login       string // e.g. "user@company.com"
	Token       string // Atlassian API token
	DeployField string // deploy-date custom field id, e.g. "customfield_11642"; "" disables deploy features
	client      *http.Client
}

// NewRestAdapter creates a new RestAdapter. deployField is the custom field id
// holding the deploy date ("" disables deploy-date operations).
func NewRestAdapter(server, login, token, deployField string) *RestAdapter {
	return &RestAdapter{
		Server:      strings.TrimRight(server, "/"),
		Login:       login,
		Token:       token,
		DeployField: deployField,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

// Available returns true if server, login, and token are all configured.
func (a *RestAdapter) Available() bool {
	return a.Server != "" && a.Login != "" && a.Token != ""
}

func (a *RestAdapter) ViewIssue(key string) (*Issue, error) {
	// v2 API returns description as plain wiki markup instead of ADF.
	fields := "summary,description,status,parent,updated,assignee"
	if a.DeployField != "" {
		fields += "," + a.DeployField
	}
	url := fmt.Sprintf("%s/rest/api/2/issue/%s?fields=%s", a.Server, key, fields)
	body, err := a.get(url)
	if err != nil {
		return nil, fmt.Errorf("REST view issue %s: %w", key, err)
	}

	var raw struct {
		Fields struct {
			Summary     string  `json:"summary"`
			Description *string `json:"description"`
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
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("REST parse issue %s: %w", key, err)
	}

	issue := &Issue{
		Key:            key,
		Summary:        raw.Fields.Summary,
		Status:         raw.Fields.Status.Name,
		StatusCategory: raw.Fields.Status.StatusCategory.Key,
	}

	if raw.Fields.Description != nil {
		issue.Description = *raw.Fields.Description
	}

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

	issue.DeployDate = deployDateFromBody(body, a.DeployField)

	issue.Updated = raw.Fields.Updated

	return issue, nil
}

func (a *RestAdapter) GetStatus(key string) (string, error) {
	issue, err := a.ViewIssue(key)
	if err != nil {
		return "", err
	}
	return issue.Status, nil
}

func (a *RestAdapter) ListChildren(parentKey string) ([]Issue, error) {
	jql := fmt.Sprintf("parent=%s", parentKey)
	url := fmt.Sprintf("%s/rest/api/3/search/jql?jql=%s&fields=summary,status", a.Server, jql)
	body, err := a.get(url)
	if err != nil {
		return nil, fmt.Errorf("REST list children of %s: %w", parentKey, err)
	}

	var raw struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary string `json:"summary"`
				Status  struct {
					Name string `json:"name"`
				} `json:"status"`
			} `json:"fields"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var issues []Issue
	for _, ri := range raw.Issues {
		issues = append(issues, Issue{Key: ri.Key, Summary: ri.Fields.Summary, Status: ri.Fields.Status.Name})
	}
	return issues, nil
}

// SearchIssues searches issues using a JQL query via REST API.
func (a *RestAdapter) SearchIssues(jql string) ([]Issue, error) {
	fields := "summary,status"
	if a.DeployField != "" {
		fields += "," + a.DeployField
	}
	url := fmt.Sprintf("%s/rest/api/3/search/jql?jql=%s&fields=%s&maxResults=50",
		a.Server, jql, fields)
	body, err := a.get(url)
	if err != nil {
		return nil, fmt.Errorf("REST search issues: %w", err)
	}

	var raw struct {
		Issues []struct {
			Key    string          `json:"key"`
			Fields json.RawMessage `json:"fields"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var issues []Issue
	for _, ri := range raw.Issues {
		var f struct {
			Summary string `json:"summary"`
			Status  struct {
				Name string `json:"name"`
			} `json:"status"`
		}
		if err := json.Unmarshal(ri.Fields, &f); err != nil {
			continue
		}
		issue := Issue{
			Key:     ri.Key,
			Summary: f.Summary,
			Status:  f.Status.Name,
		}
		var fieldMap map[string]json.RawMessage
		if json.Unmarshal(ri.Fields, &fieldMap) == nil {
			issue.DeployDate = deployDateFromFields(fieldMap, a.DeployField)
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// TransitionIssue moves an issue using the Jira REST API.
func (a *RestAdapter) TransitionIssue(key, transitionName string) error {
	// 1. Get available transitions
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", a.Server, key)
	body, err := a.get(url)
	if err != nil {
		return fmt.Errorf("REST get transitions %s: %w", key, err)
	}

	var raw struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("REST parse transitions: %w", err)
	}

	// 2. Find matching transition
	var transitionID string
	for _, t := range raw.Transitions {
		if t.Name == transitionName {
			transitionID = t.ID
			break
		}
	}
	if transitionID == "" {
		var available []string
		for _, t := range raw.Transitions {
			available = append(available, t.Name)
		}
		return fmt.Errorf("전이 '%s'을 찾을 수 없습니다 (사용 가능: %v)", transitionName, available)
	}

	// 3. Execute transition
	payload := fmt.Sprintf(`{"transition":{"id":"%s"}}`, transitionID)
	return a.post(url, payload)
}

// GetDeployDate returns the deploy date custom field as "YYYY-MM-DD" or "".
func (a *RestAdapter) GetDeployDate(key string) (string, error) {
	if a.DeployField == "" {
		return "", fmt.Errorf("배포일자 커스텀 필드가 설정되지 않았습니다 (grove config jira-workflow)")
	}
	url := fmt.Sprintf("%s/rest/api/2/issue/%s?fields=%s", a.Server, key, a.DeployField)
	body, err := a.get(url)
	if err != nil {
		return "", fmt.Errorf("REST get deploy date %s: %w", key, err)
	}
	return deployDateFromBody(body, a.DeployField), nil
}

// SetDeployDate sets the deploy date custom field on an issue.
func (a *RestAdapter) SetDeployDate(key, date string) error {
	if a.DeployField == "" {
		return fmt.Errorf("배포일자 커스텀 필드가 설정되지 않았습니다 (grove config jira-workflow)")
	}
	url := fmt.Sprintf("%s/rest/api/2/issue/%s", a.Server, key)
	payload := fmt.Sprintf(`{"fields":{"%s":"%s"}}`, a.DeployField, date)

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(a.Login + ":" + a.Token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira REST API error %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return nil
}

// AddLabel adds a label to an issue using the REST API.
func (a *RestAdapter) AddLabel(key, label string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", a.Server, key)
	payload := fmt.Sprintf(`{"update":{"labels":[{"add":"%s"}]}}`, label)

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(a.Login + ":" + a.Token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira REST API error %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return nil
}

// AddComment adds a comment to an issue using the REST API.
func (a *RestAdapter) AddComment(key, body string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/comment", a.Server, key)

	// Build Atlassian Document Format payload
	// Split body into paragraphs by newline
	var paragraphs []map[string]interface{}
	for _, line := range strings.Split(body, "\n") {
		paragraphs = append(paragraphs, map[string]interface{}{
			"type": "paragraph",
			"content": []map[string]interface{}{
				{"type": "text", "text": line},
			},
		})
	}

	payload := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": paragraphs,
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal comment: %w", err)
	}

	return a.post(url, string(jsonBytes))
}

func (a *RestAdapter) post(url, jsonBody string) error {
	req, err := http.NewRequest("POST", url, strings.NewReader(jsonBody))
	if err != nil {
		return err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(a.Login + ":" + a.Token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira REST API error %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return nil
}

func (a *RestAdapter) get(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Basic auth: email:api_token
	auth := base64.StdEncoding.EncodeToString([]byte(a.Login + ":" + a.Token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("Jira REST API rate limit (429)")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Jira REST API error %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	return body, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
