import type { Candidate, Migration } from "@/lib/api";

export function getCandidateCounts(candidates: Candidate[]): {
  running: number;
  completed: number;
  not_started: number;
} {
  const counts = { running: 0, completed: 0, not_started: 0 };
  for (const c of candidates) {
    if (c.status === "completed") counts.completed++;
    else if (c.status === "running") counts.running++;
    else counts.not_started++;
  }
  return counts;
}

export function getMigrationRunStats(candidates: Candidate[]): {
  runCount: number;
  doneCount: number;
} {
  const runCount = candidates.filter(
    (c) => c.status === "running" || c.status === "completed",
  ).length;
  const doneCount = candidates.filter((c) => c.status === "completed").length;
  return { runCount, doneCount };
}

export function getDashboardStats(migrations: Migration[]): {
  activeCandidates: number;
  completedCandidates: number;
} {
  let activeCandidates = 0;
  let completedCandidates = 0;
  for (const m of migrations) {
    for (const c of m.candidates ?? []) {
      if (c.status === "running") activeCandidates++;
      else if (c.status === "completed") completedCandidates++;
    }
  }
  return { activeCandidates, completedCandidates };
}

export function filterActiveMigrations(migrations: Migration[]): Migration[] {
  return migrations.filter((m) =>
    (m.candidates ?? []).some((c) => c.status === "running"),
  );
}
