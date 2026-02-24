# ADR-0002: Replace Dapr with direct HTTP and go-redis

**Status:** Proposed

---

## Context

Dapr currently sits between the server and its infrastructure dependencies for three purposes:

1. **Pub/sub** — server dispatches steps to workers via a Redis-backed topic
2. **State store** — server reads/writes migration and candidate state to Redis
3. **Service invocation** — server calls workers synchronously for dry-run

The full analysis is in `docs/architecture-review.md`. In summary: the abstraction benefit Dapr promises already exists in the Go port interfaces (`WorkerNotifier`, `DryRunner`, `MigrationStore`). Dapr duplicates an abstraction that's already there, at significant operational cost.

---

## Decision

Remove Dapr from the server and all worker apps. Replace each concern directly:

**State store → go-redis**
Swap `DaprMigrationStore` for a Redis adapter that uses `go-redis` directly. The `MigrationStore` interface is unchanged.

**Step dispatch → direct HTTP**
Workers register their base URL in the `MigrationAnnouncement` payload (new `workerUrl` field). The server maintains an in-memory `workerApp → URL` map, populated as workers announce. For each step dispatch the server makes a plain `net/http` POST to the stored URL. Swap `DaprBus` for an HTTP adapter implementing the `WorkerNotifier` interface.

**Dry-run → direct HTTP**
Same `workerUrl` map. Swap `DaprDryRunAdapter` for an HTTP adapter implementing `DryRunner`. This is functionally identical to what Dapr service invocation does — it was already a direct HTTP call with a proxy in the middle.

**Worker → server communication is unchanged.** The announcement (`POST /registry/announce`) and step completion callback (`POST /event/{id}`) are already direct HTTP and remain so.

---

## Consequences

**Positive**

- Local dev stack drops from 5+ services (Temporal, PostgreSQL, Redis, Dapr placement, sidecars per app) to 3 (Temporal, PostgreSQL, Redis)
- No sidecar containers in production deployments
- `dapr run` wrappers removed from all Makefile targets
- Dapr component YAML files deleted
- Request path for step dispatch goes from four hops (server → sidecar → Redis → sidecar → worker) to one (server → worker)
- go-redis is a simpler, better-understood dependency than the Dapr SDK
- Port interfaces (`WorkerNotifier`, `DryRunner`, `MigrationStore`) are unchanged — only the adapter implementations change

**Negative / trade-offs**

- Workers must now include `workerUrl` in their announcement. This is a one-line addition to the announcement handler and a small schema change.
- The server needs to be able to reach workers over HTTP. This was already true: dry-run uses Dapr service invocation, which is HTTP under the hood. Workers were already required to be network-reachable. No new network requirement is introduced.
- If the server restarts before a worker re-announces, the `workerApp → URL` map is empty and dispatch will fail until workers reconnect. This is the same behaviour as today — if Dapr's placement service restarts, pub/sub is also broken. Workers should announce with a short retry loop on startup regardless.

**No effect on rollback capability.** Rollbacks are managed by Temporal workflow activities, not by Dapr. See `docs/architecture-review.md`.

---

## Migration path

1. Add `workerUrl` to `MigrationAnnouncement` in `schemas/openapi.yaml`, regenerate types
2. Implement `RedisMigrationStore` using `go-redis` — replace `DaprMigrationStore`
3. Implement `HTTPWorkerNotifier` — replace `DaprBus`
4. Implement `HTTPDryRunAdapter` — replace `DaprDryRunAdapter`
5. Update `apps/server/main.go` to wire the new adapters; remove Dapr client initialisation
6. Update `apps/migrators/app-chart-migrator/main.go` to include `workerUrl` in announcement; remove Dapr client
7. Remove `dapr/components/` directories from server and migrator
8. Update `Makefile` — replace `dapr run ...` with plain `go run .`
9. Remove `daprio/dapr` placement from `docker-compose.yaml`
10. Remove Dapr SDK from `go.mod`
