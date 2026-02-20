import type { Target, TargetRun } from "@/lib/api";

interface ProgressBarProps {
  targets: Target[];
  targetRuns?: Record<string, TargetRun>;
}

export function ProgressBar({ targets, targetRuns }: ProgressBarProps) {
  const total = targets.length;
  const counts = { completed: 0, running: 0, failed: 0, pending: 0 };

  for (const t of targets) {
    const run = targetRuns?.[t.repo];
    if (!run) {
      counts.pending++;
    } else if (run.status === "completed") {
      counts.completed++;
    } else if (run.status === "running") {
      counts.running++;
    } else if (run.status === "failed") {
      counts.failed++;
    } else {
      counts.pending++;
    }
  }

  const segments = [
    { key: "completed", count: counts.completed, color: "bg-emerald-500", label: "completed" },
    { key: "running", count: counts.running, color: "bg-amber-500", label: "running" },
    { key: "failed", count: counts.failed, color: "bg-red-500", label: "failed" },
    { key: "pending", count: counts.pending, color: "bg-zinc-700", label: "pending" },
  ] as const;

  return (
    <div>
      <div className="flex items-center gap-3 text-[12px] text-zinc-400 mb-2">
        {segments
          .filter((s) => s.count > 0)
          .map((s, i, arr) => (
            <span key={s.key} className="flex items-center gap-1">
              <span className={`w-2 h-2 rounded-full ${s.color}`} />
              <span className="font-mono">{s.count}</span> {s.label}
              {i < arr.length - 1 && <span className="text-zinc-700 ml-2">&middot;</span>}
            </span>
          ))}
      </div>
      <div className="flex h-2 rounded-full overflow-hidden bg-zinc-800/50">
        {segments.map(
          (s) =>
            s.count > 0 && (
              <div
                key={s.key}
                className={`${s.color} transition-all duration-500`}
                style={{ width: `${(s.count / total) * 100}%` }}
              />
            ),
        )}
      </div>
    </div>
  );
}
