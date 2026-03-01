package migrator_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/migrations/migrator"
	"github.com/tilsley/loom/pkg/api"
)

func newDryRunner() *migrator.HTTPDryRunAdapter {
	return migrator.NewHTTPDryRunAdapter(http.DefaultClient)
}

var baseDryRunReq = api.DryRunRequest{
	MigrationId: "app-chart-migration",
	Candidate:   api.Candidate{Id: "billing-api", Kind: "application"},
	Steps: []api.StepDefinition{
		{Name: "update-chart", MigratorApp: "app-chart-migrator"},
	},
}

// ─── Happy path ───────────────────────────────────────────────────────────────

func TestDryRun_PostsToCorrectEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.DryRunResult{})
	}))
	defer srv.Close()

	d := newDryRunner()
	_, err := d.DryRun(context.Background(), srv.URL, baseDryRunReq)

	require.NoError(t, err)
	assert.Equal(t, "/dry-run", gotPath)
}

func TestDryRun_ReturnsDecodedResult(t *testing.T) {
	expected := api.DryRunResult{
		Steps: []api.StepDryRunResult{
			{StepName: "update-chart"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	d := newDryRunner()
	result, err := d.DryRun(context.Background(), srv.URL, baseDryRunReq)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Steps, 1)
	assert.Equal(t, "update-chart", result.Steps[0].StepName)
}

func TestDryRun_SetsContentTypeJSON(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.DryRunResult{})
	}))
	defer srv.Close()

	d := newDryRunner()
	_, err := d.DryRun(context.Background(), srv.URL, baseDryRunReq)

	require.NoError(t, err)
	assert.Equal(t, "application/json", gotContentType)
}

// ─── Error cases ──────────────────────────────────────────────────────────────

func TestDryRun_EmptyMigratorUrl_ReturnsError(t *testing.T) {
	d := newDryRunner()

	result, err := d.DryRun(context.Background(), "", baseDryRunReq)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), baseDryRunReq.MigrationId)
}

func TestDryRun_Non200Response_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	d := newDryRunner()
	result, err := d.DryRun(context.Background(), srv.URL, baseDryRunReq)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "422")
}

func TestDryRun_InvalidResponseBody_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	d := newDryRunner()
	result, err := d.DryRun(context.Background(), srv.URL, baseDryRunReq)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestDryRun_ConnectionRefused_ReturnsError(t *testing.T) {
	d := newDryRunner()

	result, err := d.DryRun(context.Background(), "http://127.0.0.1:1", baseDryRunReq)

	assert.Error(t, err)
	assert.Nil(t, result)
}
