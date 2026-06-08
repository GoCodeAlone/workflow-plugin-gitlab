package internal

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

type gitLabSecretSetStep struct {
	name   string
	config gitLabSecretSetConfig
	api    gitLabSecretsEnvironmentAPI
}

type gitLabSecretSetConfig struct {
	Scope            gitLabSecretScope `yaml:"scope"`
	Project          string            `yaml:"project"`
	Group            string            `yaml:"group"`
	Key              string            `yaml:"key"`
	Value            string            `yaml:"value"`
	Description      string            `yaml:"description"`
	EnvironmentScope string            `yaml:"environment_scope"`
	Masked           bool              `yaml:"masked"`
	Protected        bool              `yaml:"protected"`
	Raw              bool              `yaml:"raw"`
	Token            string            `yaml:"token"`
	URL              string            `yaml:"url"`
}

func newGitLabSecretSetStep(name string, raw map[string]any, api gitLabSecretsEnvironmentAPI) (*gitLabSecretSetStep, error) {
	cfg, err := parseGitLabSecretSetConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("step.gitlab_secret_set %q: %w", name, err)
	}
	if api == nil {
		api, err = newSDKGitLabSecretsEnvironmentAPI(cfg.Token, cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("step.gitlab_secret_set %q: %w", name, err)
		}
	}
	return &gitLabSecretSetStep{name: name, config: cfg, api: api}, nil
}

func parseGitLabSecretSetConfig(raw map[string]any) (gitLabSecretSetConfig, error) {
	var cfg gitLabSecretSetConfig
	cfg.Scope = gitLabSecretScope(strVal(raw, "scope"))
	if cfg.Scope == "" {
		cfg.Scope = gitLabSecretScopeProject
	}
	cfg.Project = strVal(raw, "project")
	cfg.Group = strVal(raw, "group")
	cfg.Key = strVal(raw, "key")
	cfg.Value = os.ExpandEnv(strVal(raw, "value"))
	cfg.Description = strVal(raw, "description")
	cfg.EnvironmentScope = strVal(raw, "environment_scope")
	cfg.Masked = boolConfigVal(raw, "masked", true)
	cfg.Protected = boolConfigVal(raw, "protected", false)
	cfg.Raw = boolConfigVal(raw, "raw", true)
	cfg.Token = os.ExpandEnv(strVal(raw, "token"))
	cfg.URL = strVal(raw, "url")
	if err := (gitLabVariableOptions{
		Scope:   cfg.Scope,
		Project: cfg.Project,
		Group:   cfg.Group,
		Key:     cfg.Key,
	}).validateVariable(); err != nil {
		return cfg, err
	}
	if cfg.Value == "" {
		return cfg, fmt.Errorf("config.value is required")
	}
	return cfg, nil
}

func (s *gitLabSecretSetStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, _ map[string]any, _ map[string]any, _ map[string]any) (*sdk.StepResult, error) {
	status, err := s.api.SetVariable(ctx, gitLabVariableOptions{
		Scope:            s.config.Scope,
		Project:          s.config.Project,
		Group:            s.config.Group,
		Key:              s.config.Key,
		Value:            s.config.Value,
		Description:      s.config.Description,
		EnvironmentScope: s.config.EnvironmentScope,
		Masked:           s.config.Masked,
		Protected:        s.config.Protected,
		Raw:              s.config.Raw,
	})
	if err != nil {
		return gitlabErrorResult(fmt.Sprintf("failed to set GitLab variable: %v", err)), nil
	}
	return &sdk.StepResult{Output: variableStatusMap(status)}, nil
}

type gitLabSecretListStep struct {
	name   string
	config gitLabSecretListConfig
	api    gitLabSecretsEnvironmentAPI
}

type gitLabSecretListConfig struct {
	Scope            gitLabSecretScope `yaml:"scope"`
	Project          string            `yaml:"project"`
	Group            string            `yaml:"group"`
	EnvironmentScope string            `yaml:"environment_scope"`
	Token            string            `yaml:"token"`
	URL              string            `yaml:"url"`
}

func newGitLabSecretListStep(name string, raw map[string]any, api gitLabSecretsEnvironmentAPI) (*gitLabSecretListStep, error) {
	cfg, err := parseGitLabSecretListConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("step.gitlab_secret_list %q: %w", name, err)
	}
	if api == nil {
		api, err = newSDKGitLabSecretsEnvironmentAPI(cfg.Token, cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("step.gitlab_secret_list %q: %w", name, err)
		}
	}
	return &gitLabSecretListStep{name: name, config: cfg, api: api}, nil
}

func parseGitLabSecretListConfig(raw map[string]any) (gitLabSecretListConfig, error) {
	var cfg gitLabSecretListConfig
	cfg.Scope = gitLabSecretScope(strVal(raw, "scope"))
	if cfg.Scope == "" {
		cfg.Scope = gitLabSecretScopeProject
	}
	cfg.Project = strVal(raw, "project")
	cfg.Group = strVal(raw, "group")
	cfg.EnvironmentScope = strVal(raw, "environment_scope")
	cfg.Token = os.ExpandEnv(strVal(raw, "token"))
	cfg.URL = strVal(raw, "url")
	return cfg, (gitLabVariableOptions{Scope: cfg.Scope, Project: cfg.Project, Group: cfg.Group, Key: "DUMMY"}).validateVariable()
}

func (s *gitLabSecretListStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, _ map[string]any, _ map[string]any, _ map[string]any) (*sdk.StepResult, error) {
	statuses, err := s.api.ListVariables(ctx, gitLabVariableOptions{
		Scope:            s.config.Scope,
		Project:          s.config.Project,
		Group:            s.config.Group,
		EnvironmentScope: s.config.EnvironmentScope,
	})
	if err != nil {
		return gitlabErrorResult(fmt.Sprintf("failed to list GitLab variables: %v", err)), nil
	}
	items := make([]map[string]any, 0, len(statuses))
	for _, status := range statuses {
		items = append(items, variableStatusMap(status))
	}
	return &sdk.StepResult{Output: map[string]any{"variables": items, "count": len(items)}}, nil
}

