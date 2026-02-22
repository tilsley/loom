import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  ConflictError,
  NotFoundError,
  deleteMigration,
  dequeueRun,
  dryRun,
  executeRun,
  getMigration,
  getRunInfo,
  getStatus,
  listMigrations,
  queueRun,
  registerMigration,
} from "../api";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

/**
 * Build a minimal Response-like object that satisfies the fetch API surface
 * used by api.ts (ok, status, json(), text()).
 */
function mockResponse(status: number, body: unknown) {
  const ok = status >= 200 && status < 300;
  const textBody =
    typeof body === "string" ? body : JSON.stringify(body);
  return {
    ok,
    status,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(textBody),
  };
}

beforeEach(() => {
  mockFetch.mockReset();
});

// ---------------------------------------------------------------------------
// Error classes
// ---------------------------------------------------------------------------

describe("ConflictError", () => {
  it("is instanceof both ConflictError and Error", () => {
    const err = new ConflictError("conflict");
    expect(err).toBeInstanceOf(ConflictError);
    expect(err).toBeInstanceOf(Error);
  });

  it("has name ConflictError", () => {
    expect(new ConflictError("x").name).toBe("ConflictError");
  });
});

describe("NotFoundError", () => {
  it("is instanceof both NotFoundError and Error", () => {
    const err = new NotFoundError("not found");
    expect(err).toBeInstanceOf(NotFoundError);
    expect(err).toBeInstanceOf(Error);
  });

  it("has name NotFoundError", () => {
    expect(new NotFoundError("x").name).toBe("NotFoundError");
  });
});

// ---------------------------------------------------------------------------
// listMigrations
// ---------------------------------------------------------------------------

describe("listMigrations", () => {
  it("calls GET /api/migrations", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, { migrations: [] }));
    await listMigrations();
    expect(mockFetch).toHaveBeenCalledWith("/api/migrations");
  });

  it("returns parsed response on success", async () => {
    const data = { migrations: [{ id: "abc" }] };
    mockFetch.mockResolvedValueOnce(mockResponse(200, data));
    await expect(listMigrations()).resolves.toEqual(data);
  });

  it("throws with the response body text on error", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "server error"));
    await expect(listMigrations()).rejects.toThrow("server error");
  });
});

// ---------------------------------------------------------------------------
// getMigration
// ---------------------------------------------------------------------------

describe("getMigration", () => {
  it("interpolates the id into the URL", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, {}));
    await getMigration("test-id");
    expect(mockFetch).toHaveBeenCalledWith("/api/migrations/test-id");
  });

  it("returns the migration on success", async () => {
    const migration = { id: "test-id", name: "My Migration" };
    mockFetch.mockResolvedValueOnce(mockResponse(200, migration));
    await expect(getMigration("test-id")).resolves.toEqual(migration);
  });

  it("throws with the response body text on error", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(404, "not found"));
    await expect(getMigration("bad-id")).rejects.toThrow("not found");
  });
});

// ---------------------------------------------------------------------------
// getStatus  (custom error-handling logic)
// ---------------------------------------------------------------------------

describe("getStatus", () => {
  it("returns parsed response on success", async () => {
    const data = { status: "running" };
    mockFetch.mockResolvedValueOnce(mockResponse(200, data));
    await expect(getStatus("wf-id")).resolves.toEqual(data);
  });

  it("throws NotFoundError on 404", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(404, "not found"));
    await expect(getStatus("bad-id")).rejects.toBeInstanceOf(NotFoundError);
  });

  it('throws NotFoundError when body contains "not found" regardless of status code', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(400, "workflow not found"));
    await expect(getStatus("id")).rejects.toBeInstanceOf(NotFoundError);
  });

  it("extracts the error field from a JSON error body", async () => {
    mockFetch.mockResolvedValueOnce(
      mockResponse(500, JSON.stringify({ error: "something failed" })),
    );
    await expect(getStatus("id")).rejects.toThrow("something failed");
  });

  it("throws a plain Error (not NotFoundError) for unrecognised failures", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "internal error"));
    const err = await getStatus("id").catch((e) => e);
    expect(err).toBeInstanceOf(Error);
    expect(err).not.toBeInstanceOf(NotFoundError);
  });
});

// ---------------------------------------------------------------------------
// getRunInfo
// ---------------------------------------------------------------------------

describe("getRunInfo", () => {
  it("returns parsed response on 200", async () => {
    const runInfo = { id: "run-1", status: "running" };
    mockFetch.mockResolvedValueOnce(mockResponse(200, runInfo));
    await expect(getRunInfo("run-1")).resolves.toEqual(runInfo);
  });

  it("returns null on 404 instead of throwing", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(404, "not found"));
    await expect(getRunInfo("missing")).resolves.toBeNull();
  });

  it("throws with the response body text on other errors", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "server error"));
    await expect(getRunInfo("id")).rejects.toThrow("server error");
  });
});

// ---------------------------------------------------------------------------
// queueRun
// ---------------------------------------------------------------------------

