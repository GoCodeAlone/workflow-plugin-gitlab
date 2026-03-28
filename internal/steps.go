package internal

import (
	"context"
	"fmt"
	"os"
	"strconv"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// ---- step.gitlab_trigger_pipeline ----

// triggerPipelineStep triggers a GitLab CI pipeline via the API.
//
//	config:
//	  project:   "group/project"
//	  ref:       "main"
//	  token:     "${GITLAB_TOKEN}"
//	  url:       "https://gitlab.com"   # optional, default: https://gitlab.com
//	  variables:
//	    KEY: value
type triggerPipelineStep struct {
	name   string
	config triggerPipelineConfig
	client GitLabClient
}

type triggerPipelineConfig struct {
	Project   string            `yaml:"project"`
	Ref       string            `yaml:"ref"`
	Token     string            `yaml:"token"`
	URL       string            `yaml:"url"`
	Variables map[string]string `yaml:"variables"`
}

func newTriggerPipelineStep(name string, config map[string]any, client GitLabClient) (*triggerPipelineStep, error) {
	cfg, err := parseTriggerPipelineConfig(config)
	if err != nil {
		return nil, fmt.Errorf("step.gitlab_trigger_pipeline %q: %w", name, err)
	}
	if client == nil {
		if cfg.Token == "mock" {
			client = &mockGitLabClient{}
		} else {
			client = newHTTPGitLabClient(cfg.URL)
		}
	}
	return &triggerPipelineStep{name: name, config: cfg, client: client}, nil
}

func parseTriggerPipelineConfig(raw map[string]any) (triggerPipelineConfig, error) {
	var cfg triggerPipelineConfig

	cfg.Project, _ = raw["project"].(string)
	if cfg.Project == "" {
		return cfg, fmt.Errorf("config.project is required")
	}

	cfg.Ref, _ = raw["ref"].(string)
	if cfg.Ref == "" {
		cfg.Ref = "main"
	}

	cfg.Token, _ = raw["token"].(string)
	cfg.Token = os.ExpandEnv(cfg.Token)

	cfg.URL, _ = raw["url"].(string)

	if vars, ok := raw["variables"].(map[string]any); ok {
		cfg.Variables = make(map[string]string, len(vars))
		for k, v := range vars {
			cfg.Variables[k] = fmt.Sprintf("%v", v)
		}
	}

	return cfg, nil
}

func (s *triggerPipelineStep) Execute(
	ctx context.Context,
	_ map[string]any,
	_ map[string]map[string]any,
	_ map[string]any,
	_ map[string]any,
	_ map[string]any,
) (*sdk.StepResult, error) {
	pipeline, err := s.client.TriggerPipeline(ctx, s.config.Project, s.config.Ref, s.config.Variables, s.config.Token)
	if err != nil {
		return gitlabErrorResult(fmt.Sprintf("failed to trigger pipeline: %v", err)), nil
	}

	return &sdk.StepResult{
		Output: map[string]any{
			"pipeline_id": pipeline.ID,
			"status":      pipeline.Status,
			"ref":         pipeline.Ref,
			"sha":         pipeline.SHA,
			"web_url":     pipeline.WebURL,
			"created_at":  pipeline.CreatedAt,
		},
	}, nil
}

// ---- step.gitlab_pipeline_status ----

// pipelineStatusStep checks the status of a GitLab CI pipeline.
//
//	config:
//	  project:     "group/project"
//	  pipeline_id: 42
//	  token:       "${GITLAB_TOKEN}"
//	  url:         "https://gitlab.com"
type pipelineStatusStep struct {
	name   string
	config pipelineStatusConfig
	client GitLabClient
}

type pipelineStatusConfig struct {
	Project    string `yaml:"project"`
	PipelineID int    `yaml:"pipeline_id"`
	Token      string `yaml:"token"`
	URL        string `yaml:"url"`
}

func newPipelineStatusStep(name string, config map[string]any, client GitLabClient) (*pipelineStatusStep, error) {
	cfg, err := parsePipelineStatusConfig(config)
	if err != nil {
		return nil, fmt.Errorf("step.gitlab_pipeline_status %q: %w", name, err)
	}
	if client == nil {
		if cfg.Token == "mock" {
			client = &mockGitLabClient{}
		} else {
			client = newHTTPGitLabClient(cfg.URL)
		}
	}
	return &pipelineStatusStep{name: name, config: cfg, client: client}, nil
}

func parsePipelineStatusConfig(raw map[string]any) (pipelineStatusConfig, error) {
	var cfg pipelineStatusConfig

	cfg.Project, _ = raw["project"].(string)
	if cfg.Project == "" {
		return cfg, fmt.Errorf("config.project is required")
	}

	switch v := raw["pipeline_id"].(type) {
	case int:
		cfg.PipelineID = v
	case float64:
		cfg.PipelineID = int(v)
	case string:
		if v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return cfg, fmt.Errorf("config.pipeline_id is not a valid integer: %w", err)
			}
			cfg.PipelineID = n
		}
	}
	if cfg.PipelineID == 0 {
		return cfg, fmt.Errorf("config.pipeline_id is required")
	}

	cfg.Token, _ = raw["token"].(string)
	cfg.Token = os.ExpandEnv(cfg.Token)

	cfg.URL, _ = raw["url"].(string)

	return cfg, nil
}

