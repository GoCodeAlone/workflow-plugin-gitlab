package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

type gitLabSecretScope string

const (
	gitLabSecretScopeProject gitLabSecretScope = "project"
	gitLabSecretScopeGroup   gitLabSecretScope = "group"
)

type gitLabVariableOptions struct {
	Scope            gitLabSecretScope
	Project          string
	Group            string
	Key              string
	Value            string
	Description      string
	EnvironmentScope string
	Masked           bool
	Protected        bool
	Raw              bool
}

type gitLabEnvironmentOptions struct {
	Project     string
	Name        string
	Description string
	ExternalURL string
	Tier        string
}

type gitLabVariableStatus struct {
	Key              string
	EnvironmentScope string
	Masked           bool
	Protected        bool
	Raw              bool
	VariableType     string
}

type gitLabEnvironmentStatus struct {
	ID          int64
	Name        string
	Slug        string
	State       string
	Tier        string
	ExternalURL string
}

type gitLabSecretsEnvironmentAPI interface {
	SetVariable(ctx context.Context, opts gitLabVariableOptions) (gitLabVariableStatus, error)
	ListVariables(ctx context.Context, opts gitLabVariableOptions) ([]gitLabVariableStatus, error)
	EnsureEnvironment(ctx context.Context, opts gitLabEnvironmentOptions) (gitLabEnvironmentStatus, bool, error)
	ListEnvironments(ctx context.Context, project, name string) ([]gitLabEnvironmentStatus, error)
}

type sdkGitLabSecretsEnvironmentAPI struct {
	client *gitlab.Client
}

func newSDKGitLabSecretsEnvironmentAPI(token, baseURL string) (gitLabSecretsEnvironmentAPI, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("gitlab token is required")
	}
	opts := []gitlab.ClientOptionFunc{}
	if strings.TrimSpace(baseURL) != "" {
		opts = append(opts, gitlab.WithBaseURL(baseURL))
	}
	client, err := gitlab.NewClient(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}
	return &sdkGitLabSecretsEnvironmentAPI{client: client}, nil
}

func (a *sdkGitLabSecretsEnvironmentAPI) SetVariable(ctx context.Context, opts gitLabVariableOptions) (gitLabVariableStatus, error) {
	if err := opts.validateVariable(); err != nil {
		return gitLabVariableStatus{}, err
	}
	switch opts.Scope {
	case gitLabSecretScopeProject:
		return a.setProjectVariable(ctx, opts)
	case gitLabSecretScopeGroup:
		return a.setGroupVariable(ctx, opts)
	default:
		return gitLabVariableStatus{}, fmt.Errorf("unsupported gitlab variable scope %q", opts.Scope)
	}
}

func (a *sdkGitLabSecretsEnvironmentAPI) setProjectVariable(ctx context.Context, opts gitLabVariableOptions) (gitLabVariableStatus, error) {
	filter := variableFilter(opts.EnvironmentScope)
	_, resp, getErr := a.client.ProjectVariables.GetVariable(opts.Project, opts.Key, &gitlab.GetProjectVariableOptions{Filter: filter}, gitlab.WithContext(ctx))
	if getErr == nil {
		updated, _, err := a.client.ProjectVariables.UpdateVariable(opts.Project, opts.Key, projectVariableUpdateOptions(opts, filter), gitlab.WithContext(ctx))
		if err != nil {
			return gitLabVariableStatus{}, fmt.Errorf("update gitlab project variable %s: %w", opts.Key, err)
		}
		return projectVariableStatus(updated), nil
	}
	if !gitLabNotFound(resp) {
		return gitLabVariableStatus{}, fmt.Errorf("get gitlab project variable %s: %w", opts.Key, getErr)
	}
	created, _, err := a.client.ProjectVariables.CreateVariable(opts.Project, projectVariableCreateOptions(opts), gitlab.WithContext(ctx))
	if err != nil {
		return gitLabVariableStatus{}, fmt.Errorf("create gitlab project variable %s: %w", opts.Key, err)
	}
	return projectVariableStatus(created), nil
}

func (a *sdkGitLabSecretsEnvironmentAPI) setGroupVariable(ctx context.Context, opts gitLabVariableOptions) (gitLabVariableStatus, error) {
	filter := variableFilter(opts.EnvironmentScope)
	_, resp, getErr := a.client.GroupVariables.GetVariable(opts.Group, opts.Key, &gitlab.GetGroupVariableOptions{Filter: filter}, gitlab.WithContext(ctx))
	if getErr == nil {
		updated, _, err := a.client.GroupVariables.UpdateVariable(opts.Group, opts.Key, groupVariableUpdateOptions(opts, filter), gitlab.WithContext(ctx))
		if err != nil {
			return gitLabVariableStatus{}, fmt.Errorf("update gitlab group variable %s: %w", opts.Key, err)
		}
		return groupVariableStatus(updated), nil
	}
	if !gitLabNotFound(resp) {
		return gitLabVariableStatus{}, fmt.Errorf("get gitlab group variable %s: %w", opts.Key, getErr)
	}
	created, _, err := a.client.GroupVariables.CreateVariable(opts.Group, groupVariableCreateOptions(opts), gitlab.WithContext(ctx))
	if err != nil {
		return gitLabVariableStatus{}, fmt.Errorf("create gitlab group variable %s: %w", opts.Key, err)
	}
	return groupVariableStatus(created), nil
}

