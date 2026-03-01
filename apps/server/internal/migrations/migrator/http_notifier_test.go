package migrator_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/migrations/migrator"
	"github.com/tilsley/loom/pkg/api"
)

func newNotifier() *migrator.HTTPMigratorNotifier {
	return migrator.NewHTTPMigratorNotifier(http.DefaultClient)
}

var baseDispatchReq = api.DispatchStepRequest{
	MigrationId: "app-chart-migration",
	MigratorApp: "app-chart-migrator",
	StepName:    "update-chart",
	Candidate:   api.Candidate{Id: "billing-api", Kind: "application"},
	// MigratorUrl is set per-test
}

// ─── Happy path ───────────────────────────────────────────────────────────────

func TestDispatch_PostsToCorrectEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := newNotifier()
	req := baseDispatchReq
	req.MigratorUrl = srv.URL

	require.NoError(t, n.Dispatch(context.Background(), req))
	assert.Equal(t, "/dispatch-step", gotPath)
}

func TestDispatch_SendsCorrectBody(t *testing.T) {
	var received api.DispatchStepRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := newNotifier()
	req := baseDispatchReq
	req.MigratorUrl = srv.URL

	require.NoError(t, n.Dispatch(context.Background(), req))
	assert.Equal(t, "update-chart", received.StepName)
	assert.Equal(t, "billing-api", received.Candidate.Id)
}

func TestDispatch_SetsContentTypeJSON(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := newNotifier()
	req := baseDispatchReq
	req.MigratorUrl = srv.URL

	require.NoError(t, n.Dispatch(context.Background(), req))
	assert.Equal(t, "application/json", gotContentType)
}

// ─── Error cases ──────────────────────────────────────────────────────────────

func TestDispatch_EmptyMigratorUrl_ReturnsError(t *testing.T) {
	n := newNotifier()
	req := baseDispatchReq
	req.MigratorUrl = ""

	err := n.Dispatch(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), req.StepName)
}

func TestDispatch_Non2xxResponse_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := newNotifier()
	req := baseDispatchReq
	req.MigratorUrl = srv.URL

	err := n.Dispatch(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestDispatch_ConnectionRefused_ReturnsError(t *testing.T) {
	n := newNotifier()
	req := baseDispatchReq
	req.MigratorUrl = "http://127.0.0.1:1" // nothing listening

	err := n.Dispatch(context.Background(), req)

	assert.Error(t, err)
}
