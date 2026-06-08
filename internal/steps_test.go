package internal

import (
	"context"
	"testing"
)

func TestTriggerPipelineStep_Mock(t *testing.T) {
	step, err := newTriggerPipelineStep("trigger", map[string]any{
		"project": "group/project",
		"ref":     "main",
		"token":   "mock",
	}, &mockGitLabClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := step.Execute(context.Background(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StopPipeline {
		t.Fatal("expected success, got StopPipeline=true")
	}

	pipelineID, _ := result.Output["pipeline_id"].(int)
	if pipelineID == 0 {
		t.Error("expected non-zero pipeline_id")
	}

	status, _ := result.Output["status"].(string)
	if status == "" {
		t.Error("expected non-empty status")
	}
}

func TestTriggerPipelineStep_MissingProject(t *testing.T) {
	_, err := newTriggerPipelineStep("trigger", map[string]any{
		"ref":   "main",
		"token": "tok",
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestPipelineStatusStep_Mock(t *testing.T) {
	step, err := newPipelineStatusStep("status", map[string]any{
		"project":     "group/project",
		"pipeline_id": 42,
		"token":       "mock",
	}, &mockGitLabClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := step.Execute(context.Background(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StopPipeline {
		t.Fatal("expected success, got StopPipeline=true")
	}

	pipelineID, _ := result.Output["pipeline_id"].(int)
	if pipelineID != 42 {
		t.Errorf("expected pipeline_id=42, got %d", pipelineID)
	}

	status, _ := result.Output["status"].(string)
	if status != "success" {
		t.Errorf("expected status=success, got %q", status)
	}
}

func TestPipelineStatusStep_MissingPipelineID(t *testing.T) {
	_, err := newPipelineStatusStep("status", map[string]any{
		"project": "group/project",
		"token":   "tok",
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing pipeline_id")
	}
}

func TestGitLabSecretSetStep_Project(t *testing.T) {
	api := &fakeGitLabSecretsEnvAPI{}
	step, err := newGitLabSecretSetStep("set", map[string]any{
		"project":           "group/project",
		"key":               "DEPLOY_TOKEN",
		"value":             "secret",
		"environment_scope": "production",
		"masked":            true,
	}, api)
	if err != nil {
		t.Fatalf("newGitLabSecretSetStep: %v", err)
	}

	result, err := step.Execute(context.Background(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.StopPipeline {
		t.Fatal("expected successful result")
	}
	if api.set.Scope != gitLabSecretScopeProject || api.set.Project != "group/project" || api.set.EnvironmentScope != "production" {
		t.Fatalf("set opts = %+v", api.set)
	}
	if got := result.Output["key"]; got != "DEPLOY_TOKEN" {
		t.Fatalf("output key = %v", got)
	}
}

func TestGitLabSecretSetStep_Group(t *testing.T) {
	api := &fakeGitLabSecretsEnvAPI{}
	step, err := newGitLabSecretSetStep("set", map[string]any{
		"scope": "group",
		"group": "platform",
		"key":   "SHARED_TOKEN",
		"value": "secret",
	}, api)
	if err != nil {
		t.Fatalf("newGitLabSecretSetStep: %v", err)
	}

	if _, err := step.Execute(context.Background(), nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if api.set.Scope != gitLabSecretScopeGroup || api.set.Group != "platform" {
		t.Fatalf("set opts = %+v", api.set)
	}
}

func TestGitLabSecretListStep(t *testing.T) {
	api := &fakeGitLabSecretsEnvAPI{
		variables: []gitLabVariableStatus{{Key: "A"}, {Key: "B", EnvironmentScope: "production"}},
	}
	step, err := newGitLabSecretListStep("list", map[string]any{
		"project": "group/project",
	}, api)
	if err != nil {
		t.Fatalf("newGitLabSecretListStep: %v", err)
	}

	result, err := step.Execute(context.Background(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := result.Output["count"]; got != 2 {
		t.Fatalf("count = %v, want 2", got)
	}
}

func TestGitLabEnvironmentEnsureStep(t *testing.T) {
	api := &fakeGitLabSecretsEnvAPI{}
	step, err := newGitLabEnvironmentEnsureStep("ensure", map[string]any{
		"project": "group/project",
		"name":    "production",
		"tier":    "production",
	}, api)
	if err != nil {
		t.Fatalf("newGitLabEnvironmentEnsureStep: %v", err)
	}

	result, err := step.Execute(context.Background(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if api.ensure.Project != "group/project" || api.ensure.Name != "production" {
		t.Fatalf("ensure opts = %+v", api.ensure)
	}
	if got := result.Output["created"]; got != true {
		t.Fatalf("created = %v, want true", got)
	}
}

func TestGitLabEnvironmentListStep(t *testing.T) {
	api := &fakeGitLabSecretsEnvAPI{
		environments: []gitLabEnvironmentStatus{{ID: 1, Name: "production"}},
	}
	step, err := newGitLabEnvironmentListStep("list-envs", map[string]any{
		"project": "group/project",
	}, api)
	if err != nil {
		t.Fatalf("newGitLabEnvironmentListStep: %v", err)
	}

	result, err := step.Execute(context.Background(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := result.Output["count"]; got != 1 {
		t.Fatalf("count = %v, want 1", got)
	}
}

type fakeGitLabSecretsEnvAPI struct {
	set          gitLabVariableOptions
	ensure       gitLabEnvironmentOptions
	variables    []gitLabVariableStatus
	environments []gitLabEnvironmentStatus
}

func (f *fakeGitLabSecretsEnvAPI) SetVariable(_ context.Context, opts gitLabVariableOptions) (gitLabVariableStatus, error) {
	f.set = opts
	return gitLabVariableStatus{Key: opts.Key, EnvironmentScope: opts.EnvironmentScope, Masked: opts.Masked, Protected: opts.Protected, Raw: opts.Raw, VariableType: "env_var"}, nil
}

func (f *fakeGitLabSecretsEnvAPI) ListVariables(context.Context, gitLabVariableOptions) ([]gitLabVariableStatus, error) {
	return f.variables, nil
}

func (f *fakeGitLabSecretsEnvAPI) EnsureEnvironment(_ context.Context, opts gitLabEnvironmentOptions) (gitLabEnvironmentStatus, bool, error) {
	f.ensure = opts
	return gitLabEnvironmentStatus{ID: 42, Name: opts.Name, Slug: opts.Name, Tier: opts.Tier}, true, nil
}

func (f *fakeGitLabSecretsEnvAPI) ListEnvironments(context.Context, string, string) ([]gitLabEnvironmentStatus, error) {
	return f.environments, nil
}

func TestCreateMRStep_Mock(t *testing.T) {
	step, err := newCreateMRStep("create-mr", map[string]any{
		"project":       "group/project",
		"source_branch": "feature-x",
		"target_branch": "main",
		"title":         "My Feature",
		"token":         "mock",
	}, &mockGitLabClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := step.Execute(context.Background(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StopPipeline {
		t.Fatal("expected success, got StopPipeline=true")
	}

	title, _ := result.Output["title"].(string)
	if title != "My Feature" {
		t.Errorf("expected title=My Feature, got %q", title)
	}

	state, _ := result.Output["state"].(string)
	if state != "opened" {
		t.Errorf("expected state=opened, got %q", state)
	}
}

func TestCreateMRStep_MissingSourceBranch(t *testing.T) {
	_, err := newCreateMRStep("create-mr", map[string]any{
		"project": "group/project",
		"token":   "tok",
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing source_branch")
	}
}

func TestMRCommentStep_Mock(t *testing.T) {
	step, err := newMRCommentStep("comment", map[string]any{
		"project": "group/project",
		"mr_iid":  1,
		"body":    "LGTM!",
		"token":   "mock",
	}, &mockGitLabClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := step.Execute(context.Background(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StopPipeline {
		t.Fatal("expected success, got StopPipeline=true")
	}

	commented, _ := result.Output["commented"].(bool)
	if !commented {
		t.Error("expected commented=true")
	}
}

func TestMRCommentStep_MissingBody(t *testing.T) {
	_, err := newMRCommentStep("comment", map[string]any{
		"project": "group/project",
		"mr_iid":  1,
		"token":   "tok",
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing body")
	}
}

func TestPipelineIDStringParsing(t *testing.T) {
	step, err := newPipelineStatusStep("status", map[string]any{
		"project":     "group/project",
		"pipeline_id": "99",
		"token":       "mock",
	}, &mockGitLabClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if step.config.PipelineID != 99 {
		t.Errorf("expected PipelineID=99, got %d", step.config.PipelineID)
	}
}
