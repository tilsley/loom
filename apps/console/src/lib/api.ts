import type { components } from "./api.gen";

export type MigrationManifest = components["schemas"]["MigrationManifest"];
export type StepDefinition = components["schemas"]["StepDefinition"];
export type StepResult = components["schemas"]["StepResult"];
export type MigrationResult = components["schemas"]["MigrationResult"];
export type StatusResponse = components["schemas"]["StatusResponse"];
export type StartResponse = components["schemas"]["StartResponse"];
export type StartRequest = components["schemas"]["StartRequest"];
export type RegisteredMigration = components["schemas"]["RegisteredMigration"];
export type RegisterMigrationRequest = components["schemas"]["RegisterMigrationRequest"];
export type ListMigrationsResponse = components["schemas"]["ListMigrationsResponse"];
export type QueueRunResponse = components["schemas"]["QueueRunResponse"];
export type ExecuteRunResponse = components["schemas"]["ExecuteRunResponse"];
export type RunInfo = components["schemas"]["RunInfo"];
export type Candidate = components["schemas"]["Candidate"];
export type CandidateRun = components["schemas"]["CandidateRun"];
export type CandidateStatus = components["schemas"]["CandidateStatus"];
export type CandidateWithStatus = components["schemas"]["CandidateWithStatus"];
export type SubmitCandidatesRequest = components["schemas"]["SubmitCandidatesRequest"];
export type FileGroup = components["schemas"]["FileGroup"];
export type FileRef = components["schemas"]["FileRef"];
export type DryRunResult = components["schemas"]["DryRunResult"];
export type StepDryRunResult = components["schemas"]["StepDryRunResult"];
export type FileDiff = components["schemas"]["FileDiff"];

const BASE = "/api";

export async function startMigration(manifest: MigrationManifest): Promise<StartResponse> {
  const res = await fetch(`${BASE}/start`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ manifest }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getStatus(id: string): Promise<StatusResponse> {
  const res = await fetch(`${BASE}/status/${id}`);
  if (!res.ok) {
    const body = await res.text();
    let message = body;
    try {
      const parsed = JSON.parse(body);
      if (parsed.error) message = parsed.error;
    } catch {
      // plain text error, use as-is
    }
    if (res.status === 404 || message.includes("not found")) {
      throw new NotFoundError(message);
    }
    throw new Error(message);
  }
  return res.json();
}

export async function registerMigration(
  req: RegisterMigrationRequest,
): Promise<RegisteredMigration> {
  const res = await fetch(`${BASE}/migrations`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function listMigrations(): Promise<ListMigrationsResponse> {
  const res = await fetch(`${BASE}/migrations`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getMigration(id: string): Promise<RegisteredMigration> {
  const res = await fetch(`${BASE}/migrations/${id}`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function deleteMigration(id: string): Promise<void> {
  const res = await fetch(`${BASE}/migrations/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error(await res.text());
}

export class ConflictError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ConflictError";
  }
}

export class NotFoundError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "NotFoundError";
  }
}

export async function completeStep(
  runId: string,
  stepName: string,
  candidate: Candidate,
  success: boolean,
): Promise<void> {
  const res = await fetch(`${BASE}/event/${runId}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ stepName, candidate, success }),
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function getCandidates(id: string): Promise<CandidateWithStatus[]> {
  const res = await fetch(`${BASE}/migrations/${id}/candidates`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getRunInfo(runId: string): Promise<RunInfo | null> {
  const res = await fetch(`${BASE}/runs/${runId}`);
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function queueRun(id: string, candidate: Candidate): Promise<QueueRunResponse> {
  const res = await fetch(`${BASE}/migrations/${id}/queue`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ candidate }),
  });
  if (res.status === 409) throw new ConflictError(await res.text());
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function executeRun(runId: string): Promise<ExecuteRunResponse> {
  const res = await fetch(`${BASE}/runs/${runId}/execute`, {
    method: "POST",
  });
  if (res.status === 409) throw new ConflictError(await res.text());
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function dequeueRun(runId: string): Promise<void> {
  const res = await fetch(`${BASE}/runs/${runId}/dequeue`, { method: "DELETE" });
  if (res.status === 409) throw new ConflictError(await res.text());
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
