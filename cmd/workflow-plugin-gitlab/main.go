// Command workflow-plugin-gitlab is a workflow engine external plugin that
// provides GitLab integration: webhook handling and GitLab CI pipeline management.
// It runs as a subprocess and communicates with the host workflow engine via
// the go-plugin protocol.
package main

import (
	"github.com/GoCodeAlone/workflow-plugin-gitlab/internal"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

func main() {
	sdk.Serve(internal.NewGitLabPlugin(), sdk.WithBuildVersion(sdk.ResolveBuildVersion(internal.Version)))
}
