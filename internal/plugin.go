// Package internal implements the workflow-plugin-gitlab external plugin,
// providing GitLab webhook handling and GitLab CI pipeline management.
package internal

import (
	"fmt"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// Version is set at build time via -ldflags
// "-X github.com/GoCodeAlone/workflow-plugin-gitlab/internal.Version=X.Y.Z".
// Default is a bare semver so plugin loaders that validate semver accept
// unreleased dev builds; goreleaser overrides with the real release tag.
var Version = "0.0.0"

// gitlabPlugin implements sdk.PluginProvider, sdk.ModuleProvider, and sdk.StepProvider.
type gitlabPlugin struct{}

// NewGitLabPlugin returns a new gitlabPlugin instance.
func NewGitLabPlugin() sdk.PluginProvider {
	return &gitlabPlugin{}
}

// Manifest returns plugin metadata.
func (p *gitlabPlugin) Manifest() sdk.PluginManifest {
	return sdk.PluginManifest{
		Name:        "workflow-plugin-gitlab",
		Version:     Version,
		Author:      "GoCodeAlone",
		Description: "GitLab integration plugin: webhook handling and GitLab CI pipeline management",
	}
}

// ModuleTypes returns the module type names this plugin provides.
func (p *gitlabPlugin) ModuleTypes() []string {
	return []string{
		"git.webhook",
		"gitlab.webhook",
		"gitlab.client",
	}
}

// CreateModule creates a module instance of the given type.
func (p *gitlabPlugin) CreateModule(typeName, name string, config map[string]any) (sdk.ModuleInstance, error) {
	switch typeName {
	case "git.webhook", "gitlab.webhook":
		return newWebhookModule(name, config)
	case "gitlab.client":
		return newClientModule(name, config)
	default:
		return nil, fmt.Errorf("gitlab plugin: unknown module type %q", typeName)
	}
}

// StepTypes returns the step type names this plugin provides.
func (p *gitlabPlugin) StepTypes() []string {
	return []string{
		"step.gitlab_trigger_pipeline",
		"step.gitlab_pipeline_status",
		"step.gitlab_create_merge_request",
		"step.gitlab_create_mr",
		"step.gitlab_mr_comment",
		"step.gitlab_parse_webhook",
	}
}

// CreateStep creates a step instance of the given type.
func (p *gitlabPlugin) CreateStep(typeName, name string, config map[string]any) (sdk.StepInstance, error) {
	switch typeName {
	case "step.gitlab_trigger_pipeline":
		return newTriggerPipelineStep(name, config, nil)
	case "step.gitlab_pipeline_status":
		return newPipelineStatusStep(name, config, nil)
	case "step.gitlab_create_merge_request", "step.gitlab_create_mr":
		return newCreateMRStep(name, config, nil)
	case "step.gitlab_mr_comment":
		return newMRCommentStep(name, config, nil)
	case "step.gitlab_parse_webhook":
		return newParseWebhookStep(name, config)
	default:
		return nil, fmt.Errorf("gitlab plugin: unknown step type %q", typeName)
	}
}
