package internal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-gitlab/internal/contracts"
	pb "github.com/GoCodeAlone/workflow/plugin/external/proto"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestNewGitLabPluginImplementsStrictContractProviders(t *testing.T) {
	provider := NewGitLabPlugin()
	if _, ok := provider.(sdk.TypedModuleProvider); !ok {
		t.Fatal("expected TypedModuleProvider")
	}
	if _, ok := provider.(sdk.TypedStepProvider); !ok {
		t.Fatal("expected TypedStepProvider")
	}
	if _, ok := provider.(sdk.ContractProvider); !ok {
		t.Fatal("expected ContractProvider")
	}
}

func TestContractRegistryDeclaresStrictModuleStepAndServiceContracts(t *testing.T) {
	provider := NewGitLabPlugin().(sdk.ContractProvider)
	registry := provider.ContractRegistry()
	if registry == nil {
		t.Fatal("expected contract registry")
	}
	if registry.FileDescriptorSet == nil || len(registry.FileDescriptorSet.File) == 0 {
		t.Fatal("expected file descriptor set")
	}
	files, err := protodesc.NewFiles(registry.FileDescriptorSet)
	if err != nil {
		t.Fatalf("descriptor set: %v", err)
	}
	manifestContracts := loadManifestContracts(t)

	contractsByKey := map[string]*pb.ContractDescriptor{}
	for _, contract := range registry.Contracts {
		switch contract.Kind {
		case pb.ContractKind_CONTRACT_KIND_MODULE:
			contractsByKey["module:"+contract.ModuleType] = contract
		case pb.ContractKind_CONTRACT_KIND_STEP:
			contractsByKey["step:"+contract.StepType] = contract
		case pb.ContractKind_CONTRACT_KIND_SERVICE:
			contractsByKey["service:"+contract.ServiceName+"/"+contract.Method] = contract
		default:
			t.Fatalf("unexpected contract kind %s", contract.Kind)
		}
		for _, name := range []string{contract.ConfigMessage, contract.InputMessage, contract.OutputMessage} {
			if name == "" {
				continue
			}
			if _, err := files.FindDescriptorByName(protoreflect.FullName(name)); err != nil {
				t.Fatalf("%s references unknown message %s: %v", contractKey(contract), name, err)
			}
		}
	}

	for _, key := range []string{
		"module:git.webhook",
		"module:gitlab.webhook",
		"module:gitlab.client",
		"step:step.gitlab_trigger_pipeline",
		"step:step.gitlab_pipeline_status",
		"step:step.gitlab_create_merge_request",
		"step:step.gitlab_create_mr",
		"step:step.gitlab_mr_comment",
		"step:step.gitlab_parse_webhook",
		"service:gitlab.client/trigger_pipeline",
		"service:gitlab.client/pipeline_status",
		"service:gitlab.client/create_merge_request",
		"service:gitlab.client/mr_comment",
	} {
		contract, ok := contractsByKey[key]
		if !ok {
			t.Fatalf("missing contract %s", key)
		}
		if contract.Mode != pb.ContractMode_CONTRACT_MODE_STRICT_PROTO {
			t.Fatalf("%s mode = %s, want strict proto", key, contract.Mode)
		}
		if want, ok := manifestContracts[key]; !ok {
			t.Fatalf("%s missing from plugin.contracts.json", key)
		} else if want.ConfigMessage != contract.ConfigMessage || want.InputMessage != contract.InputMessage || want.OutputMessage != contract.OutputMessage {
			t.Fatalf("%s manifest contract = %#v, runtime = %#v", key, want, contract)
		}
	}
	if len(manifestContracts) != len(contractsByKey) {
		t.Fatalf("plugin.contracts.json contract count = %d, runtime = %d", len(manifestContracts), len(contractsByKey))
	}
}

func TestTypedTriggerPipelineProviderValidatesTypedConfig(t *testing.T) {
	provider := NewGitLabPlugin().(sdk.TypedStepProvider)
	config, err := anypb.New(&contracts.TriggerPipelineConfig{
		Project: "group/project",
		Token:   "mock",
	})
	if err != nil {
		t.Fatalf("pack config: %v", err)
	}
	step, err := provider.CreateTypedStep("step.gitlab_trigger_pipeline", "trigger", config)
	if err != nil {
		t.Fatalf("CreateTypedStep: %v", err)
	}
	if _, err := step.Execute(context.Background(), nil, nil, nil, nil, nil); err == nil {
		t.Fatal("legacy Execute succeeded for typed-only step")
	}

	wrongConfig, err := anypb.New(&contracts.PipelineStatusConfig{Project: "group/project"})
	if err != nil {
		t.Fatalf("pack wrong config: %v", err)
	}
	if _, err := provider.CreateTypedStep("step.gitlab_trigger_pipeline", "trigger", wrongConfig); err == nil {
		t.Fatal("CreateTypedStep accepted wrong typed config")
	}
}

