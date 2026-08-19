package issue

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/harudev/grove-cli/internal/config"
)

// InputType represents the type of user input.
type InputType int

const (
	InputIssueKey InputType = iota
	InputNumeric
	InputJiraURL
	InputPRURL
)

// IssueKey represents a normalized Jira issue key.
type IssueKey struct {
	Prefix string // e.g. "PROJ", "PROJ"
	Number string // e.g. "12345"
}

func (k IssueKey) String() string {
	return fmt.Sprintf("%s-%s", k.Prefix, k.Number)
}

// ParsedInput holds the result of parsing user input.
type ParsedInput struct {
	Type     InputType
	IssueKey *IssueKey // nil for PR URL until resolved
	PRURL    string    // non-empty only for PR URLs
	Raw      string
}

var (
	numericRe = regexp.MustCompile(`^\d+$`)
	prURLRe   = regexp.MustCompile(`github\.com/.+/pull/\d+`)
	// Generic issue key pattern: UPPERCASE-digits (for when no prefix is configured)
	genericIssueKeyRe = regexp.MustCompile(`([A-Z][A-Z0-9]+)-(\d+)`)
)

// Parse parses user input into a ParsedInput.
// Full issue key (e.g. PROJ-123) → use as-is.
// Numeric only (e.g. 123) → prepend configured default prefix.
func Parse(input string) (ParsedInput, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return ParsedInput{}, fmt.Errorf("empty input")
	}

	issueKeyRe := config.IssueKeyRegex()

	// Check PR URL first (github.com/.../pull/N)
	if prURLRe.MatchString(input) {
		// Try configured prefix first, then generic
		if m := issueKeyRe.FindStringSubmatch(input); m != nil {
			return ParsedInput{
				Type:     InputPRURL,
				IssueKey: &IssueKey{Prefix: strings.ToUpper(m[1]), Number: m[2]},
				PRURL:    input,
				Raw:      input,
			}, nil
		}
		if m := genericIssueKeyRe.FindStringSubmatch(input); m != nil {
			return ParsedInput{
				Type:     InputPRURL,
				IssueKey: &IssueKey{Prefix: strings.ToUpper(m[1]), Number: m[2]},
				PRURL:    input,
				Raw:      input,
			}, nil
		}
		return ParsedInput{
			Type:  InputPRURL,
			PRURL: input,
			Raw:   input,
		}, nil
	}

	// Try configured prefix regex
	if m := issueKeyRe.FindStringSubmatch(input); m != nil {
		key := &IssueKey{Prefix: strings.ToUpper(m[1]), Number: m[2]}
		inputType := InputIssueKey
		if strings.Contains(input, "://") || strings.Contains(input, ".") {
			inputType = InputJiraURL
		}
		return ParsedInput{Type: inputType, IssueKey: key, Raw: input}, nil
	}

	// Try generic UPPERCASE-digits pattern (for full issue keys with any prefix)
	if m := genericIssueKeyRe.FindStringSubmatch(input); m != nil {
		key := &IssueKey{Prefix: strings.ToUpper(m[1]), Number: m[2]}
		inputType := InputIssueKey
		if strings.Contains(input, "://") || strings.Contains(input, ".") {
			inputType = InputJiraURL
		}
		return ParsedInput{Type: inputType, IssueKey: key, Raw: input}, nil
	}

	// Pure numeric → use default prefix
	if numericRe.MatchString(input) {
		prefix := config.GetIssuePrefix()
		if prefix == "" {
			return ParsedInput{}, fmt.Errorf("숫자만 입력했지만 기본 이슈 prefix가 설정되지 않았습니다.\ngrove setup을 실행하여 기본 prefix를 설정해주세요")
		}
		return ParsedInput{
			Type:     InputNumeric,
			IssueKey: &IssueKey{Prefix: prefix, Number: input},
			Raw:      input,
		}, nil
	}

	return ParsedInput{}, fmt.Errorf("이슈 번호를 추출할 수 없습니다: %s", input)
}
