# mock-github

A minimal fake GitHub API for local development and testing. Replaces the real GitHub API so migrations can run end-to-end without touching real repositories.

## What it does

- Implements the GitHub API endpoints the worker uses: create PR, get PR, list PRs, get file contents
- Stores PRs and file content in memory
- **Fires a webhook** to the worker (`POST /webhooks/github`) when a PR is merged, mimicking GitHub's `pull_request` event
- Comes with a simple HTML UI at `/` to browse all open/merged PRs, view file diffs, and merge PRs with a button click
- Seeds initial file content on startup (via `seedRepos()`) so step handlers have YAML to read

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/` | HTML dashboard — list all PRs |
| `GET` | `/:owner/:repo/pull/:number` | HTML PR page — view diff, merge button |
| `POST` | `/:owner/:repo/pull/:number/merge` | Merge a PR (triggers webhook) |
| `POST` | `/repos/:owner/:repo/pulls` | Create a PR |
| `GET` | `/repos/:owner/:repo/pulls/:number` | Get a PR |
| `GET` | `/repos/:owner/:repo/pulls` | List PRs |
| `GET` | `/repos/:owner/:repo/contents/*path` | Get file contents (base64 encoded) |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBHOOK_URL` | `http://localhost:3001/webhooks/github` | Worker webhook endpoint |
| `PORT` | `9090` | HTTP listen port |
