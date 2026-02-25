package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeEventType(t *testing.T) {
	cases := []struct {
		header   string
		expected string
	}{
		{"Push Hook", "push"},
		{"Tag Push Hook", "tag_push"},
		{"Merge Request Hook", "merge_request"},
		{"Pipeline Hook", "pipeline"},
		{"Note Hook", "note"},
		{"Job Hook", "job"},
		{"System Hook", "system"},
	}
	for _, tc := range cases {
		got := normalizeEventType(tc.header)
		if got != tc.expected {
			t.Errorf("normalizeEventType(%q): expected %q, got %q", tc.header, tc.expected, got)
		}
	}
}

func TestNormalizeGitLabEvent_Push(t *testing.T) {
	payload := `{
		"ref": "refs/heads/main",
		"checkout_sha": "abc123",
		"user_name": "alice",
		"commits": [{"message": "fix: bug", "author": {"name": "Alice"}}],
		"project": {"path_with_namespace": "group/repo", "web_url": "https://gitlab.com/group/repo"}
	}`

	event, err := normalizeGitLabEvent("push", []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertStr(t, "provider", "gitlab", event.Provider)
	assertStr(t, "event_type", "push", event.EventType)
	assertStr(t, "branch", "main", event.Branch)
	assertStr(t, "commit", "abc123", event.Commit)
	assertStr(t, "author", "Alice", event.Author)
	assertStr(t, "message", "fix: bug", event.Message)
	assertStr(t, "repository", "group/repo", event.Repository)
}

func TestNormalizeGitLabEvent_TagPush(t *testing.T) {
	payload := `{
		"ref": "refs/tags/v1.2.3",
		"checkout_sha": "deadbeef",
		"user_name": "bob",
		"project": {"path_with_namespace": "org/svc", "web_url": "https://gitlab.com/org/svc"}
	}`

	event, err := normalizeGitLabEvent("tag_push", []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertStr(t, "event_type", "tag_push", event.EventType)
	assertStr(t, "branch", "v1.2.3", event.Branch)
	assertStr(t, "commit", "deadbeef", event.Commit)
	assertStr(t, "author", "bob", event.Author)
}

func TestNormalizeGitLabEvent_MergeRequest(t *testing.T) {
	payload := `{
		"user": {"name": "carol"},
		"object_attributes": {
			"iid": 7,
			"title": "Add feature",
			"action": "opened",
			"source_branch": "feature-x",
			"last_commit": {"id": "fff000", "url": "https://gitlab.com/ns/proj/-/commit/fff000"}
		},
		"project": {"path_with_namespace": "ns/proj", "web_url": "https://gitlab.com/ns/proj"}
	}`

	event, err := normalizeGitLabEvent("merge_request", []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertStr(t, "event_type", "merge_request", event.EventType)
	assertStr(t, "author", "carol", event.Author)
	assertStr(t, "mr_action", "open", event.MRAction)
	assertStr(t, "branch", "feature-x", event.Branch)
	assertStr(t, "commit", "fff000", event.Commit)
	if event.MRNumber != 7 {
		t.Errorf("mr_number: expected 7, got %d", event.MRNumber)
	}
}

func TestNormalizeGitLabEvent_MRActionNormalization(t *testing.T) {
	cases := []struct{ raw, expected string }{
		{"opened", "open"},
		{"updated", "update"},
		{"merged", "merge"},
		{"closed", "close"},
		{"approved", "approved"},
	}
	for _, tc := range cases {
		got := normalizeMRAction(tc.raw)
		if got != tc.expected {
			t.Errorf("normalizeMRAction(%q): expected %q, got %q", tc.raw, tc.expected, got)
		}
	}
}

func TestNormalizeGitLabEvent_Pipeline(t *testing.T) {
	payload := `{
		"object_attributes": {"ref": "main", "sha": "beefdead"},
		"commit": {"message": "ci: update", "author": {"name": "dave"}},
		"project": {"path_with_namespace": "a/b", "web_url": "https://gitlab.com/a/b"}
	}`

	event, err := normalizeGitLabEvent("pipeline", []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertStr(t, "event_type", "pipeline", event.EventType)
	assertStr(t, "branch", "main", event.Branch)
	assertStr(t, "commit", "beefdead", event.Commit)
	assertStr(t, "author", "dave", event.Author)
}

func TestNormalizeGitLabEvent_InvalidJSON(t *testing.T) {
	_, err := normalizeGitLabEvent("push", []byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ---- webhook HTTP handler tests ----

func TestWebhookHandler_ValidToken(t *testing.T) {
	m, _ := newWebhookModule("test", map[string]any{
		"secret": "mysecret",
		"path":   "/webhooks/gitlab",
	})

	payload := pushPayload()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Token", "mysecret")
	req.Header.Set("X-Gitlab-Event", "Push Hook")

	rw := httptest.NewRecorder()
	m.handleWebhook(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rw.Code, rw.Body.String())
	}
}

func TestWebhookHandler_InvalidToken(t *testing.T) {
	m, _ := newWebhookModule("test", map[string]any{"secret": "mysecret"})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", strings.NewReader("{}"))
	req.Header.Set("X-Gitlab-Token", "wrong")
	req.Header.Set("X-Gitlab-Event", "Push Hook")

	rw := httptest.NewRecorder()
	m.handleWebhook(rw, req)

	if rw.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rw.Code)
	}
}

func TestWebhookHandler_MissingEventHeader(t *testing.T) {
	m, _ := newWebhookModule("test", map[string]any{})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", strings.NewReader("{}"))
	rw := httptest.NewRecorder()
	m.handleWebhook(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rw.Code)
	}
}

func TestWebhookHandler_EventFiltering(t *testing.T) {
	m, _ := newWebhookModule("test", map[string]any{
		"events": []any{"push"},
	})

	// merge_request event should be ignored
	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", strings.NewReader(mrPayload()))
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")

	rw := httptest.NewRecorder()
	m.handleWebhook(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200 (ignored), got %d", rw.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "ignored" {
		t.Errorf("expected status=ignored, got %v", resp["status"])
	}
}

func TestWebhookHandler_MethodNotAllowed(t *testing.T) {
	m, _ := newWebhookModule("test", map[string]any{})

	req := httptest.NewRequest(http.MethodGet, "/webhooks/gitlab", nil)
	rw := httptest.NewRecorder()
	m.handleWebhook(rw, req)

	if rw.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rw.Code)
	}
}

