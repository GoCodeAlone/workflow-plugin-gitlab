package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GoCodeAlone/workflow-plugin-gitlab/internal/contracts"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func typedTriggerPipeline(client GitLabClient) sdk.TypedStepHandler[*contracts.TriggerPipelineConfig, *contracts.TriggerPipelineInput, *contracts.PipelineOutput] {
	return func(ctx context.Context, req sdk.TypedStepRequest[*contracts.TriggerPipelineConfig, *contracts.TriggerPipelineInput]) (*sdk.TypedStepResult[*contracts.PipelineOutput], error) {
		step, err := newTriggerPipelineStep("", mergeMessages(req.Config, req.Input), client)
		if err != nil {
			return &sdk.TypedStepResult[*contracts.PipelineOutput]{Output: &contracts.PipelineOutput{Error: err.Error()}, StopPipeline: true}, nil
		}
		result, err := step.Execute(ctx, req.TriggerData, req.StepOutputs, req.Current, req.Metadata, nil)
		if err != nil {
			return nil, err
		}
		return &sdk.TypedStepResult[*contracts.PipelineOutput]{Output: pipelineOutputFromMap(result.Output), StopPipeline: result.StopPipeline}, nil
	}
}

func typedPipelineStatus(client GitLabClient) sdk.TypedStepHandler[*contracts.PipelineStatusConfig, *contracts.PipelineStatusInput, *contracts.PipelineOutput] {
	return func(ctx context.Context, req sdk.TypedStepRequest[*contracts.PipelineStatusConfig, *contracts.PipelineStatusInput]) (*sdk.TypedStepResult[*contracts.PipelineOutput], error) {
		step, err := newPipelineStatusStep("", mergeMessages(req.Config, req.Input), client)
		if err != nil {
			return &sdk.TypedStepResult[*contracts.PipelineOutput]{Output: &contracts.PipelineOutput{Error: err.Error()}, StopPipeline: true}, nil
		}
		result, err := step.Execute(ctx, req.TriggerData, req.StepOutputs, req.Current, req.Metadata, nil)
		if err != nil {
			return nil, err
		}
		return &sdk.TypedStepResult[*contracts.PipelineOutput]{Output: pipelineOutputFromMap(result.Output), StopPipeline: result.StopPipeline}, nil
	}
}

func typedCreateMergeRequest(client GitLabClient) sdk.TypedStepHandler[*contracts.CreateMergeRequestConfig, *contracts.CreateMergeRequestInput, *contracts.MergeRequestOutput] {
	return func(ctx context.Context, req sdk.TypedStepRequest[*contracts.CreateMergeRequestConfig, *contracts.CreateMergeRequestInput]) (*sdk.TypedStepResult[*contracts.MergeRequestOutput], error) {
		step, err := newCreateMRStep("", mergeMessages(req.Config, req.Input), client)
		if err != nil {
			return &sdk.TypedStepResult[*contracts.MergeRequestOutput]{Output: &contracts.MergeRequestOutput{Error: err.Error()}, StopPipeline: true}, nil
		}
		result, err := step.Execute(ctx, req.TriggerData, req.StepOutputs, req.Current, req.Metadata, nil)
		if err != nil {
			return nil, err
		}
		return &sdk.TypedStepResult[*contracts.MergeRequestOutput]{Output: mergeRequestOutputFromMap(result.Output), StopPipeline: result.StopPipeline}, nil
	}
}

func typedMRComment(client GitLabClient) sdk.TypedStepHandler[*contracts.MRCommentConfig, *contracts.MRCommentInput, *contracts.MRCommentOutput] {
	return func(ctx context.Context, req sdk.TypedStepRequest[*contracts.MRCommentConfig, *contracts.MRCommentInput]) (*sdk.TypedStepResult[*contracts.MRCommentOutput], error) {
		step, err := newMRCommentStep("", mergeMessages(req.Config, req.Input), client)
		if err != nil {
			return &sdk.TypedStepResult[*contracts.MRCommentOutput]{Output: &contracts.MRCommentOutput{Error: err.Error()}, StopPipeline: true}, nil
		}
		result, err := step.Execute(ctx, req.TriggerData, req.StepOutputs, req.Current, req.Metadata, nil)
		if err != nil {
			return nil, err
		}
		return &sdk.TypedStepResult[*contracts.MRCommentOutput]{Output: mrCommentOutputFromMap(result.Output), StopPipeline: result.StopPipeline}, nil
	}
}

