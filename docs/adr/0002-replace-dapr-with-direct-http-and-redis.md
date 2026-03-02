# ADR-0002: Replace Dapr with direct HTTP and Redis

**Status:** Accepted (implemented). The go-redis state store was subsequently replaced with PostgreSQL — see the current `store/` package.

---

## Context

Dapr sat between the server and its infrastructure dependencies for three purposes:

1. **Pub/sub** — server dispatched steps to migrators via a Redis-backed topic
2. **State store** — server read/wrote migration and candidate state to Redis
3. **Service invocation** — server called migrators synchronously for dry-run

The abstraction benefit Dapr promised already existed in the Go port interfaces (`MigratorNotifier`, `DryRunner`, `MigrationStore`). Dapr duplicated an abstraction that was already there, at significant operational cost.

---

## Decision

Remove Dapr from the server and all migrator apps. Replace each concern directly:

**State store → go-redis**
Swap `DaprMigrationStore` for a Redis adapter that uses `go-redis` directly. The `MigrationStore` interface is unchanged.

> **Note:** This was later replaced again with PostgreSQL (`PGMigrationStore`). The interface remained unchanged.

**Step dispatch → direct HTTP**
Migrators register their base URL in the `MigrationAnnouncement` payload (`migratorUrl` field). The URL is stored as part of the Migration and threaded through to every `DispatchStepRequest` — no separate in-memory registry is needed. For each step dispatch the server makes a plain `net/http` POST to that URL. Swap `DaprBus` for `HTTPMigratorNotifier` implementing the `MigratorNotifier` interface.

**Dry-run → direct HTTP**
Same `migratorUrl`. Swap `DaprDryRunAdapter` for an HTTP adapter implementing `DryRunner`. This is functionally identical to what Dapr service invocation did — it was already a direct HTTP call with a proxy in the middle.

**Migrator → server communication was unchanged.** The announcement (`POST /registry/announce`) and step completion callback (`POST /event/{id}`) were already direct HTTP and remained so.

---

## Consequences

**Positive**

- Local dev stack dropped from 5+ services (Temporal, PostgreSQL, Redis, Dapr placement, sidecars per app) to 3 (Temporal, PostgreSQL, Redis)
- No sidecar containers in production deployments
- `dapr run` wrappers removed from all Makefile targets
- Dapr component YAML files deleted
- Request path for step dispatch went from four hops (server → sidecar → Redis → sidecar → migrator) to one (server → migrator)
- go-redis was a simpler, better-understood dependency than the Dapr SDK
- Port interfaces (`MigratorNotifier`, `DryRunner`, `MigrationStore`) were unchanged — only the adapter implementations changed

**Negative / trade-offs**

- Migrators must include `migratorUrl` in their announcement. This was a one-line addition to the announcement handler and a small schema change.
- The server needs to be able to reach migrators over HTTP. This was already true: dry-run used Dapr service invocation, which was HTTP under the hood. Migrators were already required to be network-reachable. No new network requirement was introduced.
- If a migrator re-announces with a changed URL, the server picks it up immediately on the next dispatch (URL is read from the stored Migration). No in-memory state to go stale.

---

## Migration path (completed)

1. Add `migratorUrl` to `MigrationAnnouncement` in `schemas/openapi.yaml`, regenerate types
2. Implement `RedisMigrationStore` using `go-redis` — replace `DaprMigrationStore`
3. Implement `HTTPMigratorNotifier` — replace `DaprBus`
4. Implement `HTTPDryRunAdapter` — replace `DaprDryRunAdapter`
5. Update `apps/server/main.go` to wire the new adapters; remove Dapr client initialisation
6. Update `apps/migrators/app-chart-migrator/main.go` to include `migratorUrl` in announcement; remove Dapr client
7. Remove `dapr/components/` directories from server and migrator
8. Update `Makefile` — replace `dapr run ...` with plain `go run .`
9. Remove `daprio/dapr` placement from `docker-compose.yaml`
10. Remove Dapr SDK from `go.mod`
