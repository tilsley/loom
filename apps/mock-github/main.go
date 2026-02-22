package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// DirEntry is a file or directory returned by the GitHub contents API directory listing.
type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "dir"
}

const stateMerged = "merged"

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Head    string `json:"head"`
	Base    string `json:"base"`
	State   string `json:"state"`
}

// CreatePRRequest is the request body for creating a pull request.
type CreatePRRequest struct {
	Title string            `json:"title"           binding:"required"`
	Body  string            `json:"body"`
	Head  string            `json:"head"            binding:"required"`
	Base  string            `json:"base"            binding:"required"`
	Files map[string]string `json:"files,omitempty"`
}

// store holds PRs and file content keyed by "owner/repo".
type store struct {
	mu       sync.RWMutex
	prs      map[string][]PullRequest     // key: "owner/repo"
	counters map[string]int               // key: "owner/repo"
	files    map[string]map[string]string // repo key → path → content (main branch)
	prFiles  map[string]map[string]string // PR key "owner/repo#number" → path → content
}

func newStore() *store {
	return &store{
		prs:      make(map[string][]PullRequest),
		counters: make(map[string]int),
		files:    make(map[string]map[string]string),
		prFiles:  make(map[string]map[string]string),
	}
}

func (s *store) create(owner, repo string, req CreatePRRequest) PullRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := owner + "/" + repo
	s.counters[key]++
	num := s.counters[key]

	pr := PullRequest{
		Number:  num,
		HTMLURL: fmt.Sprintf("http://localhost:9090/%s/pull/%d", key, num),
		Title:   req.Title,
		Body:    req.Body,
		Head:    req.Head,
		Base:    req.Base,
		State:   "open",
	}
	s.prs[key] = append(s.prs[key], pr)

	// Store PR files (pending changes)
	if len(req.Files) > 0 {
		prKey := fmt.Sprintf("%s#%d", key, num)
		s.prFiles[prKey] = req.Files
	}

	return pr
}

func (s *store) get(owner, repo string, number int) *PullRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, pr := range s.prs[owner+"/"+repo] {
		if pr.Number == number {
			return &pr
		}
	}
	return nil
}

func (s *store) merge(owner, repo string, number int) *PullRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := owner + "/" + repo
	for i, pr := range s.prs[key] {
		if pr.Number != number {
			continue
		}
		s.prs[key][i].State = stateMerged

		// Apply PR files to main branch
		prKey := fmt.Sprintf("%s#%d", key, number)
		if files, ok := s.prFiles[prKey]; ok {
			if s.files[key] == nil {
				s.files[key] = make(map[string]string)
			}
			for path, content := range files {
				s.files[key][path] = content
			}
			delete(s.prFiles, prKey)
		}

		merged := s.prs[key][i]
		return &merged
	}
	return nil
}

func (s *store) list(owner, repo string) []PullRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prs := s.prs[owner+"/"+repo]
	if prs == nil {
		return []PullRequest{}
	}
	return prs
}

func (s *store) listAll() []PullRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, prs := range s.prs {
		total += len(prs)
	}
	all := make([]PullRequest, 0, total)
	for _, prs := range s.prs {
		all = append(all, prs...)
	}
	return all
}