func typedParseWebhook() sdk.TypedStepHandler[*contracts.ParseWebhookConfig, *contracts.ParseWebhookInput, *contracts.ParseWebhookOutput] {
	return func(ctx context.Context, req sdk.TypedStepRequest[*contracts.ParseWebhookConfig, *contracts.ParseWebhookInput]) (*sdk.TypedStepResult[*contracts.ParseWebhookOutput], error) {
		step, err := newParseWebhookStep("", protoMessageToMap(req.Config))
		if err != nil {
			return &sdk.TypedStepResult[*contracts.ParseWebhookOutput]{Output: &contracts.ParseWebhookOutput{Error: err.Error()}}, nil
		}
		triggerData := map[string]any{}
		if req.Input != nil {
			if req.Input.GetEventType() != "" {
				triggerData["event_type"] = req.Input.GetEventType()
			}
			if req.Input.GetBody() != nil {
				triggerData["body"] = req.Input.GetBody().AsMap()
			}
			for key, value := range req.Input.GetFields() {
				triggerData[key] = value.AsInterface()
			}
		}
		result, err := step.Execute(ctx, triggerData, req.StepOutputs, req.Current, req.Metadata, nil)
		if err != nil {
			return nil, err
		}
		return &sdk.TypedStepResult[*contracts.ParseWebhookOutput]{Output: parseWebhookOutputFromMap(result.Output), StopPipeline: result.StopPipeline}, nil
	}
}

