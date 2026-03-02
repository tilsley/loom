import { describe, expect, it } from "vitest";
import type { Candidate } from "@/lib/api";
import { discoverMetadataColumns, filterCandidates } from "../filtering";

const candidate = (id: string, status: Candidate["status"] = "not_started", metadata?: Record<string, string>): Candidate => ({
  id,
  kind: "repo",
  status,
  ...(metadata ? { metadata } : {}),
});

describe("discoverMetadataColumns", () => {
  it("returns empty array when no candidates have metadata", () => {
    expect(discoverMetadataColumns([candidate("a"), candidate("b")])).toEqual([]);
  });

  it("returns sorted unique keys across all candidates", () => {
    const candidates = [
      candidate("a", undefined, { team: "platform", env: "prod" }),
      candidate("b", undefined, { team: "data" }),
      candidate("c"),
    ];
    expect(discoverMetadataColumns(candidates)).toEqual(["env", "team"]);
  });


});

describe("filterCandidates", () => {
  const candidates = [
    candidate("billing-api", "not_started", { team: "platform", env: "prod" }),
    candidate("auth-service", "running", { team: "security", env: "prod" }),
    candidate("analytics", "completed", { team: "data", env: "staging" }),
  ];
  const metaColumns = ["env", "team"];

  it("returns all candidates when no filters applied", () => {
    expect(filterCandidates(candidates, {}, metaColumns, "all")).toEqual(candidates);
  });

  it("filters by id substring (case-insensitive)", () => {
    const result = filterCandidates(candidates, { id: "AUTH" }, metaColumns, "all");
    expect(result.map((c) => c.id)).toEqual(["auth-service"]);
  });

  it("filters by metadata column substring", () => {
    const result = filterCandidates(candidates, { team: "platform" }, metaColumns, "all");
    expect(result.map((c) => c.id)).toEqual(["billing-api"]);
  });

  it("filters by status", () => {
    const result = filterCandidates(candidates, {}, metaColumns, "running");
    expect(result.map((c) => c.id)).toEqual(["auth-service"]);
  });

  it("includes not_started candidates when filtering by not_started", () => {
    const extra = candidate("orphan");
    const result = filterCandidates([...candidates, extra], {}, metaColumns, "not_started");
    expect(result.map((c) => c.id)).toEqual(["billing-api", "orphan"]);
  });

  it("combines id, metadata, and status filters (AND logic)", () => {
    const result = filterCandidates(
      candidates,
      { id: "a", env: "prod" },
      metaColumns,
      "running",
    );
    expect(result.map((c) => c.id)).toEqual(["auth-service"]);
  });

  it("returns empty array when no candidates match", () => {
    expect(filterCandidates(candidates, { id: "xyz" }, metaColumns, "all")).toEqual([]);
  });
});
