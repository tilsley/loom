// Package github implements the gitrepo.Client and gitrepo.RepoReader ports
// using the official go-github library. Wire it up with an authenticated
// *github.Client from apps/worker/internal/platform/github.
package github

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	gogithub "github.com/google/go-github/v75/github"

	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/gitrepo"
)

// Adapter wraps a go-github client and implements gitrepo.Client and
// gitrepo.RepoReader. A single instance covers both step execution (per-file
// reads and PR creation) and discovery (bulk repo snapshot).
type Adapter struct {
	gh *gogithub.Client
}

// New creates an Adapter from an authenticated *github.Client.
func New(gh *gogithub.Client) *Adapter {
	return &Adapter{gh: gh}
}

// GetContents fetches a single file and returns its decoded content.
func (a *Adapter) GetContents(ctx context.Context, owner, repo, path string) (*gitrepo.FileContent, error) {
	fc, _, _, err := a.gh.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get contents %s/%s/%s: %w", owner, repo, path, err)
	}
	if fc == nil {
		return nil, fmt.Errorf("path %s is a directory, not a file", path)
	}
	content, err := fc.GetContent()
	if err != nil {
		return nil, fmt.Errorf("decode content %s: %w", path, err)
	}
	return &gitrepo.FileContent{Path: path, Content: content}, nil
}

// CreatePR creates a branch, commits the provided files onto it, and opens a
// pull request. This is the full Git Data API flow required by real GitHub.
func (a *Adapter) CreatePR(ctx context.Context, owner, repo string, req gitrepo.CreatePRRequest) (*gitrepo.PullRequest, error) {
	// 1. Resolve base branch HEAD SHA.
	baseRef, _, err := a.gh.Git.GetRef(ctx, owner, repo, "refs/heads/"+req.Base)
	if err != nil {
		return nil, fmt.Errorf("get base ref %s: %w", req.Base, err)
	}
	baseSHA := baseRef.Object.GetSHA()

	// 2. Build tree entries — one blob per file.
	treeEntries := make([]*gogithub.TreeEntry, 0, len(req.Files))
	for path, content := range req.Files {
		blob, _, err := a.gh.Git.CreateBlob(ctx, owner, repo, gogithub.Blob{
			Content:  gogithub.Ptr(content),
			Encoding: gogithub.Ptr("utf-8"),
		})
		if err != nil {
			return nil, fmt.Errorf("create blob %s: %w", path, err)
		}
		treeEntries = append(treeEntries, &gogithub.TreeEntry{
			Path: gogithub.Ptr(path),
			Mode: gogithub.Ptr("100644"),
			Type: gogithub.Ptr("blob"),
			SHA:  blob.SHA,
		})
	}

	// 3. Create tree on top of the base tree.
	tree, _, err := a.gh.Git.CreateTree(ctx, owner, repo, baseSHA, treeEntries)
	if err != nil {
		return nil, fmt.Errorf("create tree: %w", err)
	}

	// 4. Create commit.
	commit, _, err := a.gh.Git.CreateCommit(ctx, owner, repo, gogithub.Commit{
		Message: gogithub.Ptr(req.Title),
		Tree:    &gogithub.Tree{SHA: tree.SHA},
		Parents: []*gogithub.Commit{{SHA: gogithub.Ptr(baseSHA)}},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("create commit: %w", err)
	}

	// 5. Create branch pointing at the new commit.
	_, _, err = a.gh.Git.CreateRef(ctx, owner, repo, gogithub.CreateRef{
		Ref: "refs/heads/" + req.Head,
		SHA: commit.GetSHA(),
	})
	if err != nil {
		return nil, fmt.Errorf("create branch %s: %w", req.Head, err)
	}

	// 6. Open the pull request.
	pr, _, err := a.gh.PullRequests.Create(ctx, owner, repo, &gogithub.NewPullRequest{
		Title: gogithub.Ptr(req.Title),
		Body:  gogithub.Ptr(req.Body),
		Head:  gogithub.Ptr(req.Head),
		Base:  gogithub.Ptr(req.Base),
	})
	if err != nil {
		return nil, fmt.Errorf("create pull request: %w", err)
	}

	return &gitrepo.PullRequest{
		Number:  pr.GetNumber(),
		HTMLURL: pr.GetHTMLURL(),
	}, nil
}

// ReadAll downloads the repository tarball in one shot and returns every file
// path mapped to its content. Used by discovery to avoid N+1 API calls.
//
// For real GitHub the API returns a 302 redirect to a CDN pre-signed URL;
// the underlying http.Client follows it automatically and strips the
// Authorization header on cross-host redirects (safe). For mock servers that
// return 200 directly, it works the same way.
func (a *Adapter) ReadAll(ctx context.Context, owner, repo string) (map[string]string, error) {
	tarballURL := a.gh.BaseURL.JoinPath("repos", owner, repo, "tarball", "main")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tarballURL.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build tarball request: %w", err)
	}

	// Use the go-github client's underlying http.Client so the request carries
	// the configured auth transport (token or GitHub App).
	resp, err := a.gh.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET tarball: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // non-actionable after reading

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d", tarballURL, resp.StatusCode)
	}

	return extractTarball(resp.Body)
}

// extractTarball reads a gzip-compressed tar archive and returns every regular
// file mapped path → content. The first path segment (top-level directory) is
// stripped because GitHub wraps the tree in a directory named
// "{owner}-{repo}-{sha}/".
func extractTarball(r io.Reader) (map[string]string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close() //nolint:errcheck // close errors on readers are non-actionable

	files := make(map[string]string)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar next: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		// Strip the top-level wrapper directory, e.g. "owner-repo-sha/".
		path := hdr.Name
		if idx := strings.Index(path, "/"); idx != -1 {
			path = path[idx+1:]
		}
		if path == "" {
			continue
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", hdr.Name, err)
		}
		files[path] = string(content)
	}
	return files, nil
}
