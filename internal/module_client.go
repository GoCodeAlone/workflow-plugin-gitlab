package internal

import (
	"context"
	"os"
)

// gitlabClientModule implements sdk.ModuleInstance for the gitlab.client type.
// It stores GitLab API connection settings for use by step implementations.
// TODO: Expose as a registry service so steps can resolve it by module name.
type gitlabClientModule struct {
	name  string
	url   string
	token string
}

func newClientModule(name string, config map[string]any) (*gitlabClientModule, error) {
	url, _ := config["url"].(string)
	if url == "" {
		url = "https://gitlab.com"
	}
	token, _ := config["token"].(string)
	token = os.ExpandEnv(token)

	return &gitlabClientModule{name: name, url: url, token: token}, nil
}

func (m *gitlabClientModule) Init() error              { return nil }
func (m *gitlabClientModule) Start(_ context.Context) error { return nil }
func (m *gitlabClientModule) Stop(_ context.Context) error  { return nil }

// NewHTTPClient returns an HTTP GitLab client using this module's config.
func (m *gitlabClientModule) NewHTTPClient() GitLabClient {
	return newHTTPGitLabClient(m.url)
}
