package tui

// InteractivePrompter is the real TUI implementation of Prompter.
type InteractivePrompter struct{}

func NewInteractivePrompter() *InteractivePrompter {
	return &InteractivePrompter{}
}

func (p *InteractivePrompter) SelectBranchType(types []BranchTypeInfo) (BranchType, error) {
	return RunBranchTypeSelector(types)
}

func (p *InteractivePrompter) SelectRef(title string, refs []string) (string, error) {
	return RunRefSelector(title, refs)
}

func (p *InteractivePrompter) SelectWorktree(branches []string) (string, error) {
	return RunWorktreeSelector(branches)
}

func (p *InteractivePrompter) Confirm(message string) (bool, error) {
	return RunConfirm(message)
}

func (p *InteractivePrompter) SelectIssueContextMode(issueKey string) (IssueContextMode, error) {
	return RunIssueContextModeSelector(issueKey)
}

func (p *InteractivePrompter) SelectTDDMode() (TDDMode, error) {
	return RunTDDModeSelector()
}
