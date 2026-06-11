// Package internal implements workflow-plugin-gitlab, an external Workflow
// plugin for GitLab automation.
//
// The plugin exposes three integration surfaces:
//
//   - webhook modules (`git.webhook` and `gitlab.webhook`) that normalize
//     GitLab webhook payloads into Workflow trigger data.
//   - a `gitlab.client` module and CI/MR steps for triggering pipelines,
//     reading pipeline status, opening merge requests, and commenting on merge
//     requests.
//   - GitLab provider-management steps for CI/CD variables and environments.
//
// GitLab secrets are represented as GitLab CI/CD variables. Use
// `step.gitlab_secret_set` and `step.gitlab_secret_list` with `scope:
// project` for project variables or `scope: group` for group variables. GitLab
// environment-scoped variables are modeled with the `environment_scope` field,
// matching GitLab's API nomenclature; this is distinct from local `.env` files
// and from provider environments managed by `step.gitlab_environment_ensure`.
//
// GitLab environments are managed by `step.gitlab_environment_ensure` and
// inspected with `step.gitlab_environment_list`. Environment creation is
// idempotent: an existing environment with the requested name is returned
// unchanged, while a missing one is created through the official GitLab Go SDK.
//
// The production variable and environment client uses
// gitlab.com/gitlab-org/api/client-go/v2. Tests should inject the
// gitLabSecretsEnvironmentAPI interface instead of making live GitLab calls.
//
// The CI/MR steps also support strict protobuf contracts. The variable and
// environment steps are currently untyped step implementations because the
// Workflow secrets-provider contract is provider-specific and maps directly to
// GitLab's SDK structures.
package internal
