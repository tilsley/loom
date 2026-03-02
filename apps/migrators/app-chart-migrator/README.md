# app-chart-migrator

A Loom migrator that handles Helm chart upgrades across ArgoCD applications. It owns all domain knowledge about what the migration actually does.

## What it does

- **Announces itself** to the server on startup by POSTing a `MigrationAnnouncement` to `/registry/announce`. The announcement contains the full list of steps, the overview, required inputs, and the `migratorUrl` the server should dispatch to.
- **Discovers candidates** by scanning the GitOps repo for applications and submitting them to the server via `POST /migrations/:id/candidates`.
- **Receives step dispatch** requests from the server via `POST /dispatch-step`. Each request says which step to run and for which candidate.
- **Executes step handlers** — each step type creates or modifies files and opens a GitHub PR. The step type is determined by the `Type` field in the dispatch request.
- **Sends updates** to the server's `/event/:id` as soon as a PR is open (so the console can show the PR link immediately via metadata).
- **Receives GitHub webhooks** on `POST /webhooks/github`. When a PR is merged, it fires the step-completed callback to `/event/:id`, which signals the server's Run to advance to the next step.
- **Dry-run** — responds to `POST /dry-run` with simulated file diffs (no real PRs).

## Step types

| Type | What it does |
|------|-------------|
| `disable-base-resource-prune` | Adds `Prune=false` to non-Argo resources in the base |
| `generate-app-chart` | Creates an app-specific Helm chart with per-env values and an OCI publish workflow |
| `manual-review` | Pauses for operator approval (e.g. verify ECR publish, review ArgoCD health) |
| `disable-sync-prune` | Adds `Prune=false` sync option to the Argo overlay (per-env) |
| `swap-chart` | Updates the Argo Application overlay to point at the new OCI chart (per-env) |
| `enable-sync-prune` | Removes the `Prune=false` sync option (per-env) |
| `cleanup-common` | Removes old helm values from the base application |
| `update-deploy-workflow` | Updates the CI workflow for app chart deployment |

## HTTP endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/dispatch-step` | Server dispatches a step to execute |
| `POST` | `/webhooks/github` | GitHub webhook for PR merge events |
| `POST` | `/dry-run` | Simulate all steps, return file diffs |
| `GET` | `/health` | Health check |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOOM_URL` | `http://localhost:8080` | Server base URL for announce + callbacks |
| `WORKER_URL` | `http://localhost:8082` | This migrator's externally-reachable URL (registered as `migratorUrl`) |
| `GITHUB_API_URL` | `http://localhost:9090` | GitHub API base URL (point at mock-github locally) |
| `GITHUB_TOKEN` | _(empty)_ | GitHub personal access token (local dev / CI) |
| `GITHUB_APP_ID` | _(empty)_ | GitHub App ID (deployed auth — used with installation ID + key) |
| `GITHUB_APP_INSTALLATION_ID` | _(empty)_ | GitHub App installation ID |
| `GITHUB_APP_PRIVATE_KEY_PATH` | _(empty)_ | Path to GitHub App private key PEM file |
| `GITOPS_REPO` | `tilsley/gitops` | `owner/repo` of the GitOps repository |
| `ENVS` | `dev,staging,prod` | Comma-separated list of environments |
| `REDIS_ADDR` | `localhost:6379` | Redis address (used for pending callback store) |
| `PORT` | `8082` | HTTP listen port |
