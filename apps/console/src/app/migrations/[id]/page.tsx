"use client";

import { useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { useParams, usePathname, useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import {
  getMigration,
  getCandidates,
  cancelRun,
  type Migration,
  type Candidate,
} from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { ProgressBar } from "@/components/progress-bar";
import { CandidateTable } from "@/components/candidate-table";
import { Button, Input, Skeleton, Tooltip } from "@/components/ui";

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [migration, setMigration] = useState<Migration | null>(null);
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [stepsOpen, setStepsOpen] = useState(false);
  const [previewModal, setPreviewModal] = useState<{ candidate: Candidate; inputs: Record<string, string> } | null>(null);
  const [cancelCandidate, setCancelCandidate] = useState<Candidate | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const [cancelError, setCancelError] = useState<string | null>(null);

  // URL-persisted filter state
  const filter = searchParams.get("status") ?? "all";

  function updateParam(key: string, value: string | null) {
    const params = new URLSearchParams(searchParams.toString());
    if (!value || value === "all") {
      params.delete(key);
    } else {
      params.set(key, value);
    }
    const qs = params.toString();
    router.replace(`${pathname}${qs ? "?" + qs : ""}`, { scroll: false });
  }

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
      setCandidates(data ?? []);
    } catch {
      // Silently ignore — migration may not have candidates yet
    }
  }, [id]);

  // Dynamic polling: 2s when any candidate is running, 5s otherwise
  const hasRunning = candidates.some((c) => c.status === "running");

  useEffect(() => {
    void fetchMigration();
    void fetchCandidates();
    const interval = setInterval(() => {
      void fetchMigration();
      void fetchCandidates();
    }, hasRunning ? 2000 : 5000);
    return () => clearInterval(interval);
  }, [fetchMigration, fetchCandidates, hasRunning]);

  const kindPlural = (candidates[0]?.kind ?? "candidate") + "s";
  const kindPluralCap = kindPlural.charAt(0).toUpperCase() + kindPlural.slice(1);

  function handlePreview(candidate: Candidate) {
    const required = migration?.requiredInputs ?? [];
    if (required.length > 0) {
      const prefilled: Record<string, string> = {};
      for (const inp of required) {
        prefilled[inp.name] = candidate.metadata?.[inp.name] ?? "";
      }
      setPreviewModal({ candidate, inputs: prefilled });
    } else {
      router.push(ROUTES.preview(id, candidate.id));
    }
  }

  function handleCancel(candidate: Candidate) {
    setCancelCandidate(candidate);
    setCancelError(null);
  }

  async function confirmCancel() {
    if (!cancelCandidate) return;
    setCancelling(true);
    setCancelError(null);
    try {
      await cancelRun(id, cancelCandidate.id);
      setCancelCandidate(null);
      void fetchCandidates();
    } catch (e) {
      setCancelError(e instanceof Error ? e.message : "Failed to cancel");
    } finally {
      setCancelling(false);
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
                    {migration.steps.length} steps — applies to all {kindPlural}
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
                          {step.migratorApp}
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

  const cancelModal = cancelCandidate
    ? createPortal(
        <div
          className="fixed inset-0 z-50 overflow-y-auto bg-black/60 backdrop-blur-sm"
          onClick={() => { if (!cancelling) setCancelCandidate(null); }}
        >
          <div className="flex min-h-full items-center justify-center p-4">
            <div
              className="relative w-full max-w-md bg-[var(--color-surface)] border border-zinc-800 rounded-xl shadow-2xl"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center justify-between px-5 py-4 border-b border-zinc-800">
                <h3 className="text-[14px] font-semibold text-zinc-100">Cancel migration?</h3>
                <button
                  onClick={() => { if (!cancelling) setCancelCandidate(null); }}
                  className="text-zinc-600 hover:text-zinc-300 transition-colors"
                >
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                  </svg>
                </button>
              </div>
              <div className="px-5 py-4">
                <p className="text-sm text-zinc-400">
                  Stop the running migration for{" "}
                  <span className="font-mono text-zinc-200">{cancelCandidate.id}</span>? This will
                  reset it to not started.
                </p>
                {cancelError ? <p className="mt-3 text-sm text-red-400">{cancelError}</p> : null}
              </div>
              <div className="flex items-center justify-end gap-3 px-5 py-4 border-t border-zinc-800">
                <button
                  onClick={() => setCancelCandidate(null)}
                  disabled={cancelling}
                  className="text-sm text-zinc-500 hover:text-zinc-300 transition-colors disabled:opacity-50"
                >
                  Keep running
                </button>
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => void confirmCancel()}
                  disabled={cancelling}
                >
                  {cancelling ? "Cancelling…" : "Cancel run"}
                </Button>
              </div>
            </div>
          </div>
        </div>,
        document.body,
      )
    : null;

  const previewInputModal = previewModal && migration
    ? createPortal(
        <div
          className="fixed inset-0 z-50 overflow-y-auto bg-black/60 backdrop-blur-sm"
          onClick={() => setPreviewModal(null)}
        >
          <div className="flex min-h-full items-center justify-center p-4">
            <div
              className="relative w-full max-w-md bg-[var(--color-surface)] border border-zinc-800 rounded-xl shadow-2xl"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center justify-between px-5 py-4 border-b border-zinc-800">
                <div className="flex items-center gap-2">
                  <h3 className="text-[14px] font-semibold text-zinc-100">Required inputs</h3>
                  <Tooltip content="These values are required to migrate this candidate. They are passed to each step at runtime.">
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" className="text-zinc-600 hover:text-zinc-400 cursor-default transition-colors">
                      <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.5" />
                      <path d="M8 7v5M8 5h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                    </svg>
                  </Tooltip>
                </div>
                <button
                  onClick={() => setPreviewModal(null)}
                  className="text-zinc-600 hover:text-zinc-300 transition-colors"
                >
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                  </svg>
                </button>
              </div>
              <div className="px-5 py-5 space-y-4">
                {(migration.requiredInputs ?? []).map((inp, i) => (
                  <div key={inp.name} className="space-y-1.5">
                    <label
                      htmlFor={`preview-modal-${inp.name}`}
                      className="text-xs font-medium text-zinc-400"
                    >
                      {inp.label}
                    </label>
                    <Input
                      id={`preview-modal-${inp.name}`}
                      type="text"
                      value={previewModal.inputs[inp.name] ?? ""}
                      onChange={(e) =>
                        setPreviewModal((prev) =>
                          prev ? { ...prev, inputs: { ...prev.inputs, [inp.name]: e.target.value } } : null,
                        )
                      }
                      placeholder={inp.label}
                      className="font-mono"
                      autoFocus={i === 0}
                    />
                    {inp.description ? (
                      <p className="text-xs text-zinc-600 italic">{inp.description}</p>
                    ) : null}
                  </div>
                ))}
              </div>
              <div className="flex items-center justify-end gap-3 px-5 py-4 border-t border-zinc-800">
                <button
                  onClick={() => setPreviewModal(null)}
                  className="text-sm text-zinc-500 hover:text-zinc-300 transition-colors"
                >
                  Cancel
                </button>
                <Button
                  disabled={!(migration.requiredInputs ?? []).every((inp) => previewModal.inputs[inp.name]?.trim())}
                  onClick={() => {
                    const params = new URLSearchParams(previewModal.inputs);
                    router.push(`${ROUTES.preview(id, previewModal.candidate.id)}?${params.toString()}`);
                    setPreviewModal(null);
                  }}
                >
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none">
                    <path d="M8 1v7l4 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.5" />
                  </svg>
                  Run preview
                </Button>
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
      {cancelModal}
      {previewInputModal}

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
          <p className="text-sm text-zinc-400 mt-2 leading-relaxed">
            {migration.description}
          </p>
        </div>
      </div>

      {/* Progress bar */}
      <ProgressBar candidates={candidates} />

      {/* Candidates table */}
      <section>
        <div className="flex items-center gap-2 mb-4">
          <h3 className="text-xs font-medium text-zinc-500 uppercase tracking-widest">
            {kindPluralCap}
          </h3>
          <span className="text-xs font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
            {candidates.length}
          </span>
        </div>
        <CandidateTable
          migration={{ ...migration, candidates }}
          onPreview={handlePreview}
          onCancel={handleCancel}
          runningCandidate={null}
          filter={filter}
          onFilterChange={(v) => updateParam("status", v === "all" ? null : v)}
        />
      </section>
    </div>
  );
}