func (a *sdkGitLabSecretsEnvironmentAPI) ListVariables(ctx context.Context, opts gitLabVariableOptions) ([]gitLabVariableStatus, error) {
	if opts.Scope == "" {
		opts.Scope = gitLabSecretScopeProject
	}
	switch opts.Scope {
	case gitLabSecretScopeProject:
		if strings.TrimSpace(opts.Project) == "" {
			return nil, errors.New("project is required for gitlab project variables")
		}
		listOpts := &gitlab.ListProjectVariablesOptions{
			ListOptions: gitlab.ListOptions{PerPage: 100},
		}
		var out []gitLabVariableStatus
		for {
			vars, resp, err := a.client.ProjectVariables.ListVariables(opts.Project, listOpts, gitlab.WithContext(ctx))
			if err != nil {
				return nil, fmt.Errorf("list gitlab project variables: %w", err)
			}
			for _, variable := range vars {
				status := projectVariableStatus(variable)
				if variableEnvironmentScopeMatches(status, opts.EnvironmentScope) {
					out = append(out, status)
				}
			}
			if resp == nil || resp.NextPage == 0 {
				return out, nil
			}
			listOpts.Page = resp.NextPage
		}
	case gitLabSecretScopeGroup:
		if strings.TrimSpace(opts.Group) == "" {
			return nil, errors.New("group is required for gitlab group variables")
		}
		listOpts := &gitlab.ListGroupVariablesOptions{
			ListOptions: gitlab.ListOptions{PerPage: 100},
		}
		var out []gitLabVariableStatus
		for {
			vars, resp, err := a.client.GroupVariables.ListVariables(opts.Group, listOpts, gitlab.WithContext(ctx))
			if err != nil {
				return nil, fmt.Errorf("list gitlab group variables: %w", err)
			}
			for _, variable := range vars {
				status := groupVariableStatus(variable)
				if variableEnvironmentScopeMatches(status, opts.EnvironmentScope) {
					out = append(out, status)
				}
			}
			if resp == nil || resp.NextPage == 0 {
				return out, nil
			}
			listOpts.Page = resp.NextPage
		}
	default:
		return nil, fmt.Errorf("unsupported gitlab variable scope %q", opts.Scope)
	}
}

