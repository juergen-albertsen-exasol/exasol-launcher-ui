package launcher

import "context"

// ScriptLanguage is one entry of `exasol slc list --json`: an official
// container of the catalog, or an installed custom container.
type ScriptLanguage struct {
	Type      string   `json:"type"`
	Language  string   `json:"language"`
	Flavor    string   `json:"flavor"`
	Version   string   `json:"version"`
	Aliases   []string `json:"aliases"`
	Alias     string   `json:"alias"`
	Source    string   `json:"source"`
	Installed bool     `json:"installed"`
	Available *bool    `json:"available"`
}

// ScriptLanguages reports the script language containers of the deployment
// in deploymentDir.
func (c *Client) ScriptLanguages(
	ctx context.Context,
	deploymentDir string,
) ([]ScriptLanguage, error) {
	var languages []ScriptLanguage

	err := c.runJSON(ctx, &languages,
		"slc", "list", "--json", "--deployment-dir", deploymentDir)

	return languages, err
}
