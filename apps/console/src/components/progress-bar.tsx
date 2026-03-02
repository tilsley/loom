import type { Candidate } from "@/lib/api";
import { getCandidateCounts } from "@/lib/stats";

interface ProgressBarProps {
  candidates: Candidate[];
}

export function ProgressBar({ candidates }: ProgressBarProps) {
  const total = candidates.length;
  const counts = getCandidateCounts(candidates);

  const segments = [
    { key: "completed", count: counts.completed, color: "bg-completed-fill", label: "completed" },
    { key: "running", count: counts.running, color: "bg-running-fill", label: "running" },
    { key: "not_started", count: counts.not_started, color: "bg-not-started-fill", label: "not started" },
  ] as const;

  return (
    <div>
      <div className="flex items-center gap-3 text-xs text-muted-foreground mb-3">
        {segments
          .filter((s) => s.count > 0)
          .map((s, i, arr) => (
            <span key={s.key} className="flex items-center gap-1">
              <span className={`w-2 h-2 rounded-full ${s.color}`} />
              <span className="font-mono">{s.count}</span> {s.label}
              {i < arr.length - 1 && <span className="text-muted-foreground/50 ml-2">&middot;</span>}
            </span>
          ))}
      </div>
      <div className="flex h-2.5 rounded-full overflow-hidden bg-muted">
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
