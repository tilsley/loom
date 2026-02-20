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
export type RunMigrationResponse = components["schemas"]["RunMigrationResponse"];
export type Target = components["schemas"]["Target"];
export type TargetRun = components["schemas"]["TargetRun"];

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
  target: Target,
  success: boolean,
): Promise<void> {
  const res = await fetch(`${BASE}/event/${runId}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ stepName, target, success }),
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function runMigration(id: string, target: Target): Promise<RunMigrationResponse> {
  const res = await fetch(`${BASE}/migrations/${id}/run`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ target }),
  });
  if (res.status === 409) throw new ConflictError(await res.text());
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