type gitLabEnvironmentEnsureStep struct {
	name   string
	config gitLabEnvironmentEnsureConfig
	api    gitLabSecretsEnvironmentAPI
}

type gitLabEnvironmentEnsureConfig struct {
	Project     string `yaml:"project"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	ExternalURL string `yaml:"external_url"`
	Tier        string `yaml:"tier"`
	Token       string `yaml:"token"`
	URL         string `yaml:"url"`
}

func newGitLabEnvironmentEnsureStep(name string, raw map[string]any, api gitLabSecretsEnvironmentAPI) (*gitLabEnvironmentEnsureStep, error) {
	cfg, err := parseGitLabEnvironmentEnsureConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("step.gitlab_environment_ensure %q: %w", name, err)
	}
	if api == nil {
		api, err = newSDKGitLabSecretsEnvironmentAPI(cfg.Token, cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("step.gitlab_environment_ensure %q: %w", name, err)
		}
	}
	return &gitLabEnvironmentEnsureStep{name: name, config: cfg, api: api}, nil
}

func parseGitLabEnvironmentEnsureConfig(raw map[string]any) (gitLabEnvironmentEnsureConfig, error) {
	cfg := gitLabEnvironmentEnsureConfig{
		Project:     strVal(raw, "project"),
		Name:        strVal(raw, "name"),
		Description: strVal(raw, "description"),
		ExternalURL: strVal(raw, "external_url"),
		Tier:        strVal(raw, "tier"),
		Token:       os.ExpandEnv(strVal(raw, "token")),
		URL:         strVal(raw, "url"),
	}
	if strings.TrimSpace(cfg.Project) == "" {
		return cfg, fmt.Errorf("config.project is required")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return cfg, fmt.Errorf("config.name is required")
	}
	return cfg, nil
}

func (s *gitLabEnvironmentEnsureStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, _ map[string]any, _ map[string]any, _ map[string]any) (*sdk.StepResult, error) {
	status, created, err := s.api.EnsureEnvironment(ctx, gitLabEnvironmentOptions{
		Project:     s.config.Project,
		Name:        s.config.Name,
		Description: s.config.Description,
		ExternalURL: s.config.ExternalURL,
		Tier:        s.config.Tier,
	})
	if err != nil {
		return gitlabErrorResult(fmt.Sprintf("failed to ensure GitLab environment: %v", err)), nil
	}
	out := environmentStatusMap(status)
	out["created"] = created
	return &sdk.StepResult{Output: out}, nil
}

type gitLabEnvironmentListStep struct {
	name   string
	config gitLabEnvironmentListConfig
	api    gitLabSecretsEnvironmentAPI
}

type gitLabEnvironmentListConfig struct {
	Project string `yaml:"project"`
	Name    string `yaml:"name"`
	Token   string `yaml:"token"`
	URL     string `yaml:"url"`
}

func newGitLabEnvironmentListStep(name string, raw map[string]any, api gitLabSecretsEnvironmentAPI) (*gitLabEnvironmentListStep, error) {
	cfg, err := parseGitLabEnvironmentListConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("step.gitlab_environment_list %q: %w", name, err)
	}
	if api == nil {
		api, err = newSDKGitLabSecretsEnvironmentAPI(cfg.Token, cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("step.gitlab_environment_list %q: %w", name, err)
		}
	}
	return &gitLabEnvironmentListStep{name: name, config: cfg, api: api}, nil
}

func parseGitLabEnvironmentListConfig(raw map[string]any) (gitLabEnvironmentListConfig, error) {
	cfg := gitLabEnvironmentListConfig{
		Project: strVal(raw, "project"),
		Name:    strVal(raw, "name"),
		Token:   os.ExpandEnv(strVal(raw, "token")),
		URL:     strVal(raw, "url"),
	}
	if strings.TrimSpace(cfg.Project) == "" {
		return cfg, fmt.Errorf("config.project is required")
	}
	return cfg, nil
}

func (s *gitLabEnvironmentListStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, _ map[string]any, _ map[string]any, _ map[string]any) (*sdk.StepResult, error) {
	statuses, err := s.api.ListEnvironments(ctx, s.config.Project, s.config.Name)
	if err != nil {
		return gitlabErrorResult(fmt.Sprintf("failed to list GitLab environments: %v", err)), nil
	}
	items := make([]map[string]any, 0, len(statuses))
	for _, status := range statuses {
		items = append(items, environmentStatusMap(status))
	}
	return &sdk.StepResult{Output: map[string]any{"environments": items, "count": len(items)}}, nil
}

func variableStatusMap(status gitLabVariableStatus) map[string]any {
	return map[string]any{
		"key":               status.Key,
		"environment_scope": status.EnvironmentScope,
		"masked":            status.Masked,
		"protected":         status.Protected,
		"raw":               status.Raw,
		"variable_type":     status.VariableType,
	}
}

func environmentStatusMap(status gitLabEnvironmentStatus) map[string]any {
	return map[string]any{
		"id":           status.ID,
		"name":         status.Name,
		"slug":         status.Slug,
		"state":        status.State,
		"tier":         status.Tier,
		"external_url": status.ExternalURL,
	}
}

func boolConfigVal(raw map[string]any, key string, fallback bool) bool {
	switch value := raw[key].(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "yes", "1":
			return true
		case "false", "no", "0":
			return false
		}
	}
	return fallback
}
