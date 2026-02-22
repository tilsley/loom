import type { RegisteredMigration } from "@/lib/api";

interface DashboardStatsProps {
  migrations: RegisteredMigration[];
}

export function DashboardStats({ migrations }: DashboardStatsProps) {
  let activeCandidates = 0;
  let completedCandidates = 0;
  let failedCandidates = 0;

  for (const m of migrations) {
    if (!m.candidateRuns) continue;
    for (const tr of Object.values(m.candidateRuns)) {
      if (tr.status === "running") activeCandidates++;
      else if (tr.status === "completed") completedCandidates++;
      else if (tr.status === "failed") failedCandidates++;
    }
  }

  const stats = [
    { label: "Migrations", value: migrations.length, accent: false },
    { label: "Active Candidates", value: activeCandidates, accent: activeCandidates > 0 },
    { label: "Completed", value: completedCandidates, accent: completedCandidates > 0 },
    { label: "Failed", value: failedCandidates, accent: false, isError: failedCandidates > 0 },
  ];

  return (
    <div className="grid grid-cols-4 gap-3">
      {stats.map((s) => (
        <div
          key={s.label}
          className="bg-zinc-900/50 border border-zinc-800/60 rounded-lg px-3.5 py-3"
        >
          <div className="text-[10px] text-zinc-500 uppercase tracking-widest mb-1.5">
            {s.label}
          </div>
          <div
            className={`text-lg font-mono font-medium ${
              s.isError ? "text-red-400" : s.accent ? "text-teal-400" : "text-zinc-200"
            }`}
          >
            {s.value}
          </div>
        </div>
      ))}
    </div>
  );
}