// listDir returns the immediate children of dirPath in the repo, similar to
// GitHub's GET /repos/:owner/:repo/contents/:path when :path is a directory.
func (s *store) listDir(owner, repo, dirPath string) []DirEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := owner + "/" + repo
	files := s.files[key]
	if files == nil {
		return nil
	}

	// Build prefix for matching: "apps" → "apps/"
	prefix := dirPath
	if prefix != "" {
		prefix += "/"
	}

	seen := map[string]bool{}
	var entries []DirEntry
	for filePath := range files {
		if !strings.HasPrefix(filePath, prefix) {
			continue
		}
		rest := filePath[len(prefix):]
		idx := strings.Index(rest, "/")
		var name, entryType string
		if idx == -1 {
			name, entryType = rest, "file"
		} else {
			name, entryType = rest[:idx], "dir"
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, DirEntry{
			Name: name,
			Path: dirPath + "/" + name,
			Type: entryType,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

func (s *store) getFile(owner, repo, path string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := owner + "/" + repo
	if files, ok := s.files[key]; ok {
		if content, ok := files[path]; ok {
			return content, true
		}
	}
	return "", false
}

func (s *store) getPRFiles(owner, repo string, number int) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prKey := fmt.Sprintf("%s/%s#%d", owner, repo, number)
	return s.prFiles[prKey]
}

func (s *store) getAllFiles(owner, repo string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := owner + "/" + repo
	result := make(map[string]string)
	for path, content := range s.files[key] {
		result[path] = content
	}
	return result
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := newStore()

	// Seed repos with initial file content
	seedRepos(s)
	log.Info("seeded repos", "repos", len(s.files))

	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		webhookURL = "http://localhost:3001/webhooks/github"
	}

	r := gin.Default()
	registerHTMLRoutes(r, s, log, webhookURL)
	registerAPIRoutes(r, s, log)

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	log.Info("mock-github starting", "port", port, "webhookURL", webhookURL)
	if err := r.Run(":" + port); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func registerHTMLRoutes(r *gin.Engine, s *store, log *slog.Logger, webhookURL string) {
	r.GET("/", func(c *gin.Context) {
		prs := s.listAll()
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, renderDashboard(prs))
	})

	r.GET("/:owner/:repo/pull/:number", func(c *gin.Context) {
		owner := c.Param("owner")
		repo := c.Param("repo")
		num, err := strconv.Atoi(c.Param("number"))
		if err != nil {
			c.String(http.StatusBadRequest, "invalid PR number")
			return
		}
		pr := s.get(owner, repo, num)
		if pr == nil {
			c.String(http.StatusNotFound, "pull request not found")
			return
		}
		files := s.getPRFiles(owner, repo, num)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, renderPRPage(owner, repo, pr, files))
	})

	r.POST("/:owner/:repo/pull/:number/merge", func(c *gin.Context) {
		owner := c.Param("owner")
		repo := c.Param("repo")
		num, err := strconv.Atoi(c.Param("number"))
		if err != nil {
			c.String(http.StatusBadRequest, "invalid PR number")
			return
		}
		pr := s.merge(owner, repo, num)
		if pr == nil {
			c.String(http.StatusNotFound, "pull request not found")
			return
		}
		log.Info("PR merged", "owner", owner, "repo", repo, "number", num)
		go sendWebhook(log, webhookURL, owner, repo, pr)
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/%s/%s/pull/%d", owner, repo, num))
	})
}

func registerAPIRoutes(r *gin.Engine, s *store, log *slog.Logger) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/repos/:owner/:repo/pulls", func(c *gin.Context) {
		owner := c.Param("owner")
		repo := c.Param("repo")
		var req CreatePRRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		pr := s.create(owner, repo, req)
		log.Info(
			"PR created",
			"owner",
			owner,
			"repo",
			repo,
			"number",
			pr.Number,
			"title",
			pr.Title,
			"files",
			len(req.Files),
		)
		c.JSON(http.StatusCreated, pr)
	})

	r.GET("/repos/:owner/:repo/pulls/:number", func(c *gin.Context) {
		owner := c.Param("owner")
		repo := c.Param("repo")
		num, err := strconv.Atoi(c.Param("number"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PR number"})
			return
		}
		pr := s.get(owner, repo, num)
		if pr == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "pull request not found"})
			return
		}
		c.JSON(http.StatusOK, pr)
	})

	r.GET("/repos/:owner/:repo/pulls", func(c *gin.Context) {
		owner := c.Param("owner")
		repo := c.Param("repo")
		c.JSON(http.StatusOK, s.list(owner, repo))
	})

	// Tarball endpoint — returns the full repo as a gzip-compressed tar archive.
	// Mirrors GitHub's GET /repos/:owner/:repo/tarball/:ref endpoint.
	// The archive wraps all files under a single top-level directory named
	// "{owner}-{repo}-main/" so clients can strip the prefix uniformly.
	r.GET("/repos/:owner/:repo/tarball/:ref", func(c *gin.Context) {
		owner := c.Param("owner")
		repo := c.Param("repo")
		files := s.getAllFiles(owner, repo)

		c.Header("Content-Type", "application/x-gzip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s-main.tar.gz", owner, repo))

		prefix := fmt.Sprintf("%s-%s-main/", owner, repo)
		gw := gzip.NewWriter(c.Writer)
		tw := tar.NewWriter(gw)
		for path, content := range files {
			hdr := &tar.Header{
				Name: prefix + path,
				Mode: 0644,
				Size: int64(len(content)),
			}
			_ = tw.WriteHeader(hdr)
			_, _ = tw.Write([]byte(content))
		}
		_ = tw.Close()
		_ = gw.Close()
	})

	// File content endpoint (GitHub-compatible shape).
	// Returns a single file object for exact path matches, or a directory listing
	// array when the path is a directory prefix (mirrors the real GitHub API).
	r.GET("/repos/:owner/:repo/contents/*path", func(c *gin.Context) {
		owner := c.Param("owner")
		repo := c.Param("repo")
		path := strings.TrimPrefix(c.Param("path"), "/")

		// Exact file lookup.
		if content, ok := s.getFile(owner, repo, path); ok {
			c.JSON(http.StatusOK, gin.H{
				"path":     path,
				"content":  base64.StdEncoding.EncodeToString([]byte(content)),
				"encoding": "base64",
			})
			return
		}

		// Directory listing fallback.
		if entries := s.listDir(owner, repo, path); len(entries) > 0 {
			c.JSON(http.StatusOK, entries)
			return
		}

		c.JSON(http.StatusNotFound, gin.H{
			"message": fmt.Sprintf("path %q not found in %s/%s", path, owner, repo),
		})
	})
}