func (s *pipelineStatusStep) Execute(
	ctx context.Context,
	_ map[string]any,
	_ map[string]map[string]any,
	_ map[string]any,
	_ map[string]any,
	_ map[string]any,
) (*sdk.StepResult, error) {
	pipeline, err := s.client.GetPipeline(ctx, s.config.Project, s.config.PipelineID, s.config.Token)
	if err != nil {
		return gitlabErrorResult(fmt.Sprintf("failed to get pipeline status: %v", err)), nil
	}

	return &sdk.StepResult{
		Output: map[string]any{
			"pipeline_id": pipeline.ID,
			"status":      pipeline.Status,
			"ref":         pipeline.Ref,
			"sha":         pipeline.SHA,
			"web_url":     pipeline.WebURL,
		},
	}, nil
}

// ---- step.gitlab_create_merge_request ----

// createMRStep creates a GitLab merge request.
//
//	config:
//	  project:       "group/project"
//	  source_branch: "feature-x"
//	  target_branch: "main"
//	  title:         "My Feature"
//	  description:   "Optional"
//	  token:         "${GITLAB_TOKEN}"
//	  url:           "https://gitlab.com"
type createMRStep struct {
	name   string
	config createMRConfig
	client GitLabClient
}

type createMRConfig struct {
	Project      string `yaml:"project"`
	SourceBranch string `yaml:"source_branch"`
	TargetBranch string `yaml:"target_branch"`
	Title        string `yaml:"title"`
	Description  string `yaml:"description"`
	Token        string `yaml:"token"`
	URL          string `yaml:"url"`
}

func newCreateMRStep(name string, config map[string]any, client GitLabClient) (*createMRStep, error) {
	cfg, err := parseCreateMRConfig(config)
	if err != nil {
		return nil, fmt.Errorf("step.gitlab_create_merge_request %q: %w", name, err)
	}
	if client == nil {
		if cfg.Token == "mock" {
			client = &mockGitLabClient{}
		} else {
			client = newHTTPGitLabClient(cfg.URL)
		}
	}
	return &createMRStep{name: name, config: cfg, client: client}, nil
}

func parseCreateMRConfig(raw map[string]any) (createMRConfig, error) {
	var cfg createMRConfig

	cfg.Project, _ = raw["project"].(string)
	if cfg.Project == "" {
		return cfg, fmt.Errorf("config.project is required")
	}

	cfg.SourceBranch, _ = raw["source_branch"].(string)
	if cfg.SourceBranch == "" {
		return cfg, fmt.Errorf("config.source_branch is required")
	}

	cfg.TargetBranch, _ = raw["target_branch"].(string)
	if cfg.TargetBranch == "" {
		cfg.TargetBranch = "main"
	}

	cfg.Title, _ = raw["title"].(string)
	if cfg.Title == "" {
		cfg.Title = cfg.SourceBranch
	}

	cfg.Description, _ = raw["description"].(string)

	cfg.Token, _ = raw["token"].(string)
	cfg.Token = os.ExpandEnv(cfg.Token)

	cfg.URL, _ = raw["url"].(string)

	return cfg, nil
}

func (s *createMRStep) Execute(
	ctx context.Context,
	_ map[string]any,
	_ map[string]map[string]any,
	_ map[string]any,
	_ map[string]any,
	_ map[string]any,
) (*sdk.StepResult, error) {
	mr, err := s.client.CreateMergeRequest(ctx, s.config.Project, MergeRequestOptions{
		SourceBranch: s.config.SourceBranch,
		TargetBranch: s.config.TargetBranch,
		Title:        s.config.Title,
		Description:  s.config.Description,
	}, s.config.Token)
	if err != nil {
		return gitlabErrorResult(fmt.Sprintf("failed to create MR: %v", err)), nil
	}

	return &sdk.StepResult{
		Output: map[string]any{
			"mr_id":         mr.ID,
			"mr_iid":        mr.IID,
			"title":         mr.Title,
			"state":         mr.State,
			"source_branch": mr.SourceBranch,
			"target_branch": mr.TargetBranch,
			"web_url":       mr.WebURL,
		},
	}, nil
}

