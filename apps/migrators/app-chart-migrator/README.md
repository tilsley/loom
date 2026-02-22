# worker

The migration worker. It owns all domain knowledge about what a migration actually does.

## What it does

- **Announces itself** to the server on startup by publishing a `MigrationAnnouncement` to Dapr pub/sub (`migration-registry` topic). The announcement contains the full list of steps, their config, and which files they touch. The server saves this and exposes it to the console.
- **Receives step dispatch** messages from the server via Dapr pub/sub (`migration-steps` topic). Each message says which step to run and for which target repo.
- **Executes step handlers** — each step type creates or modifies files and opens a GitHub PR. The step type is determined by `Config["type"]` in the dispatch message.
- **Calls back** to the server's `/event/:id/pr-opened` as soon as the PR is open (so the console can show the PR link immediately).
- **Waits** for GitHub webhooks on `POST /webhooks/github`. When a PR is merged, it fires the step-completed callback to `POST /event/:id`, which signals the server's workflow to advance to the next step.

## Step types

| Type | What it does |
|------|-------------|
| `generate-app-chart` | Creates an app-specific Helm chart with per-env values and an OCI publish workflow |
| `swap-chart` | Updates the Argo Application overlay to point at the new OCI chart |
| `disable-sync-prune` | Adds `Prune=false` sync option to the Argo overlay |
| `enable-sync-prune` | Removes the `Prune=false` sync option |
| `disable-resource-prune` | Adds `Prune=false` to non-Argo resources in the base |
| `cleanup-common` | Removes old helm values from the base application |
| `update-deploy-workflow` | Updates the CI workflow for app chart deployment |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GITHUB_API_URL` | `http://localhost:9090` | GitHub API base URL (point at mock-github locally) |
| `LOOM_URL` | `http://localhost:8080` | Server base URL for callbacks |
| `GITOPS_REPO` | `acme/gitops` | `owner/repo` of the GitOps repository |
| `ENVS` | `dev,staging,prod` | Comma-separated list of environments |
| `PORT` | `3001` | HTTP listen port |