func TestTypedTriggerPipelineStepExecutesWithMockClient(t *testing.T) {
	result, err := typedTriggerPipeline(&mockGitLabClient{})(context.Background(), sdk.TypedStepRequest[*contracts.TriggerPipelineConfig, *contracts.TriggerPipelineInput]{
		Config: &contracts.TriggerPipelineConfig{
			Project: "group/project",
			Token:   "mock",
		},
		Input: &contracts.TriggerPipelineInput{Ref: "main"},
	})
	if err != nil {
		t.Fatalf("typedTriggerPipeline: %v", err)
	}
	if result.Output.GetPipelineId() != 42 {
		t.Fatalf("pipeline_id = %d, want 42", result.Output.GetPipelineId())
	}
	if result.Output.GetStatus() != "created" {
		t.Fatalf("status = %q, want created", result.Output.GetStatus())
	}
}

func TestTypedClientServicePipelineStatus(t *testing.T) {
	provider := NewGitLabPlugin().(sdk.TypedModuleProvider)
	config, err := anypb.New(&contracts.GitLabClientConfig{Token: "mock"})
	if err != nil {
		t.Fatalf("pack config: %v", err)
	}
	module, err := provider.CreateTypedModule("gitlab.client", "client", config)
	if err != nil {
		t.Fatalf("CreateTypedModule: %v", err)
	}
	invoker, ok := module.(sdk.TypedServiceInvoker)
	if !ok {
		t.Fatal("expected TypedServiceInvoker")
	}
	input, err := anypb.New(&contracts.PipelineStatusInput{
		Project:    "group/project",
		PipelineId: 77,
	})
	if err != nil {
		t.Fatalf("pack input: %v", err)
	}
	output, err := invoker.InvokeTypedMethod("pipeline_status", input)
	if err != nil {
		t.Fatalf("InvokeTypedMethod: %v", err)
	}
	var pipeline contracts.PipelineOutput
	if err := output.UnmarshalTo(&pipeline); err != nil {
		t.Fatalf("unpack output: %v", err)
	}
	if pipeline.GetPipelineId() != 77 {
		t.Fatalf("pipeline_id = %d, want 77", pipeline.GetPipelineId())
	}
	if pipeline.GetStatus() != "success" {
		t.Fatalf("status = %q, want success", pipeline.GetStatus())
	}
}

type manifestContract struct {
	Mode          string `json:"mode"`
	ConfigMessage string `json:"config"`
	InputMessage  string `json:"input"`
	OutputMessage string `json:"output"`
}

func loadManifestContracts(t *testing.T) map[string]manifestContract {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "plugin.contracts.json"))
	if err != nil {
		t.Fatalf("read plugin.contracts.json: %v", err)
	}
	var manifest struct {
		Version   string `json:"version"`
		Contracts []struct {
			Kind        string `json:"kind"`
			Type        string `json:"type"`
			ServiceName string `json:"serviceName"`
			Method      string `json:"method"`
			manifestContract
		} `json:"contracts"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse plugin.contracts.json: %v", err)
	}
	if manifest.Version != "v1" {
		t.Fatalf("plugin.contracts.json version = %q, want v1", manifest.Version)
	}
	contracts := make(map[string]manifestContract, len(manifest.Contracts))
	for _, contract := range manifest.Contracts {
		if contract.Mode != "strict" {
			t.Fatalf("%s mode = %q, want strict", contract.Type, contract.Mode)
		}
		var key string
		switch contract.Kind {
		case "module":
			key = "module:" + contract.Type
		case "step":
			key = "step:" + contract.Type
		case "service_method":
			key = "service:" + contract.ServiceName + "/" + contract.Method
		default:
			t.Fatalf("unexpected contract kind %q in plugin.contracts.json", contract.Kind)
		}
		if _, exists := contracts[key]; exists {
			t.Fatalf("duplicate contract %q in plugin.contracts.json", key)
		}
		contracts[key] = contract.manifestContract
	}
	return contracts
}

func contractKey(contract *pb.ContractDescriptor) string {
	switch contract.Kind {
	case pb.ContractKind_CONTRACT_KIND_MODULE:
		return "module:" + contract.ModuleType
	case pb.ContractKind_CONTRACT_KIND_STEP:
		return "step:" + contract.StepType
	case pb.ContractKind_CONTRACT_KIND_SERVICE:
		return "service:" + contract.ServiceName + "/" + contract.Method
	default:
		return contract.Kind.String()
	}
}
