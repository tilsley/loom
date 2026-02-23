"use client";

import { useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  getMigration,
  getCandidates,
  type Migration,
  type Candidate,
} from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { ProgressBar } from "@/components/progress-bar";
import { CandidateTable } from "@/components/candidate-table";
import { Input, Skeleton } from "@/components/ui";

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [migration, setMigration] = useState<Migration | null>(null);
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [stepsOpen, setStepsOpen] = useState(false);
  const [previewCandidate, setPreviewCandidate] = useState<Candidate | null>(null);
  const [previewInputs, setPreviewInputs] = useState<Record<string, string>>({});

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

  const kindPlural = (candidates[0]?.kind ?? "candidate") + "s";
  const kindPluralCap = kindPlural.charAt(0).toUpperCase() + kindPlural.slice(1);

  function handlePreview(candidate: Candidate) {
    const required = migration?.requiredInputs ?? [];
    if (required.length > 0) {
      const prefilled: Record<string, string> = {};
      for (const inp of required) {
        prefilled[inp.name] = candidate.metadata?.[inp.name] ?? "";
      }
      setPreviewInputs(prefilled);
      setPreviewCandidate(candidate);
    } else {
      router.push(ROUTES.preview(id, candidate.id));
    }
  }

  const requiredInputs = migration?.requiredInputs ?? [];
  const previewModal = previewCandidate && migration
    ? createPortal(
        <div
          className="fixed inset-0 z-50 overflow-y-auto bg-black/60 backdrop-blur-sm"
          onClick={() => setPreviewCandidate(null)}
        >
          <div className="flex min-h-full items-center justify-center p-4">
            <div
              className="relative w-full max-w-md bg-[var(--color-surface)] border border-zinc-800 rounded-xl shadow-2xl"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center justify-between px-5 py-4 border-b border-zinc-800">
                <div>
                  <h3 className="text-[14px] font-semibold text-zinc-100">Preview inputs</h3>
                  <p className="text-xs text-zinc-500 mt-0.5 font-mono">{previewCandidate.id}</p>
                </div>
                <button
                  onClick={() => setPreviewCandidate(null)}
                  className="text-zinc-600 hover:text-zinc-300 transition-colors"
                >
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                  </svg>
                </button>
              </div>
              <div className="px-5 py-4 space-y-4">
                {requiredInputs.map((inp) => (
                  <div key={inp.name}>
                    <label className="block text-xs font-medium text-zinc-500 uppercase tracking-widest mb-2">
                      {inp.label}
                    </label>
                    <Input
                      type="text"
                      value={previewInputs[inp.name] ?? ""}
                      onChange={(e) => setPreviewInputs((v) => ({ ...v, [inp.name]: e.target.value }))}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" && requiredInputs.every((i) => previewInputs[i.name]?.trim())) {
                          const params = new URLSearchParams(previewInputs);
                          router.push(ROUTES.preview(id, previewCandidate.id) + "?" + params.toString());
                          setPreviewCandidate(null);
                        }
                      }}
                      placeholder={inp.label}
                      className="font-mono"
                      autoFocus={requiredInputs[0]?.name === inp.name}
                    />
                  </div>
                ))}
              </div>
              <div className="flex items-center justify-end gap-3 px-5 py-4 border-t border-zinc-800">
                <button
                  onClick={() => setPreviewCandidate(null)}
                  className="text-sm text-zinc-500 hover:text-zinc-300 transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    const params = new URLSearchParams(previewInputs);
                    router.push(ROUTES.preview(id, previewCandidate.id) + "?" + params.toString());
                    setPreviewCandidate(null);
                  }}
                  disabled={requiredInputs.some((inp) => !previewInputs[inp.name]?.trim())}
                  className="inline-flex items-center gap-2 px-4 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Preview
                </button>
              </div>
            </div>
          </div>
        </div>,
        document.body,
      )
    : null;

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
      {previewModal}
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
          runningCandidate={null}
        />
      </section>
    </div>
  );
}
