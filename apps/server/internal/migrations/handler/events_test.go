package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/pkg/api"
)

// ─── POST /event/:id ─────────────────────────────────────────────────────────

func TestEvent_RaisesSignal(t *testing.T) {
	ts := newTestServer(t)
	var raised string
	ts.engine.raiseEventFn = func(_ context.Context, _, event string, _ any) error {
		raised = event
		return nil
	}

	w := ts.do(http.MethodPost, "/event/run-123", api.StepStatusEvent{
		StepName:    "update-chart",
		CandidateId: "billing-api",
		Status:      api.StepStatusEventStatusSucceeded,
	})

	require.Equal(t, http.StatusAccepted, w.Code)
	assert.NotEmpty(t, raised)
}

func TestEvent_BadJSON_Returns400(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/event/run-123",
		bytes.NewBufferString(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── POST /registry/announce ─────────────────────────────────────────────────

func TestAnnounce_DirectJSON(t *testing.T) {
	ts := newTestServer(t)

	ann := api.MigrationAnnouncement{
		Id:          "migrate-chart",
		Name:        "Migrate chart",
		Description: "Upgrades Helm charts",
		Steps:       []api.StepDefinition{{Name: "update-chart", MigratorApp: "app-chart-migrator"}},
		MigratorUrl: "http://app-chart-migrator:3001",
	}

	w := ts.do(http.MethodPost, "/registry/announce", ann)

	require.Equal(t, http.StatusOK, w.Code)
	m, err := ts.store.Get(context.Background(), "migrate-chart")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "Migrate chart", m.Name)
}