func TestWebhookHandler_PublishesEvent(t *testing.T) {
	m, _ := newWebhookModule("test", map[string]any{"topic": "git.events"})
	pub := &capturingPublisher{}
	m.SetMessagePublisher(pub)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", strings.NewReader(pushPayload()))
	req.Header.Set("X-Gitlab-Event", "Push Hook")

	rw := httptest.NewRecorder()
	m.handleWebhook(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rw.Code, rw.Body.String())
	}
	if len(pub.messages) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(pub.messages))
	}
	if pub.messages[0].topic != "git.events" {
		t.Errorf("expected topic git.events, got %s", pub.messages[0].topic)
	}
	// Verify the payload is a valid GitEvent
	var evt GitEvent
	if err := json.Unmarshal(pub.messages[0].payload, &evt); err != nil {
		t.Fatalf("failed to unmarshal published event: %v", err)
	}
	if evt.Provider != "gitlab" {
		t.Errorf("expected provider=gitlab, got %s", evt.Provider)
	}
}

// ---- helpers ----

func assertStr(t *testing.T, field, expected, got string) {
	t.Helper()
	if got != expected {
		t.Errorf("%s: expected %q, got %q", field, expected, got)
	}
}

func pushPayload() string {
	return `{"ref":"refs/heads/main","checkout_sha":"abc123","user_name":"alice","commits":[{"message":"fix","author":{"name":"Alice"}}],"project":{"path_with_namespace":"g/r","web_url":"https://gitlab.com/g/r"}}`
}

func mrPayload() string {
	return `{"user":{"name":"bob"},"object_attributes":{"iid":1,"title":"t","action":"opened","source_branch":"x","last_commit":{"id":"abc"}},"project":{"path_with_namespace":"a/b","web_url":"https://gitlab.com/a/b"}}`
}

type publishedMsg struct {
	topic   string
	payload []byte
	headers map[string]string
}

type capturingPublisher struct {
	messages []publishedMsg
}

func (p *capturingPublisher) Publish(topic string, payload []byte, headers map[string]string) (string, error) {
	p.messages = append(p.messages, publishedMsg{topic: topic, payload: payload, headers: headers})
	return "msg-id", nil
}
