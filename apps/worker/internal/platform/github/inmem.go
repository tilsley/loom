package github

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
)

// InMem is an in-memory gitrepo.Client for unit tests.
type InMem struct {
	mu    sync.Mutex
	files map[string]string // "owner/repo/path" -> content
	prs   []gitrepo.PullRequest
	nextN int
}

// NewInMem creates an empty InMem client.
func NewInMem() *InMem {
	return &InMem{
		files: make(map[string]string),
		nextN: 1,
	}
}

// SetFile seeds a file in the in-memory store.
func (m *InMem) SetFile(owner, repo, path, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[owner+"/"+repo+"/"+path] = content
}

// PRs returns all pull requests created via CreatePR.
func (m *InMem) PRs() []gitrepo.PullRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]gitrepo.PullRequest, len(m.prs))
	copy(out, m.prs)
	return out
}

// GetContents returns the file at owner/repo/path, or an error if not found.
func (m *InMem) GetContents(_ context.Context, owner, repo, path string) (*gitrepo.FileContent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + repo + "/" + path
	content, ok := m.files[key]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", key)
	}
	return &gitrepo.FileContent{Path: path, Content: content}, nil
}

// ListDir returns the immediate children of dirPath.
func (m *InMem) ListDir(_ context.Context, owner, repo, dirPath string) ([]gitrepo.DirEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := owner + "/" + repo + "/" + dirPath + "/"
	seen := make(map[string]bool)
	var entries []gitrepo.DirEntry
	for key := range m.files {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		rest := key[len(prefix):]
		parts := strings.SplitN(rest, "/", 2)
		name := parts[0]
		if seen[name] {
			continue
		}
		seen[name] = true
		entType := "file"
		if len(parts) > 1 {
			entType = "dir"
		}
		entries = append(entries, gitrepo.DirEntry{
			Name: name,
			Path: dirPath + "/" + name,
			Type: entType,
		})
	}
	return entries, nil
}

// CreatePR records a PR and writes its files into the in-memory store.
func (m *InMem) CreatePR(_ context.Context, owner, repo string, req gitrepo.CreatePRRequest) (*gitrepo.PullRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for path, content := range req.Files {
		m.files[owner+"/"+repo+"/"+path] = content
	}
	pr := gitrepo.PullRequest{
		Number:  m.nextN,
		HTMLURL: fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, m.nextN),
		Title:   req.Title,
		Body:    req.Body,
		Head:    req.Head,
		Base:    req.Base,
		State:   "open",
	}
	m.prs = append(m.prs, pr)
	m.nextN++
	return &pr, nil
}
