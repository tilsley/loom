---
name: test-patterns
description: Go test patterns for the Loom server. Use when writing or reviewing Go tests — provides the in-memory stubs and table-driven structure used throughout the codebase.
---

## Test structure

All server tests live in `_test` packages (black-box testing). The canonical example is `apps/server/internal/migrations/service_test.go`.

Use `github.com/stretchr/testify/assert` and `require` — not the stdlib `t.Error`/`t.Fatal`.

### Table-driven tests

```go
func TestService_Cancel(t *testing.T) {
    t.Run("cancels workflow and resets candidate to not_started", func(t *testing.T) { ... })
    t.Run("workflow not found is tolerated; reset still proceeds", func(t *testing.T) { ... })
    t.Run("other engine error is returned immediately", func(t *testing.T) { ... })
}
```

Name sub-tests as plain English sentences describing the exact behaviour. Include both happy paths and error propagation cases.

### In-memory stubs

Never use a mocking framework. The project uses hand-written stubs defined at the top of the test file.

**`memStore`** — implements `MigrationStore`

```go
store := newMemStore()

// Seed state
_ = store.Save(ctx, api.Migration{
    Id:         "m1",
    Candidates: []api.Candidate{{Id: "repo-a", Status: &running}},
})

// Inject errors
store.errGet = errors.New("connection refused")
store.errSetCandidateStatus = errors.New("write failed")
```

**`stubEngine`** — implements `ExecutionEngine`

```go
engine := &stubEngine{
    startFn: func(ctx context.Context, name, id string, input any) (string, error) {
        return id, nil
    },
    getStatusFn: func(ctx context.Context, id string) (*migrations.RunStatus, error) {
        return &migrations.RunStatus{RuntimeStatus: "RUNNING"}, nil
    },
    cancelFn: func(ctx context.Context, id string) error {
        return migrations.RunNotFoundError{InstanceID: id}
    },
    raiseEventFn: func(ctx context.Context, id, event string, payload any) error {
        return nil
    },
}
```

**`stubDryRunner`** — implements `DryRunner`

```go
dr := &stubDryRunner{
    result: &api.DryRunResult{Steps: []api.StepDryRunResult{{StepName: "step-1"}}},
    err:    errors.New("worker unreachable"),
}
// After call: dr.lastReq and dr.lastWorkerUrl are captured for assertions
```

### Constructor helper

```go
svc := migrations.NewService(engine, store, dr)
// or use the local helper:
svc := newSvc(store, engine, &stubDryRunner{})
```

### Compile-time interface checks

Add at the top of each test file for any stub that implements an interface:

```go
var (
    _ migrations.MigrationStore = (*memStore)(nil)
    _ migrations.ExecutionEngine = (*stubEngine)(nil)
)
```

### Common assertions

```go
require.NoError(t, err)
require.ErrorContains(t, err, "expected substring")

var target migrations.CandidateNotRunningError
require.ErrorAs(t, err, &target)
assert.Equal(t, "repo-a", target.ID)

assert.Equal(t, api.CandidateStatusNotStarted, *m.Candidates[0].Status)
assert.WithinDuration(t, time.Now(), m.CreatedAt, 2*time.Second)
```

### Helper

```go
func ptr[T any](v T) *T { return &v }
```

Useful for creating pointers to literals in test setup.

## Running tests

```bash
make test        # go test ./...
make vet         # go vet ./...
```