// ---- step.gitlab_mr_comment ----

// mrCommentStep posts a comment on a GitLab merge request.
//
//	config:
//	  project: "group/project"
//	  mr_iid:  42
//	  body:    "Pipeline passed!"
//	  token:   "${GITLAB_TOKEN}"
//	  url:     "https://gitlab.com"
type mrCommentStep struct {
	name   string
	config mrCommentConfig
	client GitLabClient
}

type mrCommentConfig struct {
	Project string `yaml:"project"`
	MrIID   int    `yaml:"mr_iid"`
	Body    string `yaml:"body"`
	Token   string `yaml:"token"`
	URL     string `yaml:"url"`
}

func newMRCommentStep(name string, config map[string]any, client GitLabClient) (*mrCommentStep, error) {
	cfg, err := parseMRCommentConfig(config)
	if err != nil {
		return nil, fmt.Errorf("step.gitlab_mr_comment %q: %w", name, err)
	}
	if client == nil {
		if cfg.Token == "mock" {
			client = &mockGitLabClient{}
		} else {
			client = newHTTPGitLabClient(cfg.URL)
		}
	}
	return &mrCommentStep{name: name, config: cfg, client: client}, nil
}

func parseMRCommentConfig(raw map[string]any) (mrCommentConfig, error) {
	var cfg mrCommentConfig

	cfg.Project, _ = raw["project"].(string)
	if cfg.Project == "" {
		return cfg, fmt.Errorf("config.project is required")
	}

	switch v := raw["mr_iid"].(type) {
	case int:
		cfg.MrIID = v
	case float64:
		cfg.MrIID = int(v)
	case string:
		if v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return cfg, fmt.Errorf("config.mr_iid is not a valid integer: %w", err)
			}
			cfg.MrIID = n
		}
	}
	if cfg.MrIID == 0 {
		return cfg, fmt.Errorf("config.mr_iid is required")
	}

	cfg.Body, _ = raw["body"].(string)
	if cfg.Body == "" {
		return cfg, fmt.Errorf("config.body is required")
	}

	cfg.Token, _ = raw["token"].(string)
	cfg.Token = os.ExpandEnv(cfg.Token)

	cfg.URL, _ = raw["url"].(string)

	return cfg, nil
}

func (s *mrCommentStep) Execute(
	ctx context.Context,
	_ map[string]any,
	_ map[string]map[string]any,
	_ map[string]any,
	_ map[string]any,
	_ map[string]any,
) (*sdk.StepResult, error) {
	if err := s.client.CommentOnMR(ctx, s.config.Project, s.config.MrIID, s.config.Body, s.config.Token); err != nil {
		return gitlabErrorResult(fmt.Sprintf("failed to comment on MR: %v", err)), nil
	}

	return &sdk.StepResult{
		Output: map[string]any{
			"commented": true,
			"project":   s.config.Project,
			"mr_iid":    s.config.MrIID,
		},
	}, nil
}

// gitlabErrorResult returns a StepResult that stops the pipeline with an error message.
func gitlabErrorResult(msg string) *sdk.StepResult {
	return &sdk.StepResult{
		StopPipeline: true,
		Output: map[string]any{
			"error": msg,
		},
	}
}

// ---- step.gitlab_parse_webhook ----

// parseWebhookStep parses a raw GitLab webhook payload from the current context.
//
//	config:
//	  source: "body"   # where to find the raw payload (default: body)
type parseWebhookStep struct {
	name   string
	source string
}

func newParseWebhookStep(name string, config map[string]any) (*parseWebhookStep, error) {
	source, _ := config["source"].(string)
	if source == "" {
		source = "body"
	}
	return &parseWebhookStep{name: name, source: source}, nil
}

func (s *parseWebhookStep) Execute(
	_ context.Context,
	triggerData map[string]any,
	_ map[string]map[string]any,
	current map[string]any,
	_ map[string]any,
	_ map[string]any,
) (*sdk.StepResult, error) {
	var raw map[string]any
	switch s.source {
	case "body":
		if b, ok := triggerData["body"].(map[string]any); ok {
			raw = b
		} else if b, ok := current["body"].(map[string]any); ok {
			raw = b
		}
	default:
		if b, ok := triggerData[s.source].(map[string]any); ok {
			raw = b
		}
	}

	if raw == nil {
		return &sdk.StepResult{Output: map[string]any{"parsed": false, "error": "no payload found"}}, nil
	}

	eventType, _ := triggerData["event_type"].(string)
	if eventType == "" {
		eventType, _ = current["event_type"].(string)
	}

	return &sdk.StepResult{
		Output: map[string]any{
			"parsed":     true,
			"event_type": eventType,
			"payload":    raw,
		},
	}, nil
}
