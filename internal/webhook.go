package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// GitEvent is the normalized event schema published to the message broker.
// It matches the common schema used by workflow-plugin-github so consumers
// can handle events from either provider uniformly.
type GitEvent struct {
	Provider   string          `json:"provider"`    // "gitlab"
	EventType  string          `json:"event_type"`  // "push", "merge_request", "tag_push", "pipeline"
	Repository string          `json:"repository"`  // "group/project"
	Branch     string          `json:"branch"`      // ref without "refs/heads/" prefix
	Commit     string          `json:"commit"`      // SHA
	Author     string          `json:"author"`      // username or display name
	Message    string          `json:"message"`     // commit message or MR title
	URL        string          `json:"url"`         // link to project/MR/commit
	RawPayload json.RawMessage `json:"raw_payload"` // original payload
	Timestamp  time.Time       `json:"timestamp"`
	// GitLab-specific MR fields
	MRNumber int    `json:"mr_number,omitempty"`
	MRTitle  string `json:"mr_title,omitempty"`
	MRAction string `json:"mr_action,omitempty"` // "open", "update", "merge", "close"
}

// webhookModule implements sdk.ModuleInstance and sdk.MessageAwareModule.
// It registers an HTTP handler at /webhooks/gitlab (configurable) that validates
// X-Gitlab-Token and publishes normalized GitEvent messages to a topic.
type webhookModule struct {
	name   string
	config webhookConfig

	publisher sdk.MessagePublisher
}

// webhookConfig holds the parsed configuration for a git.webhook module (GitLab).
type webhookConfig struct {
	Provider string   `yaml:"provider"`
	Secret   string   `yaml:"secret"`
	Path     string   `yaml:"path"`
	Events   []string `yaml:"events"`
	Topic    string   `yaml:"topic"`
}

// newWebhookModule parses the config map and returns a webhookModule.
func newWebhookModule(name string, config map[string]any) (*webhookModule, error) {
	cfg, err := parseWebhookConfig(config)
	if err != nil {
		return nil, fmt.Errorf("git.webhook %q: %w", name, err)
	}
	return &webhookModule{
		name:   name,
		config: cfg,
	}, nil
}

// parseWebhookConfig converts a raw config map to webhookConfig.
func parseWebhookConfig(raw map[string]any) (webhookConfig, error) {
	var cfg webhookConfig

	provider, _ := raw["provider"].(string)
	if provider == "" {
		provider = "gitlab"
	}
	cfg.Provider = provider

	cfg.Secret, _ = raw["secret"].(string)
	cfg.Secret = os.ExpandEnv(cfg.Secret)

	cfg.Path, _ = raw["path"].(string)
	if cfg.Path == "" {
		cfg.Path = "/webhooks/gitlab"
	}

	if events, ok := raw["events"].([]any); ok {
		for _, e := range events {
			if s, ok := e.(string); ok {
				cfg.Events = append(cfg.Events, s)
			}
		}
	}

	topic, _ := raw["topic"].(string)
	if topic == "" {
		topic = "git.events"
	}
	cfg.Topic = topic

	return cfg, nil
}

// SetMessagePublisher is called by the engine to inject the message publisher.
func (m *webhookModule) SetMessagePublisher(pub sdk.MessagePublisher) {
	m.publisher = pub
}

// SetMessageSubscriber is a no-op; this module only publishes.
func (m *webhookModule) SetMessageSubscriber(_ sdk.MessageSubscriber) {}

// Init is a no-op; the module is ready after construction.
func (m *webhookModule) Init() error { return nil }

// Start registers the HTTP webhook handler.
func (m *webhookModule) Start(_ context.Context) error {
	http.HandleFunc(m.config.Path, m.handleWebhook)
	return nil
}

// Stop is a no-op.
func (m *webhookModule) Stop(_ context.Context) error { return nil }

// Name returns the module name.
func (m *webhookModule) Name() string { return m.name }

