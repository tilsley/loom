"use client";

import { useEffect, useState, useCallback, useMemo, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  getStatus,
  getMigration,
  getRunInfo,
  executeRun,
  dequeueRun,
  completeStep,
  dryRun,
  NotFoundError,
  ConflictError,
  type StatusResponse,
  type RegisteredMigration,
  type RunInfo,
  type DryRunResult,
  type FileDiff,
} from "@/lib/api";
import { toast } from "sonner";
import { ROUTES } from "@/lib/routes";
import { StatusBadge } from "@/components/status-badge";
import { StepTimeline } from "@/components/step-timeline";
import { Skeleton, buttonVariants } from "@/components/ui";

export default function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [runInfo, setRunInfo] = useState<RunInfo | null>(null);
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

  const startPolling = useCallback(
    (pollFn: () => Promise<void>) => {
      stopPolling();
      void pollFn();
      intervalRef.current = setInterval(() => void pollFn(), 2000);
    },
    [stopPolling],
  );

  const poll = useCallback(async () => {
    try {
      const data = await getStatus(id);
      setStatus(data);
      setError(null);
    } catch (e) {
      if (e instanceof NotFoundError) {
        // Workflow not in Temporal — check if it's just queued (no workflow started yet).
        const info = await getRunInfo(id).catch(() => null);
        setRunInfo(info);
        if (!info || info.status !== "queued") {
          setNotFound(true);
        }
        stopPolling();
        return;
      }
      setError(e instanceof Error ? e.message : "Failed to fetch");
    }
  }, [id, stopPolling]);

  useEffect(() => {
    startPolling(poll);
    return stopPolling;
  }, [poll, startPolling, stopPolling]);

  // Run ID format is "{migrationId}__{candidateId}" — split on "__" to recover the
  // registration ID. status.result.migrationId is the run ID, not the reg ID.
  const registrationId = id.split("__")[0] ?? id;
  useEffect(() => {
    getMigration(registrationId)
      .then(setMigration)
      .catch(() => {});
  }, [registrationId]);

  // Map stepName → description from the registered migration
  const stepDescriptions = useMemo(() => {
    if (!migration) return new Map<string, string>();
    return new Map(
      migration.steps.filter((s) => s.description).map((s) => [s.name, s.description ?? ""]),
    );
  }, [migration]);

  // Map stepName → file URL list from the registered migration
  const stepFiles = useMemo(() => {
    if (!migration) return new Map<string, string[]>();
    return new Map(migration.steps.filter((s) => s.files?.length).map((s) => [s.name, s.files ?? []]));
  }, [migration]);

  const handleExecute = useCallback(async () => {
    try {
      await executeRun(id);
      // Clear queued state and start polling for the workflow.
      setRunInfo(null);
      startPolling(poll);
    } catch (e) {
      if (e instanceof ConflictError) {
        void poll();
      } else {
        toast.error(e instanceof Error ? e.message : "Failed to execute");
      }
    }
  }, [id, poll, startPolling]);

  const handleDequeue = useCallback(async () => {
    try {
      await dequeueRun(id);
      router.push(ROUTES.migrationDetail(registrationId));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to remove from queue");
    }
  }, [id, registrationId, router]);

  // Unique candidates involved in this run
  const candidates = useMemo(() => {
    if (!status?.result?.results.length) return [];
    const seen = new Set<string>();
    return status.result.results
      .map((r) => r.candidate)
      .filter((c) => {
        if (seen.has(c.id)) return false;
        seen.add(c.id);
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
      {/* Queued run preview — workflow not started yet */}
      {runInfo?.status === "queued" ? (
        <QueuedRunView
          runInfo={runInfo}
          migration={migration}
          runId={id}
          onExecute={handleExecute}
          onDequeue={handleDequeue}
        />
      ) : notFound ? (
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
          {/* Header — single line: back link · targets · run id  [status] */}
          <div className="flex items-center gap-2 flex-wrap">
            <Link
              href={ROUTES.migrationDetail(registrationId)}
              className="inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-300 transition-colors shrink-0"
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M7 3L4 6l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              {migration?.name ?? registrationId}
            </Link>

            {candidates.length > 0 && <span className="text-zinc-700 select-none">·</span>}
            {candidates.map((t) => {
              const team = t.metadata?.team;
              const appName = t.id;
              return (
                <span
                  key={t.id}
                  className="inline-flex items-center gap-1.5 text-xs font-mono bg-zinc-800/60 border border-zinc-700/50 text-zinc-300 px-2 py-0.5 rounded-md"
                >
                  {team ? <span className="text-zinc-500">{team} /</span> : null}
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
                  <span className="text-xs font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
                    {status.result.results.length}
                  </span>
                ) : null}
              </div>
              <div className="border border-zinc-800/80 rounded-lg p-6 overflow-y-auto max-h-[44rem]">
                <StepTimeline
                  results={status.result?.results ?? []}
                  stepDescriptions={stepDescriptions}
                  stepFiles={stepFiles}
                  onComplete={(stepName, candidate, success) => {
                    void (async () => {
                      await completeStep(id, stepName, candidate, success);
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


function QueuedRunView({
  runInfo,
  migration,
  runId,
  onExecute,
  onDequeue,
}: {
  runInfo: RunInfo;
  migration: RegisteredMigration | null;
  runId: string;
  onExecute: () => Promise<void>;
  onDequeue: () => Promise<void>;
}) {
  const [executing, setExecuting] = useState(false);
  const [dequeuing, setDequeuing] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<DryRunResult | null>(null);
  const [dryRunLoading, setDryRunLoading] = useState(false);
  const [dryRunError, setDryRunError] = useState<string | null>(null);
  const hasDryRun = useRef(false);

  const registrationId = runId.split("__")[0] ?? runId;
  const team = runInfo.candidate.metadata?.team;
  const appName = runInfo.candidate.id;
  const steps = migration?.steps ?? [];

  // Auto-trigger dry run once migration definition is loaded.
  // We use hasDryRun.current ref to track whether we've already run, so we only list
  // migration.id in the dependency array to trigger once per migration. The values of
  // registrationId and runInfo.candidate don't change during the run.
  useEffect(
    () => {
      if (!migration || hasDryRun.current) return;
      hasDryRun.current = true;
      setDryRunLoading(true);
      dryRun(registrationId, runInfo.candidate)
        .then(setDryRunResult)
        .catch((e) => setDryRunError(e instanceof Error ? e.message : "Dry run failed"))
        .finally(() => setDryRunLoading(false));
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [migration?.id],
  );  

  const dryRunByStep = useMemo(() => {
    if (!dryRunResult) return new Map<string, DryRunResult["steps"][number]>();
    return new Map(dryRunResult.steps.map((s) => [s.stepName, s]));
  }, [dryRunResult]);

  async function handleExecute() {
    setExecuting(true);
    try {
      await onExecute();
    } finally {
      setExecuting(false);
    }
  }

  async function handleDequeue() {
    setDequeuing(true);
    try {
      await onDequeue();
    } finally {
      setDequeuing(false);
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-2 flex-wrap">
        <Link
          href={ROUTES.migrationDetail(registrationId)}
          className="inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-300 transition-colors shrink-0"
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M7 3L4 6l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          {migration?.name ?? registrationId}
        </Link>
        <span className="text-zinc-700 select-none">·</span>
        <span className="inline-flex items-center gap-1.5 text-xs font-mono bg-zinc-800/60 border border-zinc-700/50 text-zinc-300 px-2 py-0.5 rounded-md">
          {team ? <span className="text-zinc-500">{team} /</span> : null}
          {appName}
        </span>
        <span className="flex-1" />
        <span className="inline-flex items-center gap-1.5 text-xs font-medium px-2 py-0.5 rounded border bg-indigo-500/10 text-indigo-400 border-indigo-500/20">
          queued
        </span>
      </div>

      {/* Steps preview */}
      {steps.length > 0 ? (
        <section className="w-fit min-w-[700px] mx-auto">
          <div className="flex items-center gap-2 mb-4">
            <h2 className="text-sm font-medium text-zinc-400 uppercase tracking-widest">Steps</h2>
            <span className="text-xs font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
              {steps.length}
            </span>
            <span className="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full border bg-amber-500/10 text-amber-400 border-amber-500/20">
              <svg width="8" height="8" viewBox="0 0 16 16" fill="none">
                <path d="M8 1v7l4 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.5"/>
              </svg>
              Dry run
            </span>
            {dryRunLoading ? <span className="inline-flex items-center gap-1.5 text-xs text-zinc-500">
                <svg className="animate-spin w-3 h-3" viewBox="0 0 16 16" fill="none">
                  <circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="2" strokeDasharray="28" strokeDashoffset="10" strokeLinecap="round" />
                </svg>
                Simulating…
              </span> : null}
            {dryRunError ? <span className="text-xs text-red-400">Dry run failed</span> : null}
          </div>
          <div className="border border-zinc-800/80 rounded-lg divide-y divide-zinc-800/60">
            {steps.map((step, i) => {
              const config = step.config ? Object.entries(step.config) : [];
              const instructions = step.config?.instructions;
              const otherConfig = config.filter(([k]) => k !== "instructions");
              const stepDryRun = dryRunByStep.get(step.name);

              return (
                <div key={step.name} className="px-5 py-4 space-y-2.5">
                  {/* Name row */}
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2.5">
                      <span className="text-xs font-mono text-zinc-600 tabular-nums w-5 shrink-0 select-none">
                        {String(i + 1).padStart(2, "0")}.
                      </span>
                      <span className="text-base font-medium font-mono text-zinc-100">{step.name}</span>
                    </div>
                    <span className="text-xs font-mono text-zinc-500 bg-zinc-800/60 px-2 py-0.5 rounded shrink-0">
                      {step.workerApp}
                    </span>
                  </div>

                  {/* Description */}
                  {Boolean(step.description) && (
                    <p className="text-sm text-zinc-400 ml-7">{step.description}</p>
                  )}

                  {/* Instructions (manual-review steps) */}
                  {instructions ? <div className="ml-7 bg-blue-500/5 border border-blue-500/15 rounded-md px-3 py-2.5">
                      <div className="text-xs font-medium text-blue-400/70 uppercase tracking-widest mb-2">
                        Instructions
                      </div>
                      <ul className="space-y-1">
                        {instructions.split("\n").map((line, j) => (
                          <li key={j} className="text-sm text-blue-200/80 font-mono">
                            {line}
                          </li>
                        ))}
                      </ul>
                    </div> : null}

                  {/* Other config chips */}
                  {otherConfig.length > 0 && (
                    <div className="ml-7 flex flex-wrap gap-1.5">
                      {otherConfig.map(([k, v]) => (
                        <span key={k} className="text-xs font-mono text-zinc-500 bg-zinc-800/40 px-1.5 py-0.5 rounded">
                          {k}=<span className="text-zinc-400">{v}</span>
                        </span>
                      ))}
                    </div>
                  )}

                  {/* Dry run results */}
                  {(dryRunLoading && !stepDryRun) || stepDryRun ? (
                    <div className="ml-7 space-y-2">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-medium text-zinc-600 uppercase tracking-widest">
                          Expected changes
                        </span>
                        <div className="flex-1 h-px bg-zinc-800/80" />
                      </div>
                      {dryRunLoading && !stepDryRun ? (
                        <div className="h-5 rounded bg-zinc-800/40 animate-pulse w-48" />
                      ) : (
                        stepDryRun && <DryRunStepResult result={stepDryRun} />
                      )}
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        </section>
      ) : (
        <div className="text-sm text-zinc-600 italic">Loading step definitions…</div>
      )}

      {/* Execute / Cancel actions */}
      <div className="w-fit min-w-[700px] mx-auto flex items-center justify-between pt-2 pb-6">
        <button
          onClick={() => void handleDequeue()}
          disabled={dequeuing || executing}
          className="text-sm text-zinc-500 hover:text-red-400 transition-colors disabled:opacity-40"
        >
          {dequeuing ? "Removing…" : "Remove from queue"}
        </button>
        <button
          onClick={() => void handleExecute()}
          disabled={executing || dequeuing}
          className="inline-flex items-center gap-2 px-5 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {executing ? (
            <>
              <svg className="animate-spin w-4 h-4" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="2" strokeDasharray="28" strokeDashoffset="10" strokeLinecap="round" />
              </svg>
              Starting…
            </>
          ) : (
            <>
              <svg width="14" height="14" viewBox="0 0 12 12" fill="none">
                <path d="M3 2l7 4-7 4V2z" fill="currentColor" />
              </svg>
              Execute
            </>
          )}
        </button>
      </div>
    </div>
  );
}

// --- Dry run helpers ---

type DiffLine = { type: "add" | "remove" | "context"; text: string };

function computeDiff(before: string, after: string): DiffLine[] {
  if (!before) {
    return after ? after.split("\n").map((text) => ({ type: "add", text })) : [];
  }
  const a = before.split("\n");
  const b = after.split("\n");
  const m = a.length;
  const n = b.length;
  const dp = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = 1; i <= m; i++)
    for (let j = 1; j <= n; j++)
      dp[i][j] = a[i - 1] === b[j - 1] ? dp[i - 1][j - 1] + 1 : Math.max(dp[i - 1][j], dp[i][j - 1]);

  const result: DiffLine[] = [];
  let i = m, j = n;
  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && a[i - 1] === b[j - 1]) {
      result.push({ type: "context", text: a[i - 1] }); i--; j--;
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      result.push({ type: "add", text: b[j - 1] }); j--;
    } else {
      result.push({ type: "remove", text: a[i - 1] }); i--;
    }
  }
  return result.reverse();
}

const CONTEXT_LINES = 2;

function collapseContext(lines: DiffLine[]): Array<DiffLine | { type: "ellipsis"; count: number }> {
  const show = new Array(lines.length).fill(false);
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].type !== "context") {
      for (let k = Math.max(0, i - CONTEXT_LINES); k <= Math.min(lines.length - 1, i + CONTEXT_LINES); k++)
        show[k] = true;
    }
  }
  const result: Array<DiffLine | { type: "ellipsis"; count: number }> = [];
  let skip = 0;
  for (let i = 0; i < lines.length; i++) {
    if (show[i]) {
      if (skip > 0) { result.push({ type: "ellipsis", count: skip }); skip = 0; }
      result.push(lines[i]);
    } else {
      skip++;
    }
  }
  if (skip > 0) result.push({ type: "ellipsis", count: skip });
  return result;
}

function FileDiffView({ diff }: { diff: FileDiff }) {
  const lines = computeDiff(diff.before ?? "", diff.after);
  const collapsed = collapseContext(lines);

  return (
    <div className="border border-zinc-800/60 rounded-md overflow-hidden text-xs font-mono">
      {/* File header */}
      <div className="flex items-center gap-2 px-3 py-1.5 bg-zinc-900/80 border-b border-zinc-800/60">
        <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor" className="text-zinc-500 shrink-0">
          <path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z" />
        </svg>
        <span className="text-zinc-300 flex-1 truncate">{diff.path}</span>
        <span className="text-zinc-600 shrink-0">{diff.repo}</span>
        {diff.status === "new" ? <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            new
          </span> : diff.status === "deleted" ? <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-red-500/10 text-red-400 border border-red-500/20">
            deleted
          </span> : null}
      </div>
      {/* Diff lines */}
      <div className="overflow-x-auto">
        {collapsed.map((line, idx) => {
          if (line.type === "ellipsis") {
            return (
              <div key={idx} className="px-3 py-0.5 text-zinc-600 bg-zinc-900/30 select-none">
                ··· {line.count} unchanged {line.count === 1 ? "line" : "lines"}
              </div>
            );
          }
          const bg = line.type === "add" ? "bg-emerald-500/10" : line.type === "remove" ? "bg-red-500/10" : "";
          const color = line.type === "add" ? "text-emerald-300" : line.type === "remove" ? "text-red-300" : "text-zinc-500";
          const prefix = line.type === "add" ? "+" : line.type === "remove" ? "-" : " ";
          return (
            <div key={idx} className={`flex ${bg} px-3 py-px`}>
              <span className={`w-4 shrink-0 select-none ${color} opacity-60`}>{prefix}</span>
              <span className={`${color} whitespace-pre`}>{line.text}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function DryRunStepResult({ result }: { result: { stepName: string; skipped: boolean; error?: string; files?: FileDiff[] } }) {
  if (result.skipped) {
    return (
      <div className="ml-7 text-xs text-zinc-600 italic">Skipped — handled by another worker</div>
    );
  }
  if (result.error) {
    return (
      <div className="ml-7 text-xs font-mono text-red-400 bg-red-500/8 border border-red-500/20 rounded-md px-3 py-2">
        {result.error}
      </div>
    );
  }
  if (!result.files?.length) {
    return <div className="ml-7 text-xs text-zinc-600 italic">No file changes</div>;
  }
  return (
    <div className="ml-7 space-y-2">
      {result.files.map((f, i) => (
        <FileDiffView key={i} diff={f} />
      ))}
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
