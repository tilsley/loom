package validation_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/platform/validation"
	"github.com/tilsley/loom/schemas"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newRouter(t *testing.T) *gin.Engine {
	t.Helper()
	mw, err := validation.New(schemas.OpenAPISpec)
	require.NoError(t, err)

	r := gin.New()
	r.Use(mw)
	// Register a catch-all so Gin doesn't 404 before the middleware runs.
	r.NoRoute(func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/migrations/:id/candidates", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.POST("/event/:id", func(c *gin.Context) { c.Status(http.StatusAccepted) })
	r.POST("/registry/announce", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ─── submitCandidates ────────────────────────────────────────────────────────

func TestSubmitCandidates_MissingKind_Returns400(t *testing.T) {
	r := newRouter(t)
	w := do(r, http.MethodPost, "/migrations/my-migration/candidates",
		`{"candidates":[{"id":"billing-api"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestSubmitCandidates_MissingCandidates_Returns400(t *testing.T) {
	r := newRouter(t)
	w := do(r, http.MethodPost, "/migrations/my-migration/candidates", `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubmitCandidates_ValidPayload_Passes(t *testing.T) {
	r := newRouter(t)
	w := do(r, http.MethodPost, "/migrations/my-migration/candidates",
		`{"candidates":[{"id":"billing-api","kind":"application","status":"not_started"}]}`)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestSubmitCandidates_MissingStatus_Returns400(t *testing.T) {
	r := newRouter(t)
	w := do(r, http.MethodPost, "/migrations/my-migration/candidates",
		`{"candidates":[{"id":"billing-api","kind":"application"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubmitCandidates_EmptyCandidatesArray_Passes(t *testing.T) {
	r := newRouter(t)
	w := do(r, http.MethodPost, "/migrations/my-migration/candidates",
		`{"candidates":[]}`)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// ─── raiseEvent ──────────────────────────────────────────────────────────────

func TestRaiseEvent_MissingRequiredFields_Returns400(t *testing.T) {
	r := newRouter(t)
	// stepName, candidateId, and status are all required
	w := do(r, http.MethodPost, "/event/wf-123", `{"stepName":"deploy"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRaiseEvent_ValidPayload_Passes(t *testing.T) {
	r := newRouter(t)
	w := do(r, http.MethodPost, "/event/wf-123",
		`{"stepName":"deploy","candidateId":"billing-api","status":"pending"}`)
	assert.Equal(t, http.StatusAccepted, w.Code)
}

// ─── unknown routes pass through ─────────────────────────────────────────────

func TestUnknownRoute_PassesThrough(t *testing.T) {
	r := newRouter(t)
	// /registry/announce is not in the OpenAPI spec — should pass through silently
	w := do(r, http.MethodPost, "/registry/announce", `{}`)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── New() with invalid spec ──────────────────────────────────────────────────

func TestNew_InvalidSpec_ReturnsError(t *testing.T) {
	_, err := validation.New([]byte(`not yaml`))
	assert.Error(t, err)
}
