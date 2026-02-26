"use client";

import { useEffect, useState, useCallback, useMemo, useRef } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  getCandidateSteps,
  getCandidates,
  getMigration,
  completeStep,
  retryStep,
  type CandidateStepsResponse,
  type Migration,
  type Candidate,
} from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { StepTimeline } from "@/components/step-timeline";
import { Skeleton, buttonVariants } from "@/components/ui";

export default function CandidateStepsPage() {
  const { id, candidateId } = useParams<{ id: string; candidateId: string }>();
  const [stepsData, setStepsData] = useState<CandidateStepsResponse | null>(null);
  const [migration, setMigration] = useState<Migration | null>(null);
  const [allCandidates, setAllCandidates] = useState<Candidate[]>([]);
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
      const data = await getCandidateSteps(id, candidateId);
      if (data === null) {
        setNotFound(true);
        stopPolling();
        return;
      }
      setStepsData(data);
      setError(null);
      // Stop polling once completed
      if (data.status === "completed") {
        stopPolling();
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to fetch");
    }
  }, [id, candidateId, stopPolling]);

  useEffect(() => {
    void poll();
    intervalRef.current = setInterval(() => void poll(), 2000);
    return stopPolling;
  }, [poll, stopPolling]);

  useEffect(() => {
    getMigration(id).then(setMigration).catch(() => {});
  }, [id]);

  // Fetch all candidates for prev/next navigation
  useEffect(() => {
    getCandidates(id).then(setAllCandidates).catch(() => {});
  }, [id]);

  const stepDescriptions = useMemo(() => {
    if (!migration) return new Map<string, string>();
    return new Map(
      migration.steps.filter((s) => s.description).map((s) => [s.name, s.description ?? ""]),
    );
  }, [migration]);

  const stepInstructions = useMemo(() => {
    if (!migration) return new Map<string, string>();
    return new Map(
      migration.steps
        .filter((s) => s.config?.instructions)
        .map((s) => [s.name, s.config?.instructions ?? ""]),
    );
  }, [migration]);

  const stats = useMemo(() => {
    if (!stepsData) return null;
    const results = stepsData.steps;
    const completed = results.filter((r) => r.status === "completed" || r.status === "merged").length;
    const failed = results.filter((r) => r.status === "failed").length;
    const prs = results.filter((r) => r.metadata?.prUrl).length;
    const merged = results.filter((r) => r.status === "merged").length;
    return { total: results.length, completed, failed, prs, merged };
  }, [stepsData]);

  // Prev/next candidate navigation
  const currentIndex = allCandidates.findIndex((c) => c.id === candidateId);
  const prevCandidate = currentIndex > 0 ? allCandidates[currentIndex - 1] : null;
  const nextCandidate = currentIndex >= 0 && currentIndex < allCandidates.length - 1
    ? allCandidates[currentIndex + 1]
    : null;

  // Temporal workflow ID used for event callbacks — derived from migration + candidate IDs
  const runId = `${id}__${candidateId}`;

  return (
    <div className="space-y-8 animate-fade-in-up">
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
            No active or completed workflow found for this candidate. This typically happens when
            the orchestration engine is restarted and ephemeral state is lost.
          </p>
          <p className="text-xs font-mono text-zinc-600 mb-6 break-all max-w-md">{candidateId}</p>
          <Link href={ROUTES.migrationDetail(id)} className={buttonVariants({ variant: "outline" })}>
            Back to Migration
          </Link>
        </div>
      ) : (
        <>
          {/* Header */}
          <div className="flex items-center gap-2 flex-wrap">
            <Link
              href={ROUTES.migrationDetail(id)}
              className="inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-300 transition-colors shrink-0"
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M7 3L4 6l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              {migration?.name ?? id}
            </Link>
            <span className="text-zinc-700 select-none">·</span>
            <span className="inline-flex items-center gap-1.5 text-xs font-mono bg-zinc-800/60 border border-zinc-700/50 text-zinc-300 px-2 py-0.5 rounded-md">
              {candidateId}
            </span>
            {stepsData ? (
              <>
                <span className="flex-1" />
                <span className={`text-xs font-medium px-2.5 py-1 rounded-md border shrink-0 ${
                  stepsData.status === "completed"
                    ? "text-emerald-400 bg-emerald-500/10 border-emerald-500/20"
                    : "text-amber-400 bg-amber-500/10 border-amber-500/20"
                }`}>
                  {stepsData.status}
                </span>
              </>
            ) : null}
          </div>

          {/* Prev/next candidate navigation */}
          {allCandidates.length > 1 ? (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-1">
                {prevCandidate ? (
                  <Link
                    href={ROUTES.candidateSteps(id, prevCandidate.id)}
                    className="inline-flex items-center gap-1.5 text-xs text-zinc-500 hover:text-zinc-300 transition-colors px-2.5 py-1.5 rounded-md border border-zinc-800/80 hover:border-zinc-700 hover:bg-zinc-800/30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path d="M7 3L4 6l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                    <span className="font-mono truncate max-w-[160px]">{prevCandidate.id}</span>
                  </Link>
                ) : (
                  <span className="px-2.5 py-1.5 text-xs text-zinc-700 border border-zinc-800/40 rounded-md">
                    First candidate
                  </span>
                )}
              </div>

              <span className="text-xs text-zinc-600 font-mono tabular-nums">
                {currentIndex + 1} / {allCandidates.length}
              </span>

              <div className="flex items-center gap-1">
                {nextCandidate ? (
                  <Link
                    href={ROUTES.candidateSteps(id, nextCandidate.id)}
                    className="inline-flex items-center gap-1.5 text-xs text-zinc-500 hover:text-zinc-300 transition-colors px-2.5 py-1.5 rounded-md border border-zinc-800/80 hover:border-zinc-700 hover:bg-zinc-800/30"
                  >
                    <span className="font-mono truncate max-w-[160px]">{nextCandidate.id}</span>
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path d="M5 3l3 3-3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </Link>
                ) : (
                  <span className="px-2.5 py-1.5 text-xs text-zinc-700 border border-zinc-800/40 rounded-md">
                    Last candidate
                  </span>
                )}
              </div>
            </div>
          ) : null}

          {error ? (
            <div className="bg-red-500/8 border border-red-500/20 rounded-lg px-4 py-3 text-sm text-red-400">
              {error}
            </div>
          ) : null}

          {/* Loading skeleton */}
          {!stepsData && !error ? (
            <div className="space-y-4">
              <div className="grid grid-cols-4 gap-3">
                {[1, 2, 3, 4].map((i) => (
                  <Skeleton key={i} className="h-[72px]" style={{ animationDelay: `${i * 80}ms` }} />
                ))}
              </div>
              <Skeleton className="h-32 w-full" />
            </div>
          ) : null}

          {/* Summary stats */}
          {stats ? (
            <div className="grid grid-cols-4 gap-3">
              <StatCard label="Total Steps" value={stats.total} />
              <StatCard label="Completed" value={stats.completed} accent={stats.completed > 0 ? "emerald" : undefined} />
              <StatCard label="Failed" value={stats.failed} accent={stats.failed > 0 ? "red" : undefined} />
              <StatCard
                label="Pull Requests"
                value={stats.prs}
                sub={stats.merged > 0 ? `${stats.merged} merged` : undefined}
                accent={stats.prs > 0 ? "teal" : undefined}
              />
            </div>
          ) : null}

          {/* Step timeline */}
          {stepsData ? (
            <section className="w-fit min-w-[700px] mx-auto">
              <div className="flex items-center gap-2 mb-4">
                <h2 className="text-sm font-medium text-zinc-400 uppercase tracking-widest">Steps</h2>
                <span className="text-xs font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
                  {stepsData.steps.length}
                </span>
              </div>
              <div className="border border-zinc-800/80 rounded-lg p-6 overflow-y-auto max-h-[44rem]">
                <StepTimeline
                  results={stepsData.steps}
                  stepDescriptions={stepDescriptions}
                  stepInstructions={stepInstructions}
                  onComplete={(stepName, candidateId, success) => {
                    void (async () => {
                      await completeStep(runId, stepName, candidateId, success);
                      void poll();
                    })();
                  }}
                  onRetry={(stepName, candidateId) => {
                    void (async () => {
                      await retryStep(id, candidateId, stepName);
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
      <div className="text-xs text-zinc-500 uppercase tracking-widest mb-1.5">{label}</div>
      <div className={`text-2xl font-mono font-semibold ${valueColor}`}>{value}</div>
      {Boolean(sub) && <div className="text-xs text-zinc-600 font-mono mt-0.5">{sub}</div>}
    </div>
  );
}
