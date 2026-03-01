import { describe, expect, it } from "vitest";
import type { Candidate, Migration } from "@/lib/api";
import {
  getCandidateCounts,
  getDashboardStats,
  filterActiveMigrations,
  getMigrationRunStats,
} from "../stats";

const c = (status?: Candidate["status"]): Candidate => ({
  id: "x",
  kind: "repo",
  ...(status ? { status } : {}),
});

const migration = (candidates: Candidate[]): Migration => ({
  id: "m",
  name: "m",
  description: "",
  migratorUrl: "",
  candidates,
  steps: [],
  createdAt: new Date().toISOString(),
});

describe("getCandidateCounts", () => {
  it("counts empty list as all zeros", () => {
    expect(getCandidateCounts([])).toEqual({ running: 0, completed: 0, not_started: 0 });
  });

  it("counts correctly across all three statuses", () => {
    const candidates = [
      c("completed"),
      c("running"),
      c("running"),
      c(), // no status → not_started
      c("not_started"),
    ];
    expect(getCandidateCounts(candidates)).toEqual({ running: 2, completed: 1, not_started: 2 });
  });


});

describe("getMigrationRunStats", () => {
  it("returns zeros for candidates with no runs", () => {
    expect(getMigrationRunStats([c(), c()])).toEqual({ runCount: 0, doneCount: 0 });
  });

  it("counts running and completed as runCount, only completed as doneCount", () => {
    const candidates = [c("running"), c("completed"), c("completed"), c()];
    expect(getMigrationRunStats(candidates)).toEqual({ runCount: 3, doneCount: 2 });
  });
});

describe("getDashboardStats", () => {
  it("returns zeros for empty migrations list", () => {
    expect(getDashboardStats([])).toEqual({ activeCandidates: 0, completedCandidates: 0 });
  });

  it("aggregates candidates across multiple migrations", () => {
    const migrations = [
      migration([c("running"), c("completed")]),
      migration([c("running"), c()]),
    ];
    expect(getDashboardStats(migrations)).toEqual({
      activeCandidates: 2,
      completedCandidates: 1,
    });
  });
});

describe("filterActiveMigrations", () => {
  it("returns only migrations with at least one running candidate", () => {
    const active = migration([c("running"), c("completed")]);
    const idle = migration([c("completed"), c()]);
    expect(filterActiveMigrations([active, idle])).toEqual([active]);
  });

  it("returns empty array when nothing is running", () => {
    expect(filterActiveMigrations([migration([c("completed")])])).toEqual([]);
  });
});
