package gitrepo

import "context"

// FileContent represents a file retrieved from a git hosting provider.
type FileContent struct {
	Path    string
	Content string // decoded content (not base64)
}

// DirEntry is a file or directory returned by a git hosting provider directory listing.
type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "dir"
}

// PullRequest represents a pull request on a git hosting provider.
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
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Head  string            `json:"head"`
	Base  string            `json:"base"`
	Files map[string]string `json:"files,omitempty"`
}

// Client is the port that steps and discovery depend on to interact with a git repository host.
type Client interface {
	GetContents(ctx context.Context, owner, repo, path string) (*FileContent, error)
	ListDir(ctx context.Context, owner, repo, path string) ([]DirEntry, error)
	CreatePR(ctx context.Context, owner, repo string, req CreatePRRequest) (*PullRequest, error)
}
