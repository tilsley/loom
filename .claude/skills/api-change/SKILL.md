---
name: api-change
description: Make a change to the Loom API — add a new endpoint or modify an existing one. Use when editing openapi.yaml or adding server endpoints.
disable-model-invocation: true
---

`schemas/openapi.yaml` is the single source of truth for all types. Every API change follows this order — do not skip steps.

## Step 1 — Edit the schema

Edit `schemas/openapi.yaml`:
- Add/modify paths, request/response bodies, or component schemas
- Properties use `camelCase`
- Reuse existing components where possible (`$ref: '#/components/schemas/...'`)

## Step 2 — Regenerate types (mandatory)

```bash
make generate
```

This regenerates both:
- `pkg/api/types.gen.go` — Go types used by the server
- `apps/console/src/lib/api.gen.ts` — TypeScript types used by the console

Never edit these generated files directly. If you see a type mismatch, re-run `make generate`.

## Step 3 — Implement the handler

Add or update the handler in `apps/server/internal/migrations/handler/`:
- `candidates.go` — endpoints scoped to a candidate (`/migrations/:id/candidates/:candidateId/...`)
- `migrations.go` — migration-scoped endpoints (`/migrations/:id/...`)
- `events.go` — event/webhook endpoints

Register any new routes in `routes.go`.

Use `pkg/api` types for all request/response structs — they come from the generated file.

## Step 4 — Update the service layer (if new business logic)

If the endpoint requires new domain behaviour:
1. Add the method to the relevant interface in `internal/migrations/ports.go`
2. Implement it in `apps/server/internal/migrations/store/redis_store.go`
3. Call it from `internal/migrations/service.go`

The layered rule: handlers call the service, the service calls adapters — never skip layers.

## Step 5 — Update the console (if UI needs the new endpoint)

TypeScript types are already refreshed from step 2. Use the HTTP client at `apps/console/src/lib/client.ts` to call new endpoints from console pages in `apps/console/src/app/migrations/`.

## Verify

```bash
make vet
make test
make console-typecheck
```
