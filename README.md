# workflow-plugin-gitlab

> ⚠️ **Experimental** — This plugin compiles and passes its unit tests but has not been validated in any active GoCodeAlone-internal production deployment. Use with caution. Please [open an issue](https://github.com/GoCodeAlone/workflow-plugin-gitlab/issues/new) if you adopt it so we can promote it to **verified** status.

GitLab integration plugin for Workflow. It supports GitLab webhook ingestion,
pipeline and merge-request automation, GitLab CI/CD variables, and GitLab
environment management.

The production GitLab API integration uses the official
[`gitlab.com/gitlab-org/api/client-go/v2`](https://gitlab.com/gitlab-org/api/client-go)
SDK. Configure `url` only for GitLab self-managed installations; omit it for
`https://gitlab.com`.

## Required Secret

| Name | Sensitive | Purpose |
|------|-----------|---------|
| `GITLAB_TOKEN` | yes | Personal, project, or group access token with API permissions for the operations used by your workflows. |

## Modules

### `gitlab.client`

Declares reusable GitLab API connection settings for service-style calls. Steps
can reference the module with `client: <module name>`, or provide `token` and
`url` directly when a reusable module is unnecessary.

```yaml
modules:
  - name: gitlab
    type: gitlab.client
    config:
      token: ${GITLAB_TOKEN}
      # url: https://gitlab.example.com
```

### `gitlab.webhook`

Normalizes GitLab webhook payloads. If `secret` is set, incoming requests must
include the matching `X-Gitlab-Token` header.

```yaml
modules:
  - name: gitlab-webhook
    type: gitlab.webhook
    config:
      secret: ${GITLAB_WEBHOOK_SECRET}
```

The plugin also accepts `git.webhook` as a compatibility alias for
`gitlab.webhook`.

## Pipeline and Merge Request Steps

```yaml
steps:
  - name: trigger-build
    type: step.gitlab_trigger_pipeline
    config:
      client: gitlab
      project: group/project
      ref: main
      variables:
        DEPLOY_ENV: staging

  - name: open-mr
    type: step.gitlab_create_merge_request
    config:
      client: gitlab
      project: group/project
      source_branch: feature/docs
      target_branch: main
      title: "docs: refresh generated API reference"
```

## GitLab CI/CD Variables

GitLab secrets are GitLab CI/CD variables. `step.gitlab_secret_set` creates or
updates a variable, and `step.gitlab_secret_list` reports variables visible at
the selected scope.

Supported variable locations:

- `project`: set or list variables on one GitLab project.
- `group`: set or list variables on one GitLab group.

GitLab's `environment_scope` is a separate optional variable field, not a
`scope` value. Use it to narrow project or group variables to an environment
pattern such as `production` or `review/*`. It is not a local `.env` scope.

```yaml
steps:
  - name: ensure-project-variable
    type: step.gitlab_secret_set
    config:
      scope: project
      project: group/project
      key: DEPLOY_TOKEN
      value: ${DEPLOY_TOKEN}
      environment_scope: production
      masked: true
      protected: true
      token: ${GITLAB_TOKEN}

  - name: list-production-variables
    type: step.gitlab_secret_list
    config:
      scope: project
      project: group/project
      environment_scope: production
      token: ${GITLAB_TOKEN}
```

For shared group variables:

```yaml
steps:
  - name: ensure-group-variable
    type: step.gitlab_secret_set
    config:
      scope: group
      group: platform
      key: SHARED_DEPLOY_TOKEN
      value: ${SHARED_DEPLOY_TOKEN}
      masked: true
      token: ${GITLAB_TOKEN}
```

## GitLab Environments

GitLab environments are provider resources and are separate from
`environment_scope` on CI/CD variables. Use
`step.gitlab_environment_ensure` to idempotently create an environment and
`step.gitlab_environment_list` to inspect existing environments.

```yaml
steps:
  - name: ensure-production-environment
    type: step.gitlab_environment_ensure
    config:
      project: group/project
      name: production
      tier: production
      external_url: https://app.example.com
      token: ${GITLAB_TOKEN}

  - name: list-environments
    type: step.gitlab_environment_list
    config:
      project: group/project
      token: ${GITLAB_TOKEN}
```

## Go Integration Notes

The public plugin entrypoint is `internal.NewGitLabPlugin`, which is served by
`cmd/workflow-plugin-gitlab`. Go-level consumers normally interact with this
plugin through Workflow's external plugin host. Tests that exercise variable or
environment behavior should inject the `gitLabSecretsEnvironmentAPI` interface
instead of making live GitLab API calls.

The pipeline and merge-request steps are strict-protobuf typed steps. The
variable and environment steps are currently untyped Workflow steps because they
map directly to provider-specific GitLab SDK request shapes.

## Development

```sh
GOWORK=off go test ./...
GOWORK=off go test -race ./...
```
