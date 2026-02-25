---
name: add-step
description: Add a new step handler to the app-chart-migrator. Use when the user wants to add a new step type to the migrator worker.
disable-model-invocation: true
argument-hint: [step-name]
---

Add a new step handler named `$ARGUMENTS` to the app-chart-migrator. Follow the existing patterns exactly.

## Three files to touch

### 1. Create the step file

Create `apps/migrators/app-chart-migrator/internal/steps/<step_name>.go` (use underscores in the filename, e.g. `disable_sync_prune.go`).

Pattern to follow — see `disable_sync_prune.go` as the canonical example:

```go
package steps

import (
    "context"
    "fmt"

    "github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/gitrepo"
    "github.com/tilsley/loom/pkg/api"
)

// <StepName> does ...
type <StepName> struct{}

// Execute implements Handler.
func (h *<StepName>) Execute(
    ctx context.Context,
    gr gitrepo.Client,
    cfg *Config,
    req api.DispatchStepRequest,
) (*Result, error) {
    // Use these helpers:
    //   appName(req.Candidate)                  — logical app/service name
    //   gitopsFileForEnv(req.Candidate, env)    — ArgoCD manifest path for env
    //   candidateOwnerRepo(req.Candidate, cfg.GitopsOwner) — (owner, repo)
    //   (*req.Config)["key"]                    — step config values
    //   gr.GetContents(ctx, owner, repo, path)  — read a file

    // Return a Result describing the PR to create:
    return &Result{
        Owner:  cfg.GitopsOwner,
        Repo:   cfg.GitopsRepo,
        Title:  fmt.Sprintf("[%s] <action> for %s", req.MigrationId, appName(req.Candidate)),
        Body:   "PR description",
        Branch: fmt.Sprintf("loom/%s/%s--%s", req.MigrationId, req.StepName, req.Candidate.Id),
        Files:  map[string]string{"path/to/file.yaml": "new content"},
    }, nil
}
```

For manual-review steps (no PR, just show instructions in UI):
```go
return &Result{Instructions: "1. Do this\n2. Do that"}, nil
```

### 2. Register in the registry

Edit `apps/migrators/app-chart-migrator/internal/steps/registry.go` — add one line:

```go
"<step-name>": &<StepName>{},
```

Key is kebab-case (matches `StepDefinition.Type` from the server). Value is a pointer to the new struct.

The dryrun runner uses this registry automatically — no dryrun changes needed.

### 3. Expose in the announcement

Edit `apps/migrators/app-chart-migrator/main.go` inside `buildAnnouncement()`. Add a `StepDefinition` to `stepDefs` in the correct position:

```go
api.StepDefinition{
    Name:        "<step-name>",           // unique name for this step instance
    Description: strPtr("<description>"), // shown in the UI
    WorkerApp:   "app-chart-migrator",
    Type:        strPtr("<step-name>"),    // must match the registry key
    Config:      &map[string]string{"env": env}, // if step needs config
},
```

Per-env steps use `Name: "<step-type>-" + env` so each environment gets its own step instance.

## Verify

```bash
make vet
make test
```
