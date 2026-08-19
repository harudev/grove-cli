package jira

import "encoding/json"

// deployDateFromBody extracts the deploy date from a raw Jira issue JSON body,
// reading the configured custom field id dynamically (Go struct tags can't be
// parameterized, so the field is plucked from a map). Returns "" when the field
// id is unset, absent, or null.
func deployDateFromBody(body []byte, fieldID string) string {
	if fieldID == "" {
		return ""
	}
	var wrap struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return ""
	}
	return deployDateFromFields(wrap.Fields, fieldID)
}

// deployDateFromFields extracts the deploy date from an already-decoded fields
// map. Returns "" when the field id is unset, absent, or null.
func deployDateFromFields(fields map[string]json.RawMessage, fieldID string) string {
	if fieldID == "" || fields == nil {
		return ""
	}
	raw, ok := fields[fieldID]
	if !ok {
		return ""
	}
	var s *string
	if err := json.Unmarshal(raw, &s); err != nil || s == nil {
		return ""
	}
	return *s
}
