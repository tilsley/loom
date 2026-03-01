import type { Migration } from "@/lib/api";
import { getDashboardStats } from "@/lib/stats";

interface DashboardStatsProps {
  migrations: Migration[];
}

export function DashboardStats({ migrations }: DashboardStatsProps) {
  const { activeCandidates, completedCandidates } = getDashboardStats(migrations);

  const stats = [
    { label: "Migrations", value: migrations.length, accent: false },
    { label: "Active", value: activeCandidates, accent: activeCandidates > 0 },
    { label: "Completed", value: completedCandidates, accent: completedCandidates > 0 },
  ];

  return (
    <div className="grid grid-cols-3 gap-3">
      {stats.map((s) => (
        <div
          key={s.label}
          className="bg-card/50 border border-border rounded-lg px-3.5 py-3"
        >
          <div className="text-xs text-muted-foreground uppercase tracking-widest mb-1.5">
            {s.label}
          </div>
          <div
            className={`text-lg font-mono font-medium ${
              s.accent ? "text-primary" : "text-foreground"
            }`}
          >
            {s.value}
          </div>
        </div>
      ))}
    </div>
  );
}
