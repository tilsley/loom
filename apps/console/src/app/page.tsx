"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useMigrationPolling } from "@/lib/hooks";
import { ROUTES } from "@/lib/routes";
import { DashboardStats } from "@/components/dashboard-stats";
import { ActiveRuns } from "@/components/active-runs";
import { Input, Skeleton } from "@/components/ui";
import type { Migration } from "@/lib/api";

export default function Dashboard() {
  const { migrations, loading } = useMigrationPolling(5000);
  const [query, setQuery] = useState("");

  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return [];

    const hits: {
      migration: Migration;
      id: string;
      status: string;
      hasRun: boolean;
    }[] = [];

    for (const m of migrations) {
      for (const t of (m.candidates ?? [])) {
        if (t.id.toLowerCase().includes(q)) {
          hits.push({
            migration: m,
            id: t.id,
            status: t.status ?? "not_started",
            hasRun: t.status === "running" || t.status === "completed",
          });
        }
      }
    }

    return hits;
  }, [query, migrations]);

  return (
    <div className="space-y-6 animate-fade-in-up">
      <div>
        <h1 className="text-xl font-semibold tracking-tight text-foreground">Dashboard</h1>
        <p className="text-sm text-muted-foreground mt-1">Migration orchestration overview</p>
      </div>

      {loading ? (
        <div className="grid grid-cols-4 gap-3">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-[72px]" style={{ animationDelay: `${i * 100}ms` }} />
          ))}
        </div>
      ) : (
        <>
          <DashboardStats migrations={migrations} />

          <ActiveRuns migrations={migrations} />
        </>
      )}

      {/* Candidate search */}
      <div className="pt-4 border-t border-border/50 space-y-3">
        <div className="flex items-center gap-3">
          <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-widest shrink-0">
            Find target
          </h2>
          <div className="relative flex-1">
            <svg
              width="14"
              height="14"
              viewBox="0 0 14 14"
              fill="none"
              className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground/70"
            >
              <circle cx="6" cy="6" r="4" stroke="currentColor" strokeWidth="1.5" />
              <path d="M9 9l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
            <Input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search by id, e.g. billing-api"
              className="pl-9 py-1.5 font-mono"
            />
          </div>
        </div>

        {results.length > 0 && (
          <div className="rounded-lg border border-border overflow-hidden">
            {results.map((r, i) => (
              <div
                key={`${r.migration.id}-${r.id}`}
                className={`flex items-center gap-4 px-4 py-3 ${i < results.length - 1 ? "border-b border-border/60" : ""}`}
              >
                {/* Candidate */}
                <span className="flex-1 text-sm font-mono text-foreground truncate min-w-0">
                  {r.id}
                </span>

                {/* Migration name */}
                <span className="text-xs text-muted-foreground truncate hidden sm:block">
                  {r.migration.name}
                </span>

                {/* Status badge */}
                <StatusChip status={r.status} />

                {/* Action link */}
                {r.hasRun ? (
                  <Link
                    href={ROUTES.candidateSteps(r.migration.id, r.id)}
                    className="text-xs text-primary hover:text-primary/80 font-medium shrink-0 transition-colors"
                  >
                    View steps →
                  </Link>
                ) : (
                  <Link
                    href={ROUTES.migrationDetail(r.migration.id)}
                    className="text-xs text-muted-foreground hover:text-foreground/80 font-medium shrink-0 transition-colors"
                  >
                    Go to migration →
                  </Link>
                )}
              </div>
            ))}
          </div>
        )}

        {query.trim() && results.length === 0 && !loading && (
          <p className="text-xs text-muted-foreground/70 pl-1">No targets match &quot;{query}&quot;</p>
        )}
      </div>
    </div>
  );
}

function StatusChip({ status }: { status: string }) {
  const styles: Record<string, string> = {
    not_started: "text-muted-foreground bg-muted border-border",
    running: "text-running bg-running/10 border-running/20",
    completed: "text-completed bg-completed/10 border-completed/20",
    failed: "text-destructive bg-destructive/10 border-destructive/20",
  };
  const cls = styles[status] ?? styles.not_started;
  return (
    <span className={`text-xs font-medium px-2 py-0.5 rounded-full border shrink-0 ${cls}`}>
      {status}
    </span>
  );
}
