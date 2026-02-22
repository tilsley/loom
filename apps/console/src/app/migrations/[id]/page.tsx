"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import {
  getMigration,
  getCandidates,
  deleteMigration,
  queueRun,
  dequeueRun,
  ConflictError,
  type RegisteredMigration,
  type Candidate,
  type CandidateRun,
  type CandidateWithStatus,
} from "@/lib/api";
import { useRole } from "@/contexts/role-context";
import { ROUTES } from "@/lib/routes";
import { ProgressBar } from "@/components/progress-bar";
import { CandidateTable } from "@/components/candidate-table";
import { Button, Skeleton } from "@/components/ui";

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { isAdmin } = useRole();
  const [migration, setMigration] = useState<RegisteredMigration | null>(null);
  const [candidates, setCandidates] = useState<CandidateWithStatus[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [runningCandidate, setRunningCandidate] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchMigration = useCallback(async () => {
    try {
      const data = await getMigration(id);
      setMigration(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, [id]);

  const fetchCandidates = useCallback(async () => {
    try {
      const data = await getCandidates(id);
      setCandidates(data);
    } catch {
      // Silently ignore — migration may not have candidates yet
    }
  }, [id]);

  // Initial load + 5s polling for both migration metadata and candidates
  useEffect(() => {
    void fetchMigration();
    void fetchCandidates();
    const interval = setInterval(() => {
      void fetchMigration();
      void fetchCandidates();
    }, 5000);
    return () => clearInterval(interval);
  }, [fetchMigration, fetchCandidates]);

  // Derive Candidate[] and CandidateRun map from candidates for CandidateTable / ProgressBar
  const derivedCandidates = useMemo<Candidate[]>(
    () => candidates.map((c) => ({ id: c.id, kind: c.kind, metadata: c.metadata, state: c.state, files: c.files })),
    [candidates],
  );

  const derivedCandidateRuns = useMemo<Record<string, CandidateRun>>(() => {
    const runs: Record<string, CandidateRun> = {};
    for (const c of candidates) {
      if (c.status !== "not_started") {
        runs[c.id] = {
          runId: c.runId ?? "",
          status: c.status as CandidateRun["status"],
        };
      }
    }
    return runs;
  }, [candidates]);

  async function handleQueue(candidate: Candidate) {
    setRunningCandidate(candidate.id);
    try {
      const { runId } = await queueRun(id, candidate);
      router.push(ROUTES.runDetail(runId));
    } catch (e) {
      if (e instanceof ConflictError) {
        await Promise.all([fetchMigration(), fetchCandidates()]);
        toast.error("Candidate already queued, running, or completed");
      } else {
        toast.error(e instanceof Error ? e.message : "Failed to queue run");
      }
      setRunningCandidate(null);
    }
  }

  async function handleDequeue(runId: string) {
    setRunningCandidate(runId);
    try {
      await dequeueRun(runId);
      await Promise.all([fetchMigration(), fetchCandidates()]);
      toast.success("Removed from queue");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to dequeue");
    } finally {
      setRunningCandidate(null);
    }
  }

  async function handleDelete() {
    if (!confirm("Delete this migration? This cannot be undone.")) return;
    setDeleting(true);
    try {
      await deleteMigration(id);
      toast.success("Migration deleted");
      router.push(ROUTES.migrations);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete");
      setDeleting(false);
    }
  }

  if (error && !migration) {
    return (
      <div className="space-y-4 animate-fade-in-up">
        <Link
          href={ROUTES.migrations}
          className="inline-flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path
              d="M7 3L4 6l3 3"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          Migrations
        </Link>
        <div className="bg-red-500/8 border border-red-500/20 rounded-lg px-4 py-3 text-[13px] text-red-400">
          {error}
        </div>
      </div>
    );
  }

  if (!migration) {
    return (
      <div className="space-y-6 animate-fade-in-up">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-3 w-full bg-zinc-900/50" />
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in-up">
      {/* Breadcrumb */}
      <Link
        href={ROUTES.migrations}
        className="inline-flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
          <path
            d="M7 3L4 6l3 3"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        Migrations
      </Link>

      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h2 className="text-xl font-semibold tracking-tight text-zinc-50">{migration.name}</h2>
          {Boolean(migration.description) && (
            <p className="text-[13px] text-zinc-400 mt-1.5 leading-relaxed">
              {migration.description}
            </p>
          )}
          <p className="text-[11px] font-mono text-zinc-600 mt-2">{migration.id}</p>
        </div>
        <div className="flex gap-2 shrink-0">
          {isAdmin ? (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => void handleDelete()}
              disabled={deleting}
            >
              {deleting ? "..." : "Delete"}
            </Button>
          ) : null}
        </div>
      </div>

      {/* Progress bar */}
      <ProgressBar candidates={derivedCandidates} candidateRuns={derivedCandidateRuns} />

      {/* Candidates table */}
      <section>
        <div className="flex items-center gap-2 mb-3">
          <h3 className="text-xs font-medium text-zinc-500 uppercase tracking-widest">
            Candidates
          </h3>
          <span className="text-[10px] font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
            {candidates.length}
          </span>
        </div>
        <CandidateTable
          migration={{ ...migration, candidates: derivedCandidates, candidateRuns: derivedCandidateRuns }}
          onQueue={handleQueue}
          onDequeue={handleDequeue}
          runningCandidate={runningCandidate}
        />
      </section>

      {/* Steps pipeline — collapsible */}
      <details className="group">
        <summary className="flex items-center gap-2 cursor-pointer list-none">
          <svg
            width="14"
            height="14"
            viewBox="0 0 14 14"
            fill="none"
            className="text-zinc-600 group-open:rotate-90 transition-transform"
          >
            <path
              d="M5 3l4 4-4 4"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          <h3 className="text-xs font-medium text-zinc-500 uppercase tracking-widest">Steps</h3>
          <span className="text-[10px] font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
            {migration.steps.length}
          </span>
        </summary>
        <div className="mt-3 space-y-0">
          {migration.steps.map((step, i) => (
            <div key={step.name} className="flex gap-3">
              <div className="flex flex-col items-center w-6 shrink-0">
                <div className="w-6 h-6 rounded-md bg-zinc-800 border border-zinc-700/50 flex items-center justify-center text-[10px] font-mono font-medium text-zinc-400">
                  {i + 1}
                </div>
                {i < migration.steps.length - 1 && (
                  <div className="w-px flex-1 bg-zinc-800 my-0.5" />
                )}
              </div>
              <div className="flex-1 pb-3 min-w-0">
                <div className="bg-zinc-900/50 border border-zinc-800/80 rounded-lg p-3">
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="font-medium text-[13px] text-zinc-200">{step.name}</span>
                    <span className="text-[10px] font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
                      {step.workerApp}
                    </span>
                  </div>
                  {Boolean(step.description) && (
                    <p className="text-[12px] text-zinc-500 mt-1">{step.description}</p>
                  )}
                  {step.files && step.files.length > 0 ? (
                    <div className="flex flex-wrap gap-x-3 gap-y-0.5 mt-1.5">
                      {step.files.map((url) => {
                        const label = url.split("/blob/main/").pop() ?? url;
                        return (
                          <span
                            key={url}
                            className="inline-flex items-center gap-1 text-[11px] font-mono text-zinc-600"
                            title={url}
                          >
                            <svg
                              width="10"
                              height="10"
                              viewBox="0 0 16 16"
                              fill="currentColor"
                              className="shrink-0"
                            >
                              <path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z" />
                            </svg>
                            {label}
                          </span>
                        );
                      })}
                    </div>
                  ) : null}
                  {step.config && Object.keys(step.config).length > 0 ? (
                    <div className="flex gap-2 mt-2">
                      {Object.entries(step.config).map(([k, v]) => (
                        <span
                          key={k}
                          className="text-[10px] font-mono text-zinc-500 bg-zinc-800/40 px-1.5 py-0.5 rounded"
                        >
                          {k}=<span className="text-zinc-400">{v}</span>
                        </span>
                      ))}
                    </div>
                  ) : null}
                </div>
              </div>
            </div>
          ))}
        </div>
      </details>

      {/* Run history — collapsible */}
      <details className="group">
        <summary className="flex items-center gap-2 cursor-pointer list-none">
          <svg
            width="14"
            height="14"
            viewBox="0 0 14 14"
            fill="none"
            className="text-zinc-600 group-open:rotate-90 transition-transform"
          >
            <path
              d="M5 3l4 4-4 4"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          <h3 className="text-xs font-medium text-zinc-500 uppercase tracking-widest">
            Run History
          </h3>
          <span className="text-[10px] font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
            {migration.runIds.length}
          </span>
        </summary>
        {migration.runIds.length === 0 ? (
          <p className="text-[13px] text-zinc-600 mt-3 italic">
            No runs yet. Click &quot;Run&quot; to start one.
          </p>
        ) : (
          <div className="mt-3 space-y-1.5">
            {[...migration.runIds].reverse().map((runId, i) => (
              <Link
                key={runId}
                href={ROUTES.runDetail(runId)}
                className="group/run flex items-center justify-between bg-zinc-900/50 border border-zinc-800/80 hover:border-zinc-700 rounded-lg px-3 py-2.5 transition-all"
              >
                <div className="flex items-center gap-2.5 min-w-0">
                  <div className="w-1.5 h-1.5 rounded-full bg-zinc-600 group-hover/run:bg-teal-400 transition-colors" />
                  <span className="text-[13px] font-mono text-zinc-300 truncate">{runId}</span>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {i === 0 && (
                    <span className="text-[10px] text-zinc-600 uppercase tracking-wider">
                      latest
                    </span>
                  )}
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 14 14"
                    fill="none"
                    className="text-zinc-700 group-hover/run:text-zinc-400 transition-colors"
                  >
                    <path
                      d="M5 3l4 4-4 4"
                      stroke="currentColor"
                      strokeWidth="1.5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </div>
              </Link>
            ))}
          </div>
        )}
      </details>
    </div>
  );
}
