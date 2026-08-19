package jira

import (
	"fmt"
	"os"
	"strings"
)

// FallbackAdapter tries CLIAdapter first, falls back to RestAdapter on failure.
type FallbackAdapter struct {
	CLI  *CLIAdapter
	REST *RestAdapter
}

// NewFallbackAdapter creates a FallbackAdapter with CLI primary and REST fallback.
func NewFallbackAdapter(cli *CLIAdapter, rest *RestAdapter) *FallbackAdapter {
	return &FallbackAdapter{CLI: cli, REST: rest}
}

func (f *FallbackAdapter) ViewIssue(key string) (*Issue, error) {
	if f.CLI.Available() {
		issue, err := f.CLI.ViewIssue(key)
		if err == nil {
			return issue, nil
		}
		if f.REST.Available() {
			logFallback("ViewIssue", key, err)
			return f.REST.ViewIssue(key)
		}
		return nil, err
	}
	if f.REST.Available() {
		return f.REST.ViewIssue(key)
	}
	return nil, fmt.Errorf("jira-cli와 REST API 모두 사용할 수 없습니다")
}

func (f *FallbackAdapter) GetStatus(key string) (string, error) {
	if f.CLI.Available() {
		status, err := f.CLI.GetStatus(key)
		if err == nil {
			return status, nil
		}
		if f.REST.Available() {
			logFallback("GetStatus", key, err)
			return f.REST.GetStatus(key)
		}
		return "", err
	}
	if f.REST.Available() {
		return f.REST.GetStatus(key)
	}
	return "", fmt.Errorf("jira-cli와 REST API 모두 사용할 수 없습니다")
}

func (f *FallbackAdapter) ListChildren(parentKey string) ([]Issue, error) {
	if f.CLI.Available() {
		issues, err := f.CLI.ListChildren(parentKey)
		if err == nil {
			return issues, nil
		}
		if f.REST.Available() {
			logFallback("ListChildren", parentKey, err)
			return f.REST.ListChildren(parentKey)
		}
		return nil, err
	}
	if f.REST.Available() {
		return f.REST.ListChildren(parentKey)
	}
	return nil, fmt.Errorf("jira-cli와 REST API 모두 사용할 수 없습니다")
}

func (f *FallbackAdapter) SearchIssues(jql string) ([]Issue, error) {
	if f.CLI.Available() {
		issues, err := f.CLI.SearchIssues(jql)
		if err == nil {
			return issues, nil
		}
		if f.REST.Available() {
			logFallback("SearchIssues", "jql", err)
			return f.REST.SearchIssues(jql)
		}
		return nil, err
	}
	if f.REST.Available() {
		return f.REST.SearchIssues(jql)
	}
	return nil, fmt.Errorf("jira-cli와 REST API 모두 사용할 수 없습니다")
}

func (f *FallbackAdapter) TransitionIssue(key, transitionName string) error {
	if f.CLI.Available() {
		err := f.CLI.TransitionIssue(key, transitionName)
		if err == nil {
			return nil
		}
		if f.REST.Available() {
			logFallback("TransitionIssue", key, err)
			return f.REST.TransitionIssue(key, transitionName)
		}
		return err
	}
	if f.REST.Available() {
		return f.REST.TransitionIssue(key, transitionName)
	}
	return fmt.Errorf("jira-cli와 REST API 모두 사용할 수 없습니다")
}

func (f *FallbackAdapter) GetDeployDate(key string) (string, error) {
	// Deploy date requires custom fields → REST only
	if f.REST.Available() {
		return f.REST.GetDeployDate(key)
	}
	return "", fmt.Errorf("배포일자 조회는 REST API가 필요합니다 (grove config jira)")
}

func (f *FallbackAdapter) SetDeployDate(key, date string) error {
	// Deploy date requires custom fields → REST only
	if f.REST.Available() {
		return f.REST.SetDeployDate(key, date)
	}
	return fmt.Errorf("배포일자 설정은 REST API가 필요합니다 (grove config jira)")
}

func (f *FallbackAdapter) AddLabel(key, label string) error {
	// Label operations require REST API (CLI doesn't support adding labels without replacing)
	if f.REST.Available() {
		return f.REST.AddLabel(key, label)
	}
	return fmt.Errorf("라벨 추가는 REST API가 필요합니다 (grove config jira)")
}

func (f *FallbackAdapter) AddComment(key, body string) error {
	if f.CLI.Available() {
		err := f.CLI.AddComment(key, body)
		if err == nil {
			return nil
		}
		if f.REST.Available() {
			logFallback("AddComment", key, err)
			return f.REST.AddComment(key, body)
		}
		return err
	}
	if f.REST.Available() {
		return f.REST.AddComment(key, body)
	}
	return fmt.Errorf("jira-cli와 REST API 모두 사용할 수 없습니다")
}

func logFallback(op, key string, cliErr error) {
	msg := cliErr.Error()
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate") {
		fmt.Fprintf(os.Stderr, "⚠ jira-cli API 제한 (%s %s), REST API로 전환\n", op, key)
	} else {
		fmt.Fprintf(os.Stderr, "⚠ jira-cli 실패 (%s %s), REST API로 전환\n", op, key)
	}
}
