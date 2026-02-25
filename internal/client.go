package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitLabClient is the interface for interacting with the GitLab REST API v4.
// It is defined as an interface so tests can inject a mock.
type GitLabClient interface {
	TriggerPipeline(ctx context.Context, projectID, ref string, variables map[string]string, token string) (*Pipeline, error)
	GetPipeline(ctx context.Context, projectID string, pipelineID int, token string) (*Pipeline, error)
	CreateMergeRequest(ctx context.Context, projectID string, opts MergeRequestOptions, token string) (*MergeRequest, error)
	CommentOnMR(ctx context.Context, projectID string, mrIID int, body, token string) error
}

// Pipeline represents a GitLab CI pipeline.
type Pipeline struct {
	ID        int    `json:"id"`
	Status    string `json:"status"`
	Ref       string `json:"ref"`
	SHA       string `json:"sha"`
	WebURL    string `json:"web_url"`
	CreatedAt string `json:"created_at"`
}

// MergeRequest represents a GitLab merge request.
type MergeRequest struct {
	ID           int    `json:"id"`
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	WebURL       string `json:"web_url"`
}

// MergeRequestOptions holds parameters for creating a merge request.
type MergeRequestOptions struct {
	SourceBranch string
	TargetBranch string
	Title        string
	Description  string
}

// httpGitLabClient implements GitLabClient using net/http.
type httpGitLabClient struct {
	baseURL    string
	httpClient *http.Client
}

// newHTTPGitLabClient returns a production GitLab API client.
func newHTTPGitLabClient(baseURL string) GitLabClient {
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	return &httpGitLabClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an authenticated request to the GitLab API v4.
func (c *httpGitLabClient) doRequest(ctx context.Context, method, path string, body any, token string) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// encodeProjectID URL-encodes a project path (e.g. "group/project" → "group%2Fproject").
func encodeProjectID(id string) string {
	return strings.ReplaceAll(id, "/", "%2F")
}

// TriggerPipeline triggers a GitLab CI pipeline on the given ref.
func (c *httpGitLabClient) TriggerPipeline(ctx context.Context, projectID, ref string, variables map[string]string, token string) (*Pipeline, error) {
	body := map[string]any{"ref": ref}
	if len(variables) > 0 {
		vars := make([]map[string]string, 0, len(variables))
		for k, v := range variables {
			vars = append(vars, map[string]string{"key": k, "value": v})
		}
		body["variables"] = vars
	}

	path := fmt.Sprintf("/api/v4/projects/%s/pipeline", encodeProjectID(projectID))
	respBody, status, err := c.doRequest(ctx, http.MethodPost, path, body, token)
	if err != nil {
		return nil, fmt.Errorf("trigger pipeline: %w", err)
	}
	if status != http.StatusCreated {
		return nil, fmt.Errorf("trigger pipeline: unexpected status %d: %s", status, string(respBody))
	}

	var pipeline Pipeline
	if err := json.Unmarshal(respBody, &pipeline); err != nil {
		return nil, fmt.Errorf("parse pipeline response: %w", err)
	}
	return &pipeline, nil
}

// GetPipeline retrieves the status of a GitLab CI pipeline.
func (c *httpGitLabClient) GetPipeline(ctx context.Context, projectID string, pipelineID int, token string) (*Pipeline, error) {
	path := fmt.Sprintf("/api/v4/projects/%s/pipelines/%d", encodeProjectID(projectID), pipelineID)
	respBody, status, err := c.doRequest(ctx, http.MethodGet, path, nil, token)
	if err != nil {
		return nil, fmt.Errorf("get pipeline: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get pipeline: unexpected status %d", status)
	}

	var pipeline Pipeline
	if err := json.Unmarshal(respBody, &pipeline); err != nil {
		return nil, fmt.Errorf("parse pipeline response: %w", err)
	}
	return &pipeline, nil
}

// CreateMergeRequest creates a GitLab merge request.
func (c *httpGitLabClient) CreateMergeRequest(ctx context.Context, projectID string, opts MergeRequestOptions, token string) (*MergeRequest, error) {
	body := map[string]any{
		"source_branch": opts.SourceBranch,
		"target_branch": opts.TargetBranch,
		"title":         opts.Title,
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}

	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests", encodeProjectID(projectID))
	respBody, status, err := c.doRequest(ctx, http.MethodPost, path, body, token)
	if err != nil {
		return nil, fmt.Errorf("create MR: %w", err)
	}
	if status != http.StatusCreated {
		return nil, fmt.Errorf("create MR: unexpected status %d: %s", status, string(respBody))
	}

	var mr MergeRequest
	if err := json.Unmarshal(respBody, &mr); err != nil {
		return nil, fmt.Errorf("parse MR response: %w", err)
	}
	return &mr, nil
}

// CommentOnMR posts a note on a GitLab merge request.
func (c *httpGitLabClient) CommentOnMR(ctx context.Context, projectID string, mrIID int, body, token string) error {
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/notes", encodeProjectID(projectID), mrIID)
	_, status, err := c.doRequest(ctx, http.MethodPost, path, map[string]any{"body": body}, token)
	if err != nil {
		return fmt.Errorf("comment on MR: %w", err)
	}
	if status != http.StatusCreated {
		return fmt.Errorf("comment on MR: unexpected status %d", status)
	}
	return nil
}

// mockGitLabClient returns canned responses for testing without a real GitLab instance.
type mockGitLabClient struct{}

func (m *mockGitLabClient) TriggerPipeline(_ context.Context, projectID, ref string, _ map[string]string, _ string) (*Pipeline, error) {
	return &Pipeline{
		ID:        42,
		Status:    "created",
		Ref:       ref,
		SHA:       "abc123def456",
		WebURL:    "https://gitlab.example.com/" + projectID + "/-/pipelines/42",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (m *mockGitLabClient) GetPipeline(_ context.Context, projectID string, pipelineID int, _ string) (*Pipeline, error) {
	return &Pipeline{
		ID:     pipelineID,
		Status: "success",
		Ref:    "main",
		SHA:    "abc123def456",
		WebURL: fmt.Sprintf("https://gitlab.example.com/%s/-/pipelines/%d", projectID, pipelineID),
	}, nil
}

func (m *mockGitLabClient) CreateMergeRequest(_ context.Context, projectID string, opts MergeRequestOptions, _ string) (*MergeRequest, error) {
	return &MergeRequest{
		ID:           100,
		IID:          1,
		Title:        opts.Title,
		State:        "opened",
		SourceBranch: opts.SourceBranch,
		TargetBranch: opts.TargetBranch,
		WebURL:       "https://gitlab.example.com/" + projectID + "/-/merge_requests/1",
	}, nil
}

func (m *mockGitLabClient) CommentOnMR(_ context.Context, _ string, _ int, _, _ string) error {
	return nil
}
