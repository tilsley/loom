import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  ConflictError,
  NotFoundError,
  dryRun,
  startRun,
  getMigration,
  listMigrations,
  getCandidateSteps,
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
// getCandidateSteps
// ---------------------------------------------------------------------------

describe("getCandidateSteps", () => {
  it("calls the correct URL", async () => {
    const data = { status: "running", steps: [] };
    mockFetch.mockResolvedValueOnce(mockResponse(200, data));
    await getCandidateSteps("mig-1", "billing-api");
    expect(mockFetch).toHaveBeenCalledWith("/api/migrations/mig-1/candidates/billing-api/steps");
  });

  it("returns parsed response on 200", async () => {
    const data = { status: "completed", steps: [{ stepName: "update-chart" }] };
    mockFetch.mockResolvedValueOnce(mockResponse(200, data));
    await expect(getCandidateSteps("mig-1", "billing-api")).resolves.toEqual(data);
  });

  it("returns null on 404 instead of throwing", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(404, "not found"));
    await expect(getCandidateSteps("mig-1", "missing")).resolves.toBeNull();
  });

  it("throws with the response body text on other errors", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "server error"));
    await expect(getCandidateSteps("mig-1", "billing-api")).rejects.toThrow("server error");
  });
});

// ---------------------------------------------------------------------------
// startRun
// ---------------------------------------------------------------------------

describe("startRun", () => {
  it("calls the correct candidate-scoped URL", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(202, null));
    await startRun("migration-id", "billing-api");
    expect(mockFetch.mock.calls[0][0]).toBe(
      "/api/migrations/migration-id/candidates/billing-api/start",
    );
  });

  it("resolves without a value on 202", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(202, null));
    await expect(startRun("migration-id", "billing-api")).resolves.toBeUndefined();
  });

  it("throws ConflictError on 409", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(409, "already queued"));
    await expect(startRun("migration-id", "billing-api")).rejects.toBeInstanceOf(ConflictError);
  });

  it("throws with the response body text on other errors", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(500, "server error"));
    await expect(startRun("migration-id", "billing-api")).rejects.toThrow("server error");
  });

  it("omits body entirely when inputs is undefined", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(202, null));
    await startRun("migration-id", "billing-api");
    expect(mockFetch.mock.calls[0][1].body).toBeUndefined();
  });

  it("omits body when inputs is an empty object", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(202, null));
    await startRun("migration-id", "billing-api", {});
    expect(mockFetch.mock.calls[0][1].body).toBeUndefined();
  });

  it("includes inputs in the body when non-empty inputs are provided", async () => {
    mockFetch.mockResolvedValueOnce(mockResponse(202, null));
    await startRun("migration-id", "billing-api", { token: "abc", env: "prod" });
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.inputs).toEqual({ token: "abc", env: "prod" });
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
