import Link from "next/link";
import type { RegisteredMigration } from "@/lib/api";
import { ProgressBar } from "./progress-bar";

interface ActiveRunsProps {
  migrations: RegisteredMigration[];
}

export function ActiveRuns({ migrations }: ActiveRunsProps) {
  const active = migrations.filter((m) => {
    if (!m.candidateRuns) return false;
    return Object.values(m.candidateRuns).some((tr) => tr.status === "running");
  });

  return (
    <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-lg">
      <div className="px-4 py-3 border-b border-zinc-800/60">
        <h3 className="text-xs font-medium text-zinc-500 uppercase tracking-widest">Active Runs</h3>
      </div>
      <div className="p-4">
        {active.length === 0 ? (
          <p className="text-sm text-zinc-600 py-4 text-center">No active runs</p>
        ) : (
          <div className="space-y-4">
            {active.map((m) => {
              const runningCount = m.candidateRuns
                ? Object.values(m.candidateRuns).filter((tr) => tr.status === "running").length
                : 0;

              return (
                <div key={m.id}>
                  <div className="flex items-center justify-between mb-2">
                    <Link
                      href={`/migrations/${m.id}`}
                      className="text-sm font-medium text-zinc-200 hover:text-teal-400 transition-colors truncate"
                    >
                      {m.name}
                    </Link>
                    <span className="text-xs font-mono text-zinc-500 shrink-0 ml-2">
                      {runningCount}/{m.candidates.length} running
                    </span>
                  </div>
                  <ProgressBar candidates={m.candidates} candidateRuns={m.candidateRuns} />
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