func (a *sdkGitLabSecretsEnvironmentAPI) EnsureEnvironment(ctx context.Context, opts gitLabEnvironmentOptions) (gitLabEnvironmentStatus, bool, error) {
	if strings.TrimSpace(opts.Project) == "" {
		return gitLabEnvironmentStatus{}, false, errors.New("project is required for gitlab environments")
	}
	if strings.TrimSpace(opts.Name) == "" {
		return gitLabEnvironmentStatus{}, false, errors.New("environment name is required")
	}
	existing, err := a.ListEnvironments(ctx, opts.Project, opts.Name)
	if err != nil {
		return gitLabEnvironmentStatus{}, false, err
	}
	for _, env := range existing {
		if env.Name == opts.Name {
			return env, false, nil
		}
	}
	created, _, err := a.client.Environments.CreateEnvironment(opts.Project, &gitlab.CreateEnvironmentOptions{
		Name:        gitlab.Ptr(opts.Name),
		Description: optionalString(opts.Description),
		ExternalURL: optionalString(opts.ExternalURL),
		Tier:        optionalString(opts.Tier),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return gitLabEnvironmentStatus{}, false, fmt.Errorf("create gitlab environment %q: %w", opts.Name, err)
	}
	return environmentStatus(created), true, nil
}

func (a *sdkGitLabSecretsEnvironmentAPI) ListEnvironments(ctx context.Context, project, name string) ([]gitLabEnvironmentStatus, error) {
	if strings.TrimSpace(project) == "" {
		return nil, errors.New("project is required for gitlab environments")
	}
	listOpts := &gitlab.ListEnvironmentsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	if strings.TrimSpace(name) != "" {
		listOpts.Name = gitlab.Ptr(name)
	}
	var out []gitLabEnvironmentStatus
	for {
		envs, resp, err := a.client.Environments.ListEnvironments(project, listOpts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("list gitlab environments: %w", err)
		}
		for _, env := range envs {
			out = append(out, environmentStatus(env))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, nil
		}
		listOpts.Page = resp.NextPage
	}
}

func (opts gitLabVariableOptions) validateVariable() error {
	if strings.TrimSpace(opts.Key) == "" {
		return errors.New("variable key is required")
	}
	switch opts.Scope {
	case "", gitLabSecretScopeProject:
		if strings.TrimSpace(opts.Project) == "" {
			return errors.New("project is required for gitlab project variables")
		}
	case gitLabSecretScopeGroup:
		if strings.TrimSpace(opts.Group) == "" {
			return errors.New("group is required for gitlab group variables")
		}
	default:
		return fmt.Errorf("unsupported gitlab variable scope %q", opts.Scope)
	}
	return nil
}

func variableFilter(environmentScope string) *gitlab.VariableFilter {
	if strings.TrimSpace(environmentScope) == "" {
		return nil
	}
	return &gitlab.VariableFilter{EnvironmentScope: environmentScope}
}

func projectVariableCreateOptions(opts gitLabVariableOptions) *gitlab.CreateProjectVariableOptions {
	return &gitlab.CreateProjectVariableOptions{
		Key:              gitlab.Ptr(opts.Key),
		Value:            gitlab.Ptr(opts.Value),
		Description:      optionalString(opts.Description),
		EnvironmentScope: optionalString(opts.EnvironmentScope),
		Masked:           gitlab.Ptr(opts.Masked),
		Protected:        gitlab.Ptr(opts.Protected),
		Raw:              gitlab.Ptr(opts.Raw),
		VariableType:     gitlab.Ptr(gitlab.EnvVariableType),
	}
}

func projectVariableUpdateOptions(opts gitLabVariableOptions, filter *gitlab.VariableFilter) *gitlab.UpdateProjectVariableOptions {
	return &gitlab.UpdateProjectVariableOptions{
		Value:            gitlab.Ptr(opts.Value),
		Description:      optionalString(opts.Description),
		EnvironmentScope: optionalString(opts.EnvironmentScope),
		Filter:           filter,
		Masked:           gitlab.Ptr(opts.Masked),
		Protected:        gitlab.Ptr(opts.Protected),
		Raw:              gitlab.Ptr(opts.Raw),
		VariableType:     gitlab.Ptr(gitlab.EnvVariableType),
	}
}

func groupVariableCreateOptions(opts gitLabVariableOptions) *gitlab.CreateGroupVariableOptions {
	return &gitlab.CreateGroupVariableOptions{
		Key:              gitlab.Ptr(opts.Key),
		Value:            gitlab.Ptr(opts.Value),
		Description:      optionalString(opts.Description),
		EnvironmentScope: optionalString(opts.EnvironmentScope),
		Masked:           gitlab.Ptr(opts.Masked),
		Protected:        gitlab.Ptr(opts.Protected),
		Raw:              gitlab.Ptr(opts.Raw),
		VariableType:     gitlab.Ptr(gitlab.EnvVariableType),
	}
}

func groupVariableUpdateOptions(opts gitLabVariableOptions, filter *gitlab.VariableFilter) *gitlab.UpdateGroupVariableOptions {
	return &gitlab.UpdateGroupVariableOptions{
		Value:            gitlab.Ptr(opts.Value),
		Description:      optionalString(opts.Description),
		EnvironmentScope: optionalString(opts.EnvironmentScope),
		Filter:           filter,
		Masked:           gitlab.Ptr(opts.Masked),
		Protected:        gitlab.Ptr(opts.Protected),
		Raw:              gitlab.Ptr(opts.Raw),
		VariableType:     gitlab.Ptr(gitlab.EnvVariableType),
	}
}

func projectVariableStatus(variable *gitlab.ProjectVariable) gitLabVariableStatus {
	if variable == nil {
		return gitLabVariableStatus{}
	}
	return gitLabVariableStatus{
		Key:              variable.Key,
		EnvironmentScope: variable.EnvironmentScope,
		Masked:           variable.Masked,
		Protected:        variable.Protected,
		Raw:              variable.Raw,
		VariableType:     string(variable.VariableType),
	}
}

func groupVariableStatus(variable *gitlab.GroupVariable) gitLabVariableStatus {
	if variable == nil {
		return gitLabVariableStatus{}
	}
	return gitLabVariableStatus{
		Key:              variable.Key,
		EnvironmentScope: variable.EnvironmentScope,
		Masked:           variable.Masked,
		Protected:        variable.Protected,
		Raw:              variable.Raw,
		VariableType:     string(variable.VariableType),
	}
}

func environmentStatus(env *gitlab.Environment) gitLabEnvironmentStatus {
	if env == nil {
		return gitLabEnvironmentStatus{}
	}
	return gitLabEnvironmentStatus{
		ID:          env.ID,
		Name:        env.Name,
		Slug:        env.Slug,
		State:       env.State,
		Tier:        env.Tier,
		ExternalURL: env.ExternalURL,
	}
}

func gitLabNotFound(resp *gitlab.Response) bool {
	return resp != nil && resp.Response != nil && resp.StatusCode == http.StatusNotFound
}

func variableEnvironmentScopeMatches(status gitLabVariableStatus, want string) bool {
	want = strings.TrimSpace(want)
	return want == "" || status.EnvironmentScope == want
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return gitlab.Ptr(value)
}