func (m *gitlabClientModule) InvokeMethod(method string, args map[string]any) (map[string]any, error) {
	client := m.NewHTTPClient()
	if m.token == "mock" {
		client = &mockGitLabClient{}
	}
	token := strVal(args, "token")
	if token == "" {
		token = m.token
	}
	switch method {
	case "trigger_pipeline":
		variables := mapStringString(args["variables"])
		ref := strVal(args, "ref")
		if ref == "" {
			ref = "main"
		}
		pipeline, err := client.TriggerPipeline(context.Background(), strVal(args, "project"), ref, variables, token)
		if err != nil {
			return nil, err
		}
		return pipelineToMap(pipeline), nil
	case "pipeline_status":
		pipelineID, _ := toInt(args["pipeline_id"])
		pipeline, err := client.GetPipeline(context.Background(), strVal(args, "project"), pipelineID, token)
		if err != nil {
			return nil, err
		}
		return pipelineToMap(pipeline), nil
	case "create_merge_request":
		mr, err := client.CreateMergeRequest(context.Background(), strVal(args, "project"), MergeRequestOptions{
			SourceBranch: strVal(args, "source_branch"),
			TargetBranch: strVal(args, "target_branch"),
			Title:        strVal(args, "title"),
			Description:  strVal(args, "description"),
		}, token)
		if err != nil {
			return nil, err
		}
		return mergeRequestToMap(mr), nil
	case "mr_comment":
		mrIID, _ := toInt(args["mr_iid"])
		if err := client.CommentOnMR(context.Background(), strVal(args, "project"), mrIID, strVal(args, "body"), token); err != nil {
			return nil, err
		}
		return map[string]any{"commented": true, "project": strVal(args, "project"), "mr_iid": mrIID}, nil
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

func (m *gitlabClientModule) InvokeTypedMethod(method string, input *anypb.Any) (*anypb.Any, error) {
	switch method {
	case "trigger_pipeline":
		args, err := unpackTypedArgs(input, &contracts.TriggerPipelineInput{})
		if err != nil {
			return nil, err
		}
		out, err := m.InvokeMethod(method, mergeMessages(&contracts.TriggerPipelineConfig{Token: m.token}, args))
		if err != nil {
			return nil, err
		}
		return anypb.New(pipelineOutputFromMap(out))
	case "pipeline_status":
		args, err := unpackTypedArgs(input, &contracts.PipelineStatusInput{})
		if err != nil {
			return nil, err
		}
		out, err := m.InvokeMethod(method, mergeMessages(&contracts.PipelineStatusConfig{Token: m.token}, args))
		if err != nil {
			return nil, err
		}
		return anypb.New(pipelineOutputFromMap(out))
	case "create_merge_request":
		args, err := unpackTypedArgs(input, &contracts.CreateMergeRequestInput{})
		if err != nil {
			return nil, err
		}
		out, err := m.InvokeMethod(method, mergeMessages(&contracts.CreateMergeRequestConfig{Token: m.token}, args))
		if err != nil {
			return nil, err
		}
		return anypb.New(mergeRequestOutputFromMap(out))
	case "mr_comment":
		args, err := unpackTypedArgs(input, &contracts.MRCommentInput{})
		if err != nil {
			return nil, err
		}
		out, err := m.InvokeMethod(method, mergeMessages(&contracts.MRCommentConfig{Token: m.token}, args))
		if err != nil {
			return nil, err
		}
		return anypb.New(mrCommentOutputFromMap(out))
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

func unpackTypedArgs[T proto.Message](input *anypb.Any, target T) (T, error) {
	if input == nil {
		var zero T
		return zero, fmt.Errorf("typed input is required")
	}
	if input.MessageName() != target.ProtoReflect().Descriptor().FullName() {
		var zero T
		return zero, fmt.Errorf("typed input type mismatch: expected %s, got %s", target.ProtoReflect().Descriptor().FullName(), input.MessageName())
	}
	if err := input.UnmarshalTo(target); err != nil {
		var zero T
		return zero, err
	}
	return target, nil
}

func mergeMessages(messages ...proto.Message) map[string]any {
	merged := map[string]any{}
	for _, msg := range messages {
		for key, value := range protoMessageToMap(msg) {
			if isZeroTypedValue(value) {
				continue
			}
			merged[key] = value
		}
	}
	return merged
}

func protoMessageToMap(msg proto.Message) map[string]any {
	if msg == nil {
		return nil
	}
	raw, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(msg)
	if err != nil {
		return nil
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}

func isZeroTypedValue(value any) bool {
	switch v := value.(type) {
	case string:
		return v == ""
	case float64:
		return v == 0
	case bool:
		return !v
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return value == nil
	}
}

func pipelineToMap(pipeline *Pipeline) map[string]any {
	if pipeline == nil {
		return nil
	}
	return map[string]any{
		"pipeline_id": pipeline.ID,
		"status":      pipeline.Status,
		"ref":         pipeline.Ref,
		"sha":         pipeline.SHA,
		"web_url":     pipeline.WebURL,
		"created_at":  pipeline.CreatedAt,
	}
}

func mergeRequestToMap(mr *MergeRequest) map[string]any {
	if mr == nil {
		return nil
	}
	return map[string]any{
		"mr_id":         mr.ID,
		"mr_iid":        mr.IID,
		"title":         mr.Title,
		"state":         mr.State,
		"source_branch": mr.SourceBranch,
		"target_branch": mr.TargetBranch,
		"web_url":       mr.WebURL,
	}
}

func pipelineOutputFromMap(values map[string]any) *contracts.PipelineOutput {
	return &contracts.PipelineOutput{
		PipelineId: int32Val(values, "pipeline_id"),
		Status:     strVal(values, "status"),
		Ref:        strVal(values, "ref"),
		Sha:        strVal(values, "sha"),
		WebUrl:     strVal(values, "web_url"),
		CreatedAt:  strVal(values, "created_at"),
		Error:      strVal(values, "error"),
	}
}

func mergeRequestOutputFromMap(values map[string]any) *contracts.MergeRequestOutput {
	return &contracts.MergeRequestOutput{
		MrId:         int32Val(values, "mr_id"),
		MrIid:        int32Val(values, "mr_iid"),
		Title:        strVal(values, "title"),
		State:        strVal(values, "state"),
		SourceBranch: strVal(values, "source_branch"),
		TargetBranch: strVal(values, "target_branch"),
		WebUrl:       strVal(values, "web_url"),
		Error:        strVal(values, "error"),
	}
}

func mrCommentOutputFromMap(values map[string]any) *contracts.MRCommentOutput {
	return &contracts.MRCommentOutput{
		Commented: boolVal(values, "commented"),
		Project:   strVal(values, "project"),
		MrIid:     int32Val(values, "mr_iid"),
		Error:     strVal(values, "error"),
	}
}

func parseWebhookOutputFromMap(values map[string]any) *contracts.ParseWebhookOutput {
	output := &contracts.ParseWebhookOutput{
		Parsed:    boolVal(values, "parsed"),
		EventType: strVal(values, "event_type"),
		Error:     strVal(values, "error"),
	}
	if payload, ok := values["payload"].(map[string]any); ok {
		output.Payload, _ = structpb.NewStruct(payload)
	}
	return output
}

func int32Val(values map[string]any, key string) int32 {
	if values == nil {
		return 0
	}
	n, _ := toInt(values[key])
	return int32(n)
}

func boolVal(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	v, _ := values[key].(bool)
	return v
}

func strVal(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if v, ok := values[key].(string); ok {
		return v
	}
	return ""
}

func toInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		n, err := v.Int64()
		return int(n), err == nil
	default:
		return 0, false
	}
}

func mapStringString(value any) map[string]string {
	switch vars := value.(type) {
	case map[string]string:
		return vars
	case map[string]any:
		out := make(map[string]string, len(vars))
		for key, value := range vars {
			out[key] = fmt.Sprintf("%v", value)
		}
		return out
	default:
		return nil
	}
}
