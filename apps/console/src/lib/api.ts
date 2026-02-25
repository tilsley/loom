import type { components } from "./api.gen";

export type StepResult = components["schemas"]["StepResult"];
export type Migration = components["schemas"]["Migration"];
export type Candidate = components["schemas"]["Candidate"];
export type CandidateStatus = components["schemas"]["CandidateStatus"];
export type CandidateStepsResponse = components["schemas"]["CandidateStepsResponse"];
export type DryRunResult = components["schemas"]["DryRunResult"];
export type FileDiff = components["schemas"]["FileDiff"];

const BASE = "/api";

export async function listMigrations(): Promise<{ migrations: Migration[] }> {
  const res = await fetch(`${BASE}/migrations`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getMigration(id: string): Promise<Migration> {
  const res = await fetch(`${BASE}/migrations/${id}`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export class ConflictError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ConflictError";
  }
}

export async function completeStep(
  runId: string,
  stepName: string,
  candidateId: string,
  success: boolean,
): Promise<void> {
  const res = await fetch(`${BASE}/event/${runId}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ stepName, candidateId, success }),
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function getCandidates(id: string): Promise<Candidate[]> {
  const res = await fetch(`${BASE}/migrations/${id}/candidates`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getCandidateSteps(
  migrationId: string,
  candidateId: string,
): Promise<CandidateStepsResponse | null> {
  const res = await fetch(`${BASE}/migrations/${migrationId}/candidates/${candidateId}/steps`);
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function startRun(
  migrationId: string,
  candidateId: string,
  inputs?: Record<string, string>,
): Promise<void> {
  const body = inputs && Object.keys(inputs).length > 0 ? { inputs } : undefined;
  const res = await fetch(`${BASE}/migrations/${migrationId}/candidates/${candidateId}/start`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  });
  if (res.status === 409) throw new ConflictError(await res.text());
  if (!res.ok) throw new Error(await res.text());
}

export async function cancelRun(migrationId: string, candidateId: string): Promise<void> {
  const res = await fetch(`${BASE}/migrations/${migrationId}/candidates/${candidateId}/cancel`, {
    method: "POST",
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function retryStep(
  migrationId: string,
  candidateId: string,
  stepName: string,
): Promise<void> {
  const res = await fetch(
    `${BASE}/migrations/${migrationId}/candidates/${candidateId}/retry-step`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ stepName }),
    },
  );
  if (!res.ok) throw new Error(await res.text());
}

export async function dryRun(migrationId: string, candidate: Candidate): Promise<DryRunResult> {
  const res = await fetch(`${BASE}/migrations/${encodeURIComponent(migrationId)}/dry-run`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ candidate }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<DryRunResult>;
}
