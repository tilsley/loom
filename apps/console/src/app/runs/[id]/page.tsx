"use client";

import { useEffect, useState, useCallback, useMemo, useRef } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  getStatus,
  getMigration,
  completeStep,
  NotFoundError,
  type StatusResponse,
  type RegisteredMigration,
} from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { StatusBadge } from "@/components/status-badge";
import { StepTimeline } from "@/components/step-timeline";
import { Skeleton, buttonVariants } from "@/components/ui";

export default function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [migration, setMigration] = useState<RegisteredMigration | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notFound, setNotFound] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  }, []);

  const poll = useCallback(async () => {
    try {
      const data = await getStatus(id);
      setStatus(data);
      setError(null);
    } catch (e) {
      if (e instanceof NotFoundError) {
        setNotFound(true);
        stopPolling();
        return;
      }
      setError(e instanceof Error ? e.message : "Failed to fetch");
    }
  }, [id, stopPolling]);

  useEffect(() => {
    void poll();
    intervalRef.current = setInterval(() => {
      void poll();
    }, 2000);
    return stopPolling;
  }, [poll, stopPolling]);

  // Fetch migration definition for step descriptions.
  // The run ID format is "{registrationId}-{unixTimestamp}", so we strip the trailing
  // numeric suffix to recover the registration ID (e.g. "app-chart-migration-1771581227"
  // → "app-chart-migration"). status.result.migrationId is the run ID, not the reg ID.
  const registrationId = id.replace(/-\d+$/, "");
  useEffect(() => {
    getMigration(registrationId)
      .then(setMigration)
      .catch(() => {});
  }, [registrationId]);

  // Map stepName → description from the registered migration
  const stepDescriptions = useMemo(() => {
    if (!migration) return new Map<string, string>();
    return new Map(
      migration.steps.filter((s) => s.description).map((s) => [s.name, s.description!]),
    );
  }, [migration]);

  // Map stepName → file URL list from the registered migration
  const stepFiles = useMemo(() => {
    if (!migration) return new Map<string, string[]>();
    return new Map(migration.steps.filter((s) => s.files?.length).map((s) => [s.name, s.files!]));
  }, [migration]);

  // Unique targets involved in this run
  const targets = useMemo(() => {
    if (!status?.result?.results.length) return [];
    const seen = new Set<string>();
    return status.result.results
      .map((r) => r.target)
      .filter((t) => {
        if (seen.has(t.repo)) return false;
        seen.add(t.repo);
        return true;
      });
  }, [status]);

  const stats = useMemo(() => {
    if (!status?.result) return null;
    const results = status.result.results;
    const completed = results.filter(
      (r) => r.success && r.metadata?.phase !== "in_progress",
    ).length;
    const failed = results.filter((r) => !r.success).length;
    const prs = results.filter((r) => r.metadata?.prUrl).length;
    const merged = results.filter((r) => r.metadata?.phase === "merged").length;
    return { total: results.length, completed, failed, prs, merged };
  }, [status]);

  return (
    <div className="space-y-8 animate-fade-in-up">
      {/* Workflow not found state */}
      {notFound ? (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <div className="w-14 h-14 rounded-xl bg-zinc-800/50 border border-zinc-700/50 flex items-center justify-center mb-5">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" className="text-zinc-500">
              <path
                d="M12 8v4m0 4h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-1">Workflow Not Found</h2>
          <p className="text-sm text-zinc-500 mb-2 max-w-md">
            The workflow instance for this run no longer exists. This typically happens when the
            orchestration engine is restarted and ephemeral state is lost.
          </p>
          <p className="text-xs font-mono text-zinc-600 mb-6 break-all max-w-md">{id}</p>
          <Link href={ROUTES.migrations} className={buttonVariants({ variant: "outline" })}>
            Back to Migrations
          </Link>
        </div>
      ) : (
        <>
          {/* Header — single line: back link · migration name · targets · run id  [status] */}
          <div className="flex items-center gap-2 flex-wrap">
            <Link
              href={ROUTES.migrations}
              className="inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-300 transition-colors shrink-0"
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M7 3L4 6l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              Migrations
            </Link>

            <span className="text-zinc-700 select-none">·</span>

            <span className="text-sm font-medium text-foreground">
              {migration?.name ?? registrationId}
            </span>

            {targets.length > 0 && <span className="text-zinc-700 select-none">·</span>}
            {targets.map((t) => {
              const team = t.repo.split("/")[0];
              const appName = t.metadata?.appName ?? t.repo.split("/")[1] ?? t.repo;
              return (
                <span
                  key={t.repo}
                  className="inline-flex items-center gap-1.5 text-xs font-mono bg-zinc-800/60 border border-zinc-700/50 text-zinc-300 px-2 py-0.5 rounded-md"
                >
                  <span className="text-zinc-500">{team} /</span>
                  {appName}
                </span>
              );
            })}

            {status ? (
              <>
                <span className="flex-1" />
                <StatusBadge status={status.runtimeStatus} />
              </>
            ) : null}
          </div>

          {error ? (
            <div className="bg-red-500/8 border border-red-500/20 rounded-lg px-4 py-3 text-sm text-red-400">
              {error}
            </div>
          ) : null}

          {/* Loading skeleton */}
          {!status && !error ? (
            <div className="space-y-4">
              <div className="grid grid-cols-4 gap-3">
                {[1, 2, 3, 4].map((i) => (
                  <Skeleton
                    key={i}
                    className="h-[72px]"
                    style={{ animationDelay: `${i * 80}ms` }}
                  />
                ))}
              </div>
              <Skeleton className="h-32 w-full" />
            </div>
          ) : null}

          {/* Summary stats */}
          {stats ? (
            <div className="grid grid-cols-4 gap-3">
              <StatCard label="Total Steps" value={stats.total} />
              <StatCard
                label="Completed"
                value={stats.completed}
                accent={stats.completed > 0 ? "emerald" : undefined}
              />
              <StatCard
                label="Failed"
                value={stats.failed}
                accent={stats.failed > 0 ? "red" : undefined}
              />
              <StatCard
                label="Pull Requests"
                value={stats.prs}
                sub={stats.merged > 0 ? `${stats.merged} merged` : undefined}
                accent={stats.prs > 0 ? "teal" : undefined}
              />
            </div>
          ) : null}

          {/* Step timeline */}
          {status ? (
            <section className="w-fit min-w-[700px] mx-auto">
              <div className="flex items-center gap-2 mb-4">
                <h2 className="text-sm font-medium text-zinc-400 uppercase tracking-widest">
                  Steps
                </h2>
                {status.result ? (
                  <span className="text-[11px] font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
                    {status.result.results.length}
                  </span>
                ) : null}
              </div>
              <div className="border border-zinc-800/80 rounded-lg p-6 overflow-y-auto max-h-[44rem]">
                <StepTimeline
                  results={status.result?.results ?? []}
                  stepDescriptions={stepDescriptions}
                  stepFiles={stepFiles}
                  onComplete={(stepName, target, success) => {
                    void (async () => {
                      await completeStep(id, stepName, target, success);
                      void poll();
                    })();
                  }}
                />
              </div>
            </section>
          ) : null}
        </>
      )}
    </div>
  );
}

function StatCard({
  label,
  value,
  sub,
  accent,
}: {
  label: string;
  value: number;
  sub?: string;
  accent?: "emerald" | "red" | "teal";
}) {
  const valueColor =
    accent === "emerald"
      ? "text-emerald-400"
      : accent === "red"
        ? "text-red-400"
        : accent === "teal"
          ? "text-teal-400"
          : "text-zinc-100";

  return (
    <div className="bg-zinc-900/50 border border-zinc-800/80 rounded-lg px-4 py-3.5">
      <div className="text-[11px] text-zinc-500 uppercase tracking-widest mb-1.5">{label}</div>
      <div className={`text-2xl font-mono font-semibold ${valueColor}`}>{value}</div>
      {Boolean(sub) && <div className="text-[11px] text-zinc-600 font-mono mt-0.5">{sub}</div>}
    </div>
  );
}
