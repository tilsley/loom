import Link from "next/link";
import type { RegisteredMigration } from "@/lib/api";

interface RecentFailuresProps {
  migrations: RegisteredMigration[];
}

interface FailureEntry {
  migrationName: string;
  migrationId: string;
  candidateId: string;
  runId: string;
}

export function RecentFailures({ migrations }: RecentFailuresProps) {
  const failures: FailureEntry[] = [];

  for (const m of migrations) {
    if (!m.candidateRuns) continue;
    for (const [candidateId, tr] of Object.entries(m.candidateRuns)) {
      if (tr.status === "failed") {
        failures.push({
          migrationName: m.name,
          migrationId: m.id,
          candidateId,
          runId: tr.runId,
        });
      }
    }
    if (failures.length >= 10) break;
  }

  const limited = failures.slice(0, 10);

  return (
    <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-lg">
      <div className="px-4 py-3 border-b border-zinc-800/60">
        <h3 className="text-xs font-medium text-zinc-500 uppercase tracking-widest">
          Recent Failures
        </h3>
      </div>
      <div className="p-4">
        {limited.length === 0 ? (
          <p className="text-[13px] text-zinc-600 py-4 text-center">No failures</p>
        ) : (
          <div className="space-y-2">
            {limited.map((f) => (
              <div
                key={`${f.migrationId}-${f.candidateId}`}
                className="flex items-center justify-between gap-2 py-1.5"
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="w-1.5 h-1.5 rounded-full bg-red-400 shrink-0" />
                    <span className="text-[12px] font-mono text-zinc-300 truncate">{f.candidateId}</span>
                  </div>
                  <Link
                    href={`/migrations/${f.migrationId}`}
                    className="text-[11px] text-zinc-500 hover:text-zinc-300 transition-colors ml-3.5"
                  >
                    {f.migrationName}
                  </Link>
                </div>
                <Link
                  href={`/runs/${f.runId}`}
                  className="text-[10px] font-mono text-zinc-600 hover:text-teal-400 transition-colors shrink-0"
                >
                  {f.runId.split("-").pop()}
                </Link>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
