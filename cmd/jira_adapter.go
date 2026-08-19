package cmd

import (
	"github.com/harudev/grove-cli/internal/config"
	"github.com/harudev/grove-cli/internal/jira"
)

// newJiraAdapter creates a Jira adapter with CLI primary + REST API fallback.
func newJiraAdapter() jira.Adapter {
	config.ExportJiraToken()

	deployField := config.GetJiraWorkflow().DeployField
	cli := jira.NewCLIAdapter(deployField)
	server, login, token := config.GetJiraConfig()
	rest := jira.NewRestAdapter(server, login, token, deployField)

	if cli.Available() || rest.Available() {
		return jira.NewFallbackAdapter(cli, rest)
	}
	return nil
}