// handleWebhook is the HTTP handler for incoming GitLab webhook events.
func (m *webhookModule) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 25*1024*1024))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Validate X-Gitlab-Token when a secret is configured.
	if m.config.Secret != "" {
		token := r.Header.Get("X-Gitlab-Token")
		if token != m.config.Secret {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
	}

	eventHeader := r.Header.Get("X-Gitlab-Event")
	if eventHeader == "" {
		http.Error(w, "missing X-Gitlab-Event header", http.StatusBadRequest)
		return
	}

	eventType := normalizeEventType(eventHeader)

	// Filter to configured event types if specified.
	if len(m.config.Events) > 0 && !containsString(m.config.Events, eventType) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored"}`))
		return
	}

	event, err := normalizeGitLabEvent(eventType, body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to normalize event: %v", err), http.StatusBadRequest)
		return
	}

	if m.publisher != nil {
		payload, err := json.Marshal(event)
		if err != nil {
			http.Error(w, "failed to marshal event", http.StatusInternalServerError)
			return
		}
		_, err = m.publisher.Publish(m.config.Topic, payload, map[string]string{
			"event_type": event.EventType,
			"provider":   event.Provider,
			"repository": event.Repository,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to publish event: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

// normalizeEventType converts the X-Gitlab-Event header value to a lower-case
// canonical event type string compatible with the common git event schema.
func normalizeEventType(header string) string {
	lower := strings.ToLower(header)
	switch lower {
	case "push hook":
		return "push"
	case "tag push hook":
		return "tag_push"
	case "merge request hook":
		return "merge_request"
	case "pipeline hook":
		return "pipeline"
	case "note hook":
		return "note"
	case "job hook":
		return "job"
	default:
		s := strings.TrimSuffix(lower, " hook")
		return strings.ReplaceAll(s, " ", "_")
	}
}

// normalizeGitLabEvent converts a raw GitLab webhook payload into a GitEvent.
func normalizeGitLabEvent(eventType string, body []byte) (*GitEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	event := &GitEvent{
		Provider:   "gitlab",
		EventType:  eventType,
		RawPayload: json.RawMessage(body),
		Timestamp:  time.Now().UTC(),
	}

	// Extract common project fields.
	if project, ok := payload["project"].(map[string]any); ok {
		event.Repository, _ = project["path_with_namespace"].(string)
		event.URL, _ = project["web_url"].(string)
	}

	switch eventType {
	case "push":
		normalizePushEvent(event, payload)
	case "tag_push":
		normalizeTagPushEvent(event, payload)
	case "merge_request":
		normalizeMREvent(event, payload)
	case "pipeline":
		normalizePipelineEvent(event, payload)
	default:
		normalizeGenericEvent(event, payload)
	}

	return event, nil
}

// normalizePushEvent extracts fields from a GitLab push event.
func normalizePushEvent(event *GitEvent, payload map[string]any) {
	ref, _ := payload["ref"].(string)
	event.Branch = strings.TrimPrefix(ref, "refs/heads/")
	event.Commit, _ = payload["checkout_sha"].(string)

	if commits, ok := payload["commits"].([]any); ok && len(commits) > 0 {
		if last, ok := commits[len(commits)-1].(map[string]any); ok {
			event.Message, _ = last["message"].(string)
			if author, ok := last["author"].(map[string]any); ok {
				event.Author, _ = author["name"].(string)
			}
		}
	}
	if event.Author == "" {
		event.Author, _ = payload["user_name"].(string)
	}
}

// normalizeTagPushEvent extracts fields from a GitLab tag push event.
func normalizeTagPushEvent(event *GitEvent, payload map[string]any) {
	ref, _ := payload["ref"].(string)
	event.Branch = strings.TrimPrefix(ref, "refs/tags/")
	event.Commit, _ = payload["checkout_sha"].(string)
	event.Author, _ = payload["user_name"].(string)
}

// normalizeMREvent extracts fields from a GitLab merge_request event.
func normalizeMREvent(event *GitEvent, payload map[string]any) {
	if ua, ok := payload["user"].(map[string]any); ok {
		event.Author, _ = ua["name"].(string)
	}

	if oa, ok := payload["object_attributes"].(map[string]any); ok {
		event.Branch, _ = oa["source_branch"].(string)
		event.Message, _ = oa["title"].(string)
		event.MRTitle, _ = oa["title"].(string)
		iid, _ := oa["iid"].(float64)
		event.MRNumber = int(iid)
		action, _ := oa["action"].(string)
		event.MRAction = normalizeMRAction(action)

		if lc, ok := oa["last_commit"].(map[string]any); ok {
			event.Commit, _ = lc["id"].(string)
			if event.URL == "" {
				event.URL, _ = lc["url"].(string)
			}
		}
	}
}

// normalizePipelineEvent extracts fields from a GitLab pipeline event.
func normalizePipelineEvent(event *GitEvent, payload map[string]any) {
	if oa, ok := payload["object_attributes"].(map[string]any); ok {
		event.Branch, _ = oa["ref"].(string)
		event.Commit, _ = oa["sha"].(string)
	}
	if commit, ok := payload["commit"].(map[string]any); ok {
		if author, ok := commit["author"].(map[string]any); ok {
			event.Author, _ = author["name"].(string)
		}
		event.Message, _ = commit["message"].(string)
	}
}

// normalizeGenericEvent does best-effort extraction.
func normalizeGenericEvent(event *GitEvent, payload map[string]any) {
	if user, ok := payload["user"].(map[string]any); ok {
		event.Author, _ = user["name"].(string)
	}
	if event.Author == "" {
		event.Author, _ = payload["user_name"].(string)
	}
}

// normalizeMRAction maps GitLab MR actions to the common vocabulary.
func normalizeMRAction(action string) string {
	switch action {
	case "opened":
		return "open"
	case "updated":
		return "update"
	case "merged":
		return "merge"
	case "closed":
		return "close"
	default:
		return action
	}
}

// containsString reports whether slice contains s.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
