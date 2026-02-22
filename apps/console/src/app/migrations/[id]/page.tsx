"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import {
  getMigration,
  getCandidates,
  deleteMigration,
  type RegisteredMigration,
  type Candidate,
  type CandidateRun,
  type CandidateWithStatus,
} from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { ProgressBar } from "@/components/progress-bar";
import { CandidateTable } from "@/components/candidate-table";
import { Button, Skeleton } from "@/components/ui";

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [migration, setMigration] = useState<RegisteredMigration | null>(null);
  const [candidates, setCandidates] = useState<CandidateWithStatus[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [stepsOpen, setStepsOpen] = useState(false);

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
          status: c.status as CandidateRun["status"],
        };
      }
    }
    return runs;
  }, [candidates]);

  function handlePreview(candidate: Candidate) {
    router.push(ROUTES.preview(id, candidate.id));
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

  const stepsModal = stepsOpen && migration
    ? createPortal(
        <div
          className="fixed inset-0 z-50 overflow-y-auto bg-black/60 backdrop-blur-sm"
          onClick={() => setStepsOpen(false)}
        >
          <div className="flex min-h-full items-center justify-center p-4">
            <div
              className="relative w-full max-w-lg bg-[var(--color-surface)] border border-zinc-800 rounded-xl shadow-2xl"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center justify-between px-5 py-4 border-b border-zinc-800">
                <div>
                  <h3 className="text-[14px] font-semibold text-zinc-100">Workflow definition</h3>
                  <p className="text-xs text-zinc-500 mt-0.5">
                    {migration.steps.length} steps — applies to all candidates
                  </p>
                </div>
                <button
                  onClick={() => setStepsOpen(false)}
                  className="text-zinc-600 hover:text-zinc-300 transition-colors"
                >
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                  </svg>
                </button>
              </div>
              <div className="px-5 py-4 space-y-0">
                {migration.steps.map((step, i) => (
                  <div key={step.name} className="flex gap-3">
                    <div className="flex flex-col items-center w-6 shrink-0">
                      <div className="w-6 h-6 rounded-md bg-zinc-800 border border-zinc-700/50 flex items-center justify-center text-xs font-mono font-medium text-zinc-400">
                        {i + 1}
                      </div>
                      {i < migration.steps.length - 1 && (
                        <div className="w-px flex-1 bg-zinc-800 my-0.5" />
                      )}
                    </div>
                    <div className="flex-1 pb-3 min-w-0">
                      <div className="flex items-baseline justify-between gap-2">
                        <span className="font-medium text-sm text-zinc-200">{step.name}</span>
                        <span className="text-xs font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded shrink-0">
                          {step.workerApp}
                        </span>
                      </div>
                      {Boolean(step.description) && (
                        <p className="text-xs text-zinc-500 mt-0.5">{step.description}</p>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>,
        document.body,
      )
    : null;

  if (error && !migration) {
    return (
      <div className="space-y-6 animate-fade-in-up">
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
        <div className="bg-red-500/8 border border-red-500/20 rounded-lg px-4 py-3 text-sm text-red-400">
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
    <div className="space-y-8 animate-fade-in-up">
      {stepsModal}
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
          <div className="flex items-center gap-3">
            <h2 className="text-xl font-semibold tracking-tight text-zinc-50">{migration.name}</h2>
            <button
              onClick={() => setStepsOpen(true)}
              className="text-xs text-zinc-500 hover:text-zinc-300 transition-colors underline underline-offset-2 decoration-zinc-700 hover:decoration-zinc-500 shrink-0"
            >
              {migration.steps.length} steps
            </button>
          </div>
          {Boolean(migration.description) && (
            <p className="text-sm text-zinc-400 mt-2 leading-relaxed">
              {migration.description}
            </p>
          )}
        </div>
        <div className="flex gap-2 shrink-0">
          <Button
            variant="destructive"
            size="sm"
            onClick={() => void handleDelete()}
            disabled={deleting}
          >
            {deleting ? "..." : "Delete"}
          </Button>
        </div>
      </div>

      {/* Progress bar */}
      <ProgressBar candidates={derivedCandidates} candidateRuns={derivedCandidateRuns} />

      {/* Candidates table */}
      <section>
        <div className="flex items-center gap-2 mb-4">
          <h3 className="text-xs font-medium text-zinc-500 uppercase tracking-widest">
            Candidates
          </h3>
          <span className="text-xs font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
            {candidates.length}
          </span>
        </div>
        <CandidateTable
          migration={{ ...migration, candidates: derivedCandidates, candidateRuns: derivedCandidateRuns }}
          onPreview={handlePreview}
          runningCandidate={null}
        />
      </section>
    </div>
  );
}