describe("queueRun", () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const candidate = { id: "cand-1", repo: "org/repo", ref: "main" } as any;

  it("returns the response on success", async () => {
    const data = { runId: "run-123" };
    mockFetch.mockResolvedValueOnce(mockResponse(200, data));
    await expect(queueRun("migration-id", candidate)).resolves.toEqual(data);
  });

  it("throws ConflictError on 409", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(409, "already queued"));
    await expect(queueRun("migration-id", candidate)).rejects.toBeInstanceOf(
      ConflictError,
    );
  });

  it("throws with the response body text on other errors", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "server error"));
    await expect(queueRun("migration-id", candidate)).rejects.toThrow(
      "server error",
    );
  });

  it("omits inputs key when inputs is undefined", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, {}));
    await queueRun("migration-id", candidate);
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body).not.toHaveProperty("inputs");
  });

  it("omits inputs key when inputs is an empty object", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, {}));
    await queueRun("migration-id", candidate, {});
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body).not.toHaveProperty("inputs");
  });

  it("includes inputs in the body when non-empty inputs are provided", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, {}));
    await queueRun("migration-id", candidate, { token: "abc", env: "prod" });
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.inputs).toEqual({ token: "abc", env: "prod" });
  });
});

// ---------------------------------------------------------------------------
// executeRun
// ---------------------------------------------------------------------------

describe("executeRun", () => {
  it("returns the response on success", async () => {
    const data = { runId: "run-123" };
    mockFetch.mockResolvedValueOnce(mockResponse(200, data));
    await expect(executeRun("run-123")).resolves.toEqual(data);
  });

  it("throws ConflictError on 409", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(409, "conflict"));
    await expect(executeRun("run-123")).rejects.toBeInstanceOf(ConflictError);
  });

  it("throws with the response body text on other errors", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "server error"));
    await expect(executeRun("run-123")).rejects.toThrow("server error");
  });
});

// ---------------------------------------------------------------------------
// dequeueRun
// ---------------------------------------------------------------------------

describe("dequeueRun", () => {
  it("sends DELETE to /api/runs/{runId}/dequeue", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, {}));
    await dequeueRun("run-abc");
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/runs/run-abc/dequeue",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("resolves to undefined on success", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, {}));
    await expect(dequeueRun("run-123")).resolves.toBeUndefined();
  });

  it("throws ConflictError on 409", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(409, "conflict"));
    await expect(dequeueRun("run-123")).rejects.toBeInstanceOf(ConflictError);
  });

  it("throws with the response body text on other errors", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "server error"));
    await expect(dequeueRun("run-123")).rejects.toThrow("server error");
  });
});

// ---------------------------------------------------------------------------
// registerMigration
// ---------------------------------------------------------------------------

describe("registerMigration", () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const req = { name: "My Migration" } as any;

  it("sends POST to /api/migrations", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, { id: "new-migration", name: "My Migration", description: "", requiredInputs: [], candidates: [], steps: [], createdAt: new Date().toISOString(), cancelledAt: null }));
    await registerMigration(req);
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/migrations",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("returns the new migration on success", async () => {
    const migration = { id: "new-migration", name: "My Migration" };
    mockFetch.mockResolvedValueOnce(mockResponse(200, migration));
    await expect(registerMigration(req)).resolves.toEqual(migration);
  });

  it("throws with the response body text on error", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(400, "bad request"));
    await expect(registerMigration(req)).rejects.toThrow("bad request");
  });
});

// ---------------------------------------------------------------------------
// deleteMigration
// ---------------------------------------------------------------------------

describe("deleteMigration", () => {
  it("sends DELETE to /api/migrations/{id}", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, {}));
    await deleteMigration("my-id");
    expect(mockFetch).toHaveBeenCalledWith("/api/migrations/my-id", {
      method: "DELETE",
    });
  });

  it("resolves to undefined on success", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, {}));
    await expect(deleteMigration("id")).resolves.toBeUndefined();
  });

  it("throws with the response body text on error", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "server error"));
    await expect(deleteMigration("id")).rejects.toThrow("server error");
  });
});

// ---------------------------------------------------------------------------
// dryRun
// ---------------------------------------------------------------------------

describe("dryRun", () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const candidate = { id: "cand-1", repo: "org/repo", ref: "main" } as any;

  it("returns the result on success", async () => {
    const result = { steps: [] };
    mockFetch.mockResolvedValueOnce(mockResponse(200, result));
    await expect(dryRun("migration-id", candidate)).resolves.toEqual(result);
  });

  it("URL-encodes the migration ID", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(200, {}));
    await dryRun("my migration/id", candidate);
    expect(mockFetch.mock.calls[0][0]).toBe(
      "/api/migrations/my%20migration%2Fid/dry-run",
    );
  });

  it("throws with the response body text on error", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "server error"));
    await expect(dryRun("id", candidate)).rejects.toThrow("server error");
  });
});
