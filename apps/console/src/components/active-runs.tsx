import Link from "next/link";
import type { Migration } from "@/lib/api";
import { ProgressBar } from "./progress-bar";

interface ActiveRunsProps {
  migrations: Migration[];
}

export function ActiveRuns({ migrations }: ActiveRunsProps) {
  const active = migrations.filter((m) =>
    (m.candidates ?? []).some((c) => c.status === "running"),
  );

  return (
    <div className="bg-card/50 border border-border rounded-lg">
      <div className="px-4 py-3 border-b border-border">
        <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-widest">Active Runs</h3>
      </div>
      <div className="p-4">
        {active.length === 0 ? (
          <p className="text-sm text-muted-foreground/70 py-4 text-center">No active runs</p>
        ) : (
          <div className="space-y-4">
            {active.map((m) => {
              const runningCount = (m.candidates ?? []).filter((c) => c.status === "running").length;

              return (
                <div key={m.id}>
                  <div className="flex items-center justify-between mb-2">
                    <Link
                      href={`/migrations/${m.id}`}
                      className="text-sm font-medium text-foreground hover:text-primary transition-colors truncate"
                    >
                      {m.name}
                    </Link>
                    <span className="text-xs font-mono text-muted-foreground shrink-0 ml-2">
                      {runningCount}/{(m.candidates ?? []).length} running
                    </span>
                  </div>
                  <ProgressBar candidates={m.candidates ?? []} />
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
