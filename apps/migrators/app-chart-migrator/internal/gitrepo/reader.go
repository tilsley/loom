package gitrepo

import "context"

// RepoReader can fetch the full content of a repository in a single operation.
// It is the port used by discovery — downloading everything at once is far more
// efficient than walking the tree file-by-file via the Contents API.
type RepoReader interface {
	ReadAll(ctx context.Context, owner, repo string) (map[string]string, error)
}
