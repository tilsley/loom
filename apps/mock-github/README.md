# mock-github

A minimal fake GitHub API for local development and testing. Replaces the real GitHub API so migrations can run end-to-end without touching real repositories.

## What it does

- Implements the GitHub API endpoints the migrator uses: create PR, get PR, list PRs, get file contents, download tarball
- Stores PRs and file content in memory
- **Fires a webhook** to the migrator (`POST /webhooks/github`) when a PR is merged, mimicking GitHub's `pull_request` event
- Comes with a simple HTML UI at `/` to browse all open/merged PRs, view file diffs, and merge PRs with a button click
- Seeds initial file content on startup (via `seedRepos()`) so step handlers have YAML to read

## Endpoints

### HTML UI

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/` | Dashboard — list all PRs |
| `GET` | `/:owner/:repo/pull/:number` | PR detail page — view diff, merge button |
| `POST` | `/:owner/:repo/pull/:number/merge` | Merge a PR (triggers webhook to migrator) |

### GitHub-compatible API

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/repos/:owner/:repo/pulls` | Create a PR |
| `GET` | `/repos/:owner/:repo/pulls/:number` | Get a PR |
| `GET` | `/repos/:owner/:repo/pulls` | List PRs |
| `GET` | `/repos/:owner/:repo/contents/*path` | Get file contents (base64) or directory listing |
| `GET` | `/repos/:owner/:repo/tarball/:ref` | Download repo as gzip tarball |
| `GET` | `/health` | Health check |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBHOOK_URL` | `http://localhost:3001/webhooks/github` | Migrator webhook endpoint (note: migrator defaults to port 8082 — override this in dev if needed) |
| `PORT` | `9090` | HTTP listen port |
