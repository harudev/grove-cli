package tui

// BranchType represents a branch type selection.
type BranchType string

// BranchTypeInfo holds display info for a branch type (built from the repo's policy).
type BranchTypeInfo struct {
	Type        BranchType
	Description string
}

// Prompter defines the interface for interactive prompts.
type Prompter interface {
	SelectBranchType(types []BranchTypeInfo) (BranchType, error)
	SelectRef(title string, refs []string) (string, error)
	SelectWorktree(branches []string) (string, error)
	Confirm(message string) (bool, error)
	SelectIssueContextMode(issueKey string) (IssueContextMode, error)
	SelectTDDMode() (TDDMode, error)
}

// MockPrompter provides predetermined answers for testing.
type MockPrompter struct {
	BranchTypeResult       BranchType
	RefResult              string
	WorktreeResult         string
	ConfirmResult          bool
	IssueContextModeResult IssueContextMode
	TDDModeResult          TDDMode
	Err                    error
}

func (m *MockPrompter) SelectBranchType(types []BranchTypeInfo) (BranchType, error) {
	return m.BranchTypeResult, m.Err
}

func (m *MockPrompter) SelectRef(title string, refs []string) (string, error) {
	return m.RefResult, m.Err
}

func (m *MockPrompter) SelectWorktree(branches []string) (string, error) {
	return m.WorktreeResult, m.Err
}

func (m *MockPrompter) Confirm(message string) (bool, error) {
	return m.ConfirmResult, m.Err
}

func (m *MockPrompter) SelectIssueContextMode(issueKey string) (IssueContextMode, error) {
	return m.IssueContextModeResult, m.Err
}

func (m *MockPrompter) SelectTDDMode() (TDDMode, error) {
	return m.TDDModeResult, m.Err
}
