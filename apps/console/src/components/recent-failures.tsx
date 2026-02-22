import Link from "next/link";
import type { RegisteredMigration } from "@/lib/api";

interface RecentFailuresProps {
  migrations: RegisteredMigration[];
}

interface CancellationEntry {
  migrationName: string;
  migrationId: string;
  candidateId: string;
  runId: string;
  cancelledAt: string;
}

export function RecentFailures({ migrations }: RecentFailuresProps) {
  const cancellations: CancellationEntry[] = [];

  for (const m of migrations) {
    if (!m.cancelledAttempts) continue;
    for (const ca of m.cancelledAttempts) {
      cancellations.push({
        migrationName: m.name,
        migrationId: m.id,
        candidateId: ca.candidateId,
        runId: ca.runId,
        cancelledAt: ca.cancelledAt,
      });
    }
  }

  cancellations.sort((a, b) => new Date(b.cancelledAt).getTime() - new Date(a.cancelledAt).getTime());
  const limited = cancellations.slice(0, 10);

  return (
    <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-lg">
      <div className="px-4 py-3 border-b border-zinc-800/60">
        <h3 className="text-xs font-medium text-zinc-500 uppercase tracking-widest">
          Cancelled Runs
        </h3>
      </div>
      <div className="p-4">
        {limited.length === 0 ? (
          <p className="text-sm text-zinc-600 py-4 text-center">No cancellations</p>
        ) : (
          <div className="space-y-2">
            {limited.map((f) => (
              <div
                key={`${f.migrationId}-${f.candidateId}`}
                className="flex items-center justify-between gap-2 py-1.5"
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="w-1.5 h-1.5 rounded-full bg-zinc-500 shrink-0" />
                    <span className="text-xs font-mono text-zinc-300 truncate">{f.candidateId}</span>
                  </div>
                  <Link
                    href={`/migrations/${f.migrationId}`}
                    className="text-xs text-zinc-500 hover:text-zinc-300 transition-colors ml-3.5"
                  >
                    {f.migrationName}
                  </Link>
                </div>
                <Link
                  href={`/runs/${f.runId}`}
                  className="text-xs font-mono text-zinc-600 hover:text-teal-400 transition-colors shrink-0"
                >
                  View
                </Link>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
