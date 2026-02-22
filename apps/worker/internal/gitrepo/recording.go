package gitrepo

import "context"

// RecordingClient wraps a real Client, capturing GetContents responses and
// faking CreatePR without making network calls. Used during dry-run execution.
//
// overlay holds accumulated file content written by previous steps. GetContents
// checks the overlay first so each step sees the world as it would exist after
// all earlier PRs have been merged (stacked diffs).
type RecordingClient struct {
	real     Client
	overlay  map[string]string // "owner/repo/path" → content written by earlier steps
	contents map[string]string // "owner/repo/path" → content seen by this step (for diff "before")
}

// NewRecordingClient creates a RecordingClient backed by the given real client.
// overlay is the accumulated output of all previous steps; pass nil for the first step.
func NewRecordingClient(real Client, overlay map[string]string) *RecordingClient {
	if overlay == nil {
		overlay = make(map[string]string)
	}
	return &RecordingClient{real: real, overlay: overlay, contents: make(map[string]string)}
}

// GetContents checks the overlay (accumulated prior-step output) before calling
// the real client. Whatever is returned is recorded so ContentBefore can later
// produce the correct "before" side of the diff for this step.
func (r *RecordingClient) GetContents(ctx context.Context, owner, repo, path string) (*FileContent, error) {
	key := owner + "/" + repo + "/" + path

	if content, ok := r.overlay[key]; ok {
		r.contents[key] = content
		return &FileContent{Path: path, Content: content}, nil
	}

	fc, err := r.real.GetContents(ctx, owner, repo, path)
	if err != nil {
		return nil, err
	}
	r.contents[key] = fc.Content
	return fc, nil
}

// ListDir forwards to the real client unchanged.
func (r *RecordingClient) ListDir(ctx context.Context, owner, repo, path string) ([]DirEntry, error) {
	return r.real.ListDir(ctx, owner, repo, path)
}

// CreatePR returns a fake pull request without making any network call.
// The files the step would have committed are available via the CreatePRRequest
// passed by the step handler — the dryrun.Runner captures them from the Result.
func (r *RecordingClient) CreatePR(_ context.Context, owner, repo string, req CreatePRRequest) (*PullRequest, error) {
	return &PullRequest{
		Number:  1,
		HTMLURL: "https://github.com/" + owner + "/" + repo + "/pull/1",
		Title:   req.Title,
		Body:    req.Body,
		Head:    req.Head,
		Base:    req.Base,
		State:   "open",
	}, nil
}

// ContentBefore returns the content of a file as it existed before the step ran.
// Returns an empty string for files that were not previously fetched (new files).
func (r *RecordingClient) ContentBefore(owner, repo, path string) string {
	return r.contents[owner+"/"+repo+"/"+path]
}
