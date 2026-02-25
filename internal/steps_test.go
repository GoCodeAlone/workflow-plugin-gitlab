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

	result, err := step.Execute(context.Background(), nil, nil, nil, nil)
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

	result, err := step.Execute(context.Background(), nil, nil, nil, nil)
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

	result, err := step.Execute(context.Background(), nil, nil, nil, nil)
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

	result, err := step.Execute(context.Background(), nil, nil, nil, nil)
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
