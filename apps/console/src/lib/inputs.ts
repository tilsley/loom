import type { Candidate } from "@/lib/api";
import type { components } from "@/lib/api.gen";

type InputDefinition = components["schemas"]["InputDefinition"];

export function prefillInputs(
  requiredInputs: InputDefinition[],
  candidate: Candidate,
): Record<string, string> {
  const prefilled: Record<string, string> = {};
  for (const inp of requiredInputs) {
    prefilled[inp.name] = candidate.metadata?.[inp.name] ?? "";
  }
  return prefilled;
}

export function prefillInputsFromUrl(
  requiredInputs: InputDefinition[],
  candidate: Candidate,
  searchParams: { get(name: string): string | null },
): { values: Record<string, string>; allFromUrl: boolean } {
  const values: Record<string, string> = {};
  let allFromUrl = true;
  for (const inp of requiredInputs) {
    const urlVal = searchParams.get(inp.name);
    values[inp.name] = urlVal !== null ? urlVal : (candidate.metadata?.[inp.name] ?? "");
    if (urlVal === null) allFromUrl = false;
  }
  return { values, allFromUrl };
}

export function mergeInputsIntoCandidate(
  candidate: Candidate,
  requiredInputs: InputDefinition[],
  inputs: Record<string, string>,
): Candidate {
  if (requiredInputs.length === 0) return candidate;
  const merged = { ...(candidate.metadata ?? {}), ...inputs };
  return { ...candidate, metadata: merged };
}
