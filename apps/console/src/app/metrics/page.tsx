"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  getMetricsOverview,
  getStepMetrics,
  getMetricsTimeline,
  getRecentFailures,
  type MetricsOverview,
  type StepMetrics,
  type TimelinePoint,
  type StepEventFailure,
} from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { MetricsChart } from "@/components/metrics-chart";
import { Skeleton } from "@/components/ui";

export default function MetricsPage() {
  const [loading, setLoading] = useState(true);
  const [overview, setOverview] = useState<MetricsOverview | null>(null);
  const [steps, setSteps] = useState<StepMetrics[]>([]);
  const [timeline, setTimeline] = useState<TimelinePoint[]>([]);
  const [failures, setFailures] = useState<StepEventFailure[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [o, s, t, f] = await Promise.all([
        getMetricsOverview(),
        getStepMetrics(),
        getMetricsTimeline(30),
        getRecentFailures(20),
      ]);
      setOverview(o);
      setSteps(s);
      setTimeline(t);
      setFailures(f);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load metrics");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (error) {
    return (
      <div className="space-y-4 animate-fade-in-up">
        <h1 className="text-xl font-semibold tracking-tight text-foreground">Metrics</h1>
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in-up">
      <div>
        <h1 className="text-xl font-semibold tracking-tight text-foreground">Metrics</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Migration analytics and step performance
        </p>
      </div>

      {/* Overview cards */}
      {loading ? (
        <div className="grid grid-cols-4 gap-3">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-[72px]" style={{ animationDelay: `${i * 100}ms` }} />
          ))}
        </div>
      ) : overview ? (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <StatCard label="Total Runs" value={overview.totalRuns} />
          <StatCard label="PRs Raised" value={overview.prsRaised} />
          <StatCard label="Avg Duration" value={formatMs(overview.avgDurationMs)} />
          <StatCard
            label="Failure Rate"
            value={`${(overview.failureRate * 100).toFixed(1)}%`}
            destructive={overview.failureRate > 0.1}
          />
        </div>
      ) : null}

      {/* Timeline chart */}
      <section className="rounded-lg border border-border bg-card p-4">
        <h2 className="text-sm font-medium text-foreground mb-3">Activity (30 days)</h2>
        {loading ? <Skeleton className="h-[264px]" /> : <MetricsChart data={timeline} />}
      </section>

      {/* Step duration table */}
      {!loading && steps.length > 0 && (
        <section className="rounded-lg border border-border overflow-hidden">
          <div className="px-4 py-3 border-b border-border">
            <h2 className="text-sm font-medium text-foreground">Step Performance</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="text-left px-4 py-2 font-medium">Step</th>
                  <th className="text-right px-4 py-2 font-medium">Count</th>
                  <th className="text-right px-4 py-2 font-medium">Avg (ms)</th>
                  <th className="text-right px-4 py-2 font-medium">P95 (ms)</th>
                  <th className="text-right px-4 py-2 font-medium">Fail %</th>
                </tr>
              </thead>
              <tbody>
                {steps.map((s) => (
                  <tr key={s.stepName} className="border-b border-border/60 last:border-0">
                    <td className="px-4 py-2 font-mono text-foreground">{s.stepName}</td>
                    <td className="px-4 py-2 text-right text-muted-foreground">{s.count}</td>
                    <td className="px-4 py-2 text-right font-mono text-muted-foreground">
                      {Math.round(s.avgMs)}
                    </td>
                    <td className="px-4 py-2 text-right font-mono text-muted-foreground">
                      {Math.round(s.p95Ms)}
                    </td>
                    <td className="px-4 py-2 text-right">
                      <span
                        className={
                          s.failureRate > 0.1 ? "text-destructive" : "text-muted-foreground"
                        }
                      >
                        {(s.failureRate * 100).toFixed(1)}%
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* Recent failures */}
      {!loading && failures.length > 0 && (
        <section className="rounded-lg border border-border overflow-hidden">
          <div className="px-4 py-3 border-b border-border">
            <h2 className="text-sm font-medium text-foreground">Recent Failures</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="text-left px-4 py-2 font-medium">Migration</th>
                  <th className="text-left px-4 py-2 font-medium">Candidate</th>
                  <th className="text-left px-4 py-2 font-medium">Step</th>
                  <th className="text-left px-4 py-2 font-medium">Status</th>
                  <th className="text-left px-4 py-2 font-medium">Time</th>
                  <th className="text-right px-4 py-2 font-medium" />
                </tr>
              </thead>
              <tbody>
                {failures.map((f) => (
                  <tr key={f.id} className="border-b border-border/60 last:border-0">
                    <td className="px-4 py-2 font-mono text-foreground truncate max-w-[160px]">
                      {f.migrationId}
                    </td>
                    <td className="px-4 py-2 font-mono text-muted-foreground truncate max-w-[160px]">
                      {f.candidateId}
                    </td>
                    <td className="px-4 py-2 font-mono text-muted-foreground">{f.stepName}</td>
                    <td className="px-4 py-2">
                      <span className="text-xs font-medium px-2 py-0.5 rounded-full border text-destructive bg-destructive/10 border-destructive/20">
                        {f.status}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-xs text-muted-foreground whitespace-nowrap">
                      {new Date(f.createdAt).toLocaleString()}
                    </td>
                    <td className="px-4 py-2 text-right">
                      <Link
                        href={ROUTES.candidateSteps(f.migrationId, f.candidateId)}
                        className="text-xs text-primary hover:text-primary/80 font-medium transition-colors"
                      >
                        View steps
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {!loading && steps.length === 0 && failures.length === 0 && (
        <div className="rounded-lg border border-border bg-card px-6 py-12 text-center">
          <p className="text-sm text-muted-foreground">
            No events recorded yet. Run a migration to see metrics here.
          </p>
        </div>
      )}
    </div>
  );
}

function StatCard({
  label,
  value,
  destructive,
}: {
  label: string;
  value: string | number;
  destructive?: boolean;
}) {
  return (
    <div className="rounded-lg border border-border bg-card px-4 py-3">
      <p className="text-xs text-muted-foreground mb-0.5">{label}</p>
      <p
        className={`text-lg font-semibold tracking-tight ${destructive ? "text-destructive" : "text-foreground"}`}
      >
        {value}
      </p>
    </div>
  );
}

function formatMs(ms: number): string {
  if (ms === 0) return "0ms";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60_000).toFixed(1)}m`;
}
