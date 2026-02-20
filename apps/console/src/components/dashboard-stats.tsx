import type { RegisteredMigration } from "@/lib/api";

interface DashboardStatsProps {
  migrations: RegisteredMigration[];
}

export function DashboardStats({ migrations }: DashboardStatsProps) {
  let activeTargets = 0;
  let completedTargets = 0;
  let failedTargets = 0;

  for (const m of migrations) {
    if (!m.targetRuns) continue;
    for (const tr of Object.values(m.targetRuns)) {
      if (tr.status === "running") activeTargets++;
      else if (tr.status === "completed") completedTargets++;
      else if (tr.status === "failed") failedTargets++;
    }
  }

  const stats = [
    { label: "Migrations", value: migrations.length, accent: false },
    { label: "Active Targets", value: activeTargets, accent: activeTargets > 0 },
    { label: "Completed", value: completedTargets, accent: completedTargets > 0 },
    { label: "Failed", value: failedTargets, accent: false, isError: failedTargets > 0 },
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
