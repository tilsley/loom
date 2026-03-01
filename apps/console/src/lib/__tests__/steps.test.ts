import { describe, expect, it } from "vitest";
import type { Candidate, CandidateStepsResponse, Migration } from "@/lib/api";
import type { components } from "@/lib/api.gen";
import { getApplicableSteps, buildStepDescriptionMap, calculateStepProgress } from "../steps";

type StepDefinition = components["schemas"]["StepDefinition"];
type StepState = components["schemas"]["StepState"];

const step = (name: string, description?: string): StepDefinition => ({
  name,
  migratorApp: "test",
  ...(description ? { description } : {}),
});

const migrationWith = (steps: StepDefinition[]): Migration => ({
  id: "m",
  name: "m",
  description: "",
  migratorUrl: "",
  candidates: [],
  steps,
  createdAt: new Date().toISOString(),
});

const candidateWith = (steps?: StepDefinition[]): Candidate => ({
  id: "c",
  kind: "repo",
  ...(steps ? { steps } : {}),
});

describe("getApplicableSteps", () => {
  it("falls back to migration steps when candidate has none", () => {
    const migSteps = [step("a"), step("b")];
    expect(getApplicableSteps(candidateWith(), migrationWith(migSteps))).toEqual(migSteps);
  });

  it("uses candidate steps when present", () => {
    const candSteps = [step("x")];
    const migSteps = [step("a"), step("b")];
    expect(getApplicableSteps(candidateWith(candSteps), migrationWith(migSteps))).toEqual(candSteps);
  });

  it("falls back to migration steps when candidate steps array is empty", () => {
    const migSteps = [step("a")];
    expect(getApplicableSteps(candidateWith([]), migrationWith(migSteps))).toEqual(migSteps);
  });

  it("returns empty array when both are null", () => {
    expect(getApplicableSteps(null, null)).toEqual([]);
  });
});

describe("buildStepDescriptionMap", () => {
  it("maps step name → description, skipping steps without description", () => {
    const steps = [step("a", "desc-a"), step("b"), step("c", "desc-c")];
    const map = buildStepDescriptionMap(steps);
    expect(map.get("a")).toBe("desc-a");
    expect(map.get("b")).toBeUndefined();
    expect(map.get("c")).toBe("desc-c");
  });

  it("returns empty map for empty steps list", () => {
    expect(buildStepDescriptionMap([])).toEqual(new Map());
  });
});

describe("calculateStepProgress", () => {
  const makeState = (stepName: string, status: StepState["status"]): StepState => ({
    stepName,
    status,
  });

  const stepsData = (steps: StepState[]): CandidateStepsResponse => ({
    status: "running",
    steps,
  });

  it("counts succeeded and merged as done", () => {
    const data = stepsData([
      makeState("a", "succeeded"),
      makeState("b", "merged"),
      makeState("c", "in_progress"),
    ]);
    const result = calculateStepProgress(data, 5);
    expect(result.done).toBe(2);
    expect(result.total).toBe(5);
  });

  it("prefers in_progress step as active", () => {
    const data = stepsData([
      makeState("a", "succeeded"),
      makeState("b", "in_progress"),
      makeState("c", "pending"),
    ]);
    expect(calculateStepProgress(data, 3).activeStepName).toBe("b");
  });

  it("falls back to failed step when there is no in_progress step", () => {
    const data = stepsData([
      makeState("a", "succeeded"),
      makeState("b", "failed"),
      makeState("c", "pending"),
    ]);
    expect(calculateStepProgress(data, 3).activeStepName).toBe("b");
  });

  it("falls back to pending step when there is no in_progress or failed step", () => {
    const data = stepsData([
      makeState("a", "succeeded"),
      makeState("b", "pending"),
    ]);
    expect(calculateStepProgress(data, 2).activeStepName).toBe("b");
  });

  it("returns undefined activeStepName when all steps are done", () => {
    const data = stepsData([makeState("a", "succeeded"), makeState("b", "merged")]);
    expect(calculateStepProgress(data, 2).activeStepName).toBeUndefined();
  });
});
