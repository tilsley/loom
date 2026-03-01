import type { Candidate, CandidateStepsResponse, Migration } from "@/lib/api";
import type { components } from "@/lib/api.gen";

type StepDefinition = components["schemas"]["StepDefinition"];

export function getApplicableSteps(
  candidate: Candidate | null,
  migration: Migration | null,
): StepDefinition[] {
  return (candidate?.steps?.length ? candidate.steps : migration?.steps) ?? [];
}

export function buildStepDescriptionMap(steps: StepDefinition[]): Map<string, string> {
  return new Map(
    steps.filter((s) => s.description).map((s) => [s.name, s.description ?? ""]),
  );
}

export function calculateStepProgress(
  stepsData: CandidateStepsResponse,
  totalSteps: number,
): { done: number; total: number; activeStepName: string | undefined } {
  const reported = stepsData.steps;
  const done = reported.filter(
    (s) => s.status === "succeeded" || s.status === "merged",
  ).length;
  const active =
    reported.find((s) => s.status === "in_progress") ??
    reported.find((s) => s.status === "failed") ??
    reported.find((s) => s.status === "pending");
  return { done, total: totalSteps, activeStepName: active?.stepName };
}
