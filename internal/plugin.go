// Package internal implements the workflow-plugin-gitlab external plugin,
// providing GitLab webhook handling and GitLab CI pipeline management.
package internal

import (
	"fmt"

	"github.com/GoCodeAlone/workflow-plugin-gitlab/internal/contracts"
	pb "github.com/GoCodeAlone/workflow/plugin/external/proto"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
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

// TypedModuleTypes returns the protobuf-typed module type names this plugin provides.
func (p *gitlabPlugin) TypedModuleTypes() []string {
	return p.ModuleTypes()
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

// CreateTypedModule creates a typed module instance of the given type.
func (p *gitlabPlugin) CreateTypedModule(typeName, name string, config *anypb.Any) (sdk.ModuleInstance, error) {
	switch typeName {
	case "git.webhook", "gitlab.webhook":
		factory := sdk.NewTypedModuleFactory(typeName, &contracts.WebhookConfig{}, func(name string, cfg *contracts.WebhookConfig) (sdk.ModuleInstance, error) {
			return newWebhookModule(name, protoMessageToMap(cfg))
		})
		return factory.CreateTypedModule(typeName, name, config)
	case "gitlab.client":
		factory := sdk.NewTypedModuleFactory("gitlab.client", &contracts.GitLabClientConfig{}, func(name string, cfg *contracts.GitLabClientConfig) (sdk.ModuleInstance, error) {
			return newClientModule(name, protoMessageToMap(cfg))
		})
		return factory.CreateTypedModule(typeName, name, config)
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
		"step.gitlab_secret_set",
		"step.gitlab_secret_list",
		"step.gitlab_environment_ensure",
		"step.gitlab_environment_list",
	}
}

// TypedStepTypes returns the protobuf-typed step type names this plugin provides.
func (p *gitlabPlugin) TypedStepTypes() []string {
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
	case "step.gitlab_secret_set":
		return newGitLabSecretSetStep(name, config, nil)
	case "step.gitlab_secret_list":
		return newGitLabSecretListStep(name, config, nil)
	case "step.gitlab_environment_ensure":
		return newGitLabEnvironmentEnsureStep(name, config, nil)
	case "step.gitlab_environment_list":
		return newGitLabEnvironmentListStep(name, config, nil)
	default:
		return nil, fmt.Errorf("gitlab plugin: unknown step type %q", typeName)
	}
}

// CreateTypedStep creates a typed step instance of the given type.
func (p *gitlabPlugin) CreateTypedStep(typeName, name string, config *anypb.Any) (sdk.StepInstance, error) {
	switch typeName {
	case "step.gitlab_trigger_pipeline":
		factory := sdk.NewTypedStepFactory(typeName, &contracts.TriggerPipelineConfig{}, &contracts.TriggerPipelineInput{}, typedTriggerPipeline(nil))
		return factory.CreateTypedStep(typeName, name, config)
	case "step.gitlab_pipeline_status":
		factory := sdk.NewTypedStepFactory(typeName, &contracts.PipelineStatusConfig{}, &contracts.PipelineStatusInput{}, typedPipelineStatus(nil))
		return factory.CreateTypedStep(typeName, name, config)
	case "step.gitlab_create_merge_request", "step.gitlab_create_mr":
		factory := sdk.NewTypedStepFactory(typeName, &contracts.CreateMergeRequestConfig{}, &contracts.CreateMergeRequestInput{}, typedCreateMergeRequest(nil))
		return factory.CreateTypedStep(typeName, name, config)
	case "step.gitlab_mr_comment":
		factory := sdk.NewTypedStepFactory(typeName, &contracts.MRCommentConfig{}, &contracts.MRCommentInput{}, typedMRComment(nil))
		return factory.CreateTypedStep(typeName, name, config)
	case "step.gitlab_parse_webhook":
		factory := sdk.NewTypedStepFactory(typeName, &contracts.ParseWebhookConfig{}, &contracts.ParseWebhookInput{}, typedParseWebhook())
		return factory.CreateTypedStep(typeName, name, config)
	default:
		return nil, fmt.Errorf("gitlab plugin: unknown step type %q", typeName)
	}
}

// ContractRegistry returns strict protobuf descriptors for plugin boundaries.
func (p *gitlabPlugin) ContractRegistry() *pb.ContractRegistry {
	return &pb.ContractRegistry{
		FileDescriptorSet: &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
			protodesc.ToFileDescriptorProto(structpb.File_google_protobuf_struct_proto),
			protodesc.ToFileDescriptorProto(contracts.File_internal_contracts_gitlab_proto),
		}},
		Contracts: []*pb.ContractDescriptor{
			moduleContract("git.webhook", "WebhookConfig"),
			moduleContract("gitlab.webhook", "WebhookConfig"),
			moduleContract("gitlab.client", "GitLabClientConfig"),
			stepContract("step.gitlab_trigger_pipeline", "TriggerPipelineConfig", "TriggerPipelineInput", "PipelineOutput"),
			stepContract("step.gitlab_pipeline_status", "PipelineStatusConfig", "PipelineStatusInput", "PipelineOutput"),
			stepContract("step.gitlab_create_merge_request", "CreateMergeRequestConfig", "CreateMergeRequestInput", "MergeRequestOutput"),
			stepContract("step.gitlab_create_mr", "CreateMergeRequestConfig", "CreateMergeRequestInput", "MergeRequestOutput"),
			stepContract("step.gitlab_mr_comment", "MRCommentConfig", "MRCommentInput", "MRCommentOutput"),
			stepContract("step.gitlab_parse_webhook", "ParseWebhookConfig", "ParseWebhookInput", "ParseWebhookOutput"),
			serviceContract("trigger_pipeline", "TriggerPipelineInput", "PipelineOutput"),
			serviceContract("pipeline_status", "PipelineStatusInput", "PipelineOutput"),
			serviceContract("create_merge_request", "CreateMergeRequestInput", "MergeRequestOutput"),
			serviceContract("mr_comment", "MRCommentInput", "MRCommentOutput"),
		},
	}
}

func moduleContract(moduleType, configMessage string) *pb.ContractDescriptor {
	const pkg = "workflow.plugins.gitlab.v1."
	return &pb.ContractDescriptor{
		Kind:          pb.ContractKind_CONTRACT_KIND_MODULE,
		ModuleType:    moduleType,
		ConfigMessage: pkg + configMessage,
		Mode:          pb.ContractMode_CONTRACT_MODE_STRICT_PROTO,
	}
}

func stepContract(stepType, configMessage, inputMessage, outputMessage string) *pb.ContractDescriptor {
	const pkg = "workflow.plugins.gitlab.v1."
	return &pb.ContractDescriptor{
		Kind:          pb.ContractKind_CONTRACT_KIND_STEP,
		StepType:      stepType,
		ConfigMessage: pkg + configMessage,
		InputMessage:  pkg + inputMessage,
		OutputMessage: pkg + outputMessage,
		Mode:          pb.ContractMode_CONTRACT_MODE_STRICT_PROTO,
	}
}

func serviceContract(method, inputMessage, outputMessage string) *pb.ContractDescriptor {
	const pkg = "workflow.plugins.gitlab.v1."
	return &pb.ContractDescriptor{
		Kind:          pb.ContractKind_CONTRACT_KIND_SERVICE,
		ModuleType:    "gitlab.client",
		ServiceName:   "gitlab.client",
		Method:        method,
		InputMessage:  pkg + inputMessage,
		OutputMessage: pkg + outputMessage,
		Mode:          pb.ContractMode_CONTRACT_MODE_STRICT_PROTO,
	}
}
