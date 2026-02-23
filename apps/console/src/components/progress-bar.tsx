import type { Candidate } from "@/lib/api";

interface ProgressBarProps {
  candidates: Candidate[];
}

export function ProgressBar({ candidates }: ProgressBarProps) {
  const total = candidates.length;
  const counts = { completed: 0, running: 0, not_started: 0 };

  for (const c of candidates) {
    if (c.status === "completed") counts.completed++;
    else if (c.status === "running") counts.running++;
    else counts.not_started++;
  }

  const segments = [
    { key: "completed", count: counts.completed, color: "bg-emerald-500", label: "completed" },
    { key: "running", count: counts.running, color: "bg-amber-500", label: "running" },
    { key: "not_started", count: counts.not_started, color: "bg-zinc-700", label: "not started" },
  ] as const;

  return (
    <div>
      <div className="flex items-center gap-3 text-xs text-zinc-400 mb-3">
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
      <div className="flex h-2.5 rounded-full overflow-hidden bg-zinc-800/50">
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