// sendWebhook POSTs a GitHub-shaped pull_request webhook event to the worker.
func sendWebhook(log *slog.Logger, webhookURL, owner, repo string, pr *PullRequest) {
	payload := map[string]any{
		"action": "closed",
		"pull_request": map[string]any{
			"number":   pr.Number,
			"merged":   true,
			"html_url": pr.HTMLURL,
		},
		"repository": map[string]any{
			"full_name": owner + "/" + repo,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Error("failed to marshal webhook", "error", err)
		return
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		log.Error("failed to build webhook request", "error", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Error("failed to send webhook", "error", err, "url", webhookURL)
		return
	}
	defer func() { //nolint:errcheck // response body close errors are non-actionable after reading
		_ = resp.Body.Close()
	}()

	log.Info("webhook sent", "url", webhookURL, "status", resp.StatusCode)
}

// --- HTML templates ---

func renderDashboard(prs []PullRequest) string {
	rows := ""
	openCount := 0
	var rowsSb345 strings.Builder
	for _, pr := range prs {
		if pr.State == "open" {
			openCount++
		}
		stateColor := "#3fb950"
		stateLabel := "Open"
		if pr.State == stateMerged {
			stateColor = "#a371f7"
			stateLabel = "Merged"
		}
		rowsSb345.WriteString(fmt.Sprintf(`
        <tr>
          <td style="padding:12px 16px;border-bottom:1px solid #21262d;">
            <a href="%s" style="color:#58a6ff;text-decoration:none;font-weight:600;">%s</a>
          </td>
          <td style="padding:12px 16px;border-bottom:1px solid #21262d;font-family:monospace;font-size:13px;color:#8b949e;">%s</td>
          <td style="padding:12px 16px;border-bottom:1px solid #21262d;">
            <span style="display:inline-block;padding:2px 10px;border-radius:12px;font-size:12px;font-weight:500;background:%s22;color:%s;border:1px solid %s44;">%s</span>
          </td>
        </tr>`, pr.HTMLURL, pr.Title, pr.Head, stateColor, stateColor, stateColor, stateLabel))
	}
	rows += rowsSb345.String()

	if len(prs) == 0 {
		rows = `<tr><td colspan="3" style="padding:40px 16px;text-align:center;color:#8b949e;">No pull requests yet. Start a migration run from the Loom console.</td></tr>`
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>Mock GitHub</title>
  <meta http-equiv="refresh" content="3">
  <style>
    * { margin:0; padding:0; box-sizing:border-box; }
    body { background:#0d1117; color:#c9d1d9; font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; }
  </style>
</head>
<body>
  <div style="max-width:860px;margin:0 auto;padding:32px 16px;">
    <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:24px;">
      <h1 style="font-size:20px;font-weight:600;">Pull Requests</h1>
      <span style="font-size:13px;color:#8b949e;">%d open</span>
    </div>
    <table style="width:100%%;border-collapse:collapse;background:#161b22;border:1px solid #30363d;border-radius:6px;overflow:hidden;">
      <thead>
        <tr style="background:#161b22;">
          <th style="padding:12px 16px;text-align:left;font-size:12px;color:#8b949e;border-bottom:1px solid #21262d;font-weight:500;">Title</th>
          <th style="padding:12px 16px;text-align:left;font-size:12px;color:#8b949e;border-bottom:1px solid #21262d;font-weight:500;">Branch</th>
          <th style="padding:12px 16px;text-align:left;font-size:12px;color:#8b949e;border-bottom:1px solid #21262d;font-weight:500;">Status</th>
        </tr>
      </thead>
      <tbody>%s</tbody>
    </table>
  </div>
</body>
</html>`, openCount, rows)
}

func renderPRPage(owner, repo string, pr *PullRequest, files map[string]string) string {
	stateColor := "#3fb950"
	stateLabel := "Open"
	mergeButton := fmt.Sprintf(`
    <form method="POST" action="/%s/%s/pull/%d/merge" style="margin-top:24px;">
      <button type="submit" style="padding:8px 20px;background:#238636;color:#fff;border:1px solid #2ea04366;border-radius:6px;font-size:14px;font-weight:500;cursor:pointer;">
        Merge pull request
      </button>
    </form>`, owner, repo, pr.Number)

	if pr.State == "merged" {
		stateColor = "#a371f7"
		stateLabel = "Merged"
		mergeButton = `<div style="margin-top:24px;padding:12px 16px;background:#a371f722;border:1px solid #a371f744;border-radius:6px;color:#a371f7;font-size:14px;">Pull request successfully merged.</div>`
	}

	// Render file changes section
	filesHTML := ""
	if len(files) > 0 {
		fileEntries := ""
		var fileEntriesSb422 strings.Builder
		for path, content := range files {
			fileEntriesSb422.WriteString(fmt.Sprintf(`
      <div style="margin-bottom:2px;">
        <div style="padding:10px 16px;background:#1c2128;border:1px solid #30363d;border-radius:6px 6px 0 0;font-size:13px;">
          <code style="color:#79c0ff;">%s</code>
        </div>
        <pre style="margin:0;padding:16px;background:#0d1117;border:1px solid #30363d;border-top:none;border-radius:0 0 6px 6px;font-size:12px;color:#8b949e;overflow-x:auto;white-space:pre-wrap;">%s</pre>
      </div>`, path, content))
		}
		fileEntries += fileEntriesSb422.String()
		filesHTML = fmt.Sprintf(`
    <div style="margin-top:24px;">
      <h3 style="font-size:16px;font-weight:500;margin-bottom:12px;color:#c9d1d9;">Files changed (%d)</h3>
      %s
    </div>`, len(files), fileEntries)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>%s - Mock GitHub</title>
  <style>
    * { margin:0; padding:0; box-sizing:border-box; }
    body { background:#0d1117; color:#c9d1d9; font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; }
    a { color:#58a6ff; text-decoration:none; }
    a:hover { text-decoration:underline; }
  </style>
</head>
<body>
  <div style="max-width:860px;margin:0 auto;padding:32px 16px;">
    <div style="margin-bottom:24px;font-size:13px;">
      <a href="/">All pull requests</a>
    </div>

    <div style="display:flex;align-items:flex-start;gap:12px;margin-bottom:16px;">
      <h1 style="font-size:24px;font-weight:400;">
        <span style="color:#c9d1d9;">%s</span>
        <span style="color:#8b949e;font-weight:300;"> #%d</span>
      </h1>
    </div>

    <div style="margin-bottom:24px;">
      <span style="display:inline-block;padding:4px 12px;border-radius:16px;font-size:13px;font-weight:500;background:%s22;color:%s;border:1px solid %s44;">%s</span>
      <span style="margin-left:8px;font-size:13px;color:#8b949e;">%s/%s</span>
    </div>

    <div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:20px;">
      <div style="font-size:14px;color:#c9d1d9;line-height:1.6;">%s</div>
      <div style="margin-top:16px;padding-top:16px;border-top:1px solid #21262d;font-size:13px;color:#8b949e;">
        <code style="background:#1c2128;padding:2px 6px;border-radius:4px;color:#79c0ff;">%s</code>
        &rarr;
        <code style="background:#1c2128;padding:2px 6px;border-radius:4px;color:#79c0ff;">%s</code>
      </div>
    </div>

    %s
    %s
  </div>
</body>
</html>`, pr.Title, pr.Title, pr.Number,
		stateColor, stateColor, stateColor, stateLabel,
		owner, repo,
		pr.Body, pr.Head, pr.Base, mergeButton, filesHTML)
}
