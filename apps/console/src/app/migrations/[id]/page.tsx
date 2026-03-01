"use client";

import { useCallback, useEffect, useState } from "react";
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
import {
  Button,
  Input,
  Skeleton,
  Tooltip,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
} from "@/components/ui";

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

  if (error && !migration) {
    return (
      <div className="space-y-6 animate-fade-in-up">
        <Link
          href={ROUTES.migrations}
          className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground/80 transition-colors"
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
        <div className="bg-destructive/8 border border-destructive/20 rounded-lg px-4 py-3 text-sm text-destructive">
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
        <Skeleton className="h-3 w-full bg-card/50" />
      </div>
    );
  }

  return (
    <div className="space-y-8 animate-fade-in-up">
      {/* Steps modal */}
      <Dialog open={stepsOpen} onOpenChange={setStepsOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Workflow definition</DialogTitle>
            <DialogDescription>
              {migration.steps.length} steps — applies to all {kindPlural}
            </DialogDescription>
          </DialogHeader>
          <div className="px-5 py-4 space-y-0">
            {migration.steps.map((step, i) => (
              <div key={step.name} className="flex gap-3">
                <div className="flex flex-col items-center w-6 shrink-0">
                  <div className="w-6 h-6 rounded-md bg-muted border border-border-hover/50 flex items-center justify-center text-xs font-mono font-medium text-muted-foreground">
                    {i + 1}
                  </div>
                  {i < migration.steps.length - 1 && (
                    <div className="w-px flex-1 bg-border my-0.5" />
                  )}
                </div>
                <div className="flex-1 pb-3 min-w-0">
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="font-medium text-sm text-foreground">{step.name}</span>
                    <span className="text-xs font-mono text-muted-foreground/70 bg-muted px-1.5 py-0.5 rounded shrink-0">
                      {step.migratorApp}
                    </span>
                  </div>
                  {Boolean(step.description) && (
                    <p className="text-xs text-muted-foreground mt-0.5">{step.description}</p>
                  )}
                </div>
              </div>
            ))}
          </div>
        </DialogContent>
      </Dialog>

      {/* Cancel confirmation modal */}
      <Dialog
        open={!!cancelCandidate}
        onOpenChange={(open) => { if (!open && !cancelling) setCancelCandidate(null); }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Cancel migration?</DialogTitle>
          </DialogHeader>
          <div className="px-5 py-4">
            <p className="text-sm text-muted-foreground">
              Stop the running migration for{" "}
              <span className="font-mono text-foreground">{cancelCandidate?.id}</span>? This will
              reset it to not started.
            </p>
            {cancelError ? <p className="mt-3 text-sm text-destructive">{cancelError}</p> : null}
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setCancelCandidate(null)}
              disabled={cancelling}
            >
              Keep running
            </Button>
            <Button
              variant="danger"
              size="sm"
              onClick={() => void confirmCancel()}
              disabled={cancelling}
            >
              {cancelling ? "Cancelling…" : "Cancel run"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Preview inputs modal */}
      <Dialog open={!!previewModal} onOpenChange={(open) => { if (!open) setPreviewModal(null); }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <div className="flex items-center gap-2">
              <DialogTitle>Required inputs</DialogTitle>
              <Tooltip content="These values are required to migrate this candidate. They are passed to each step at runtime.">
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" className="text-muted-foreground/70 hover:text-muted-foreground cursor-default transition-colors">
                  <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.5" />
                  <path d="M8 7v5M8 5h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                </svg>
              </Tooltip>
            </div>
          </DialogHeader>
          <div className="px-5 py-5 space-y-4">
            {(migration.requiredInputs ?? []).map((inp, i) => (
              <div key={inp.name} className="space-y-1.5">
                <label
                  htmlFor={`preview-modal-${inp.name}`}
                  className="text-xs font-medium text-muted-foreground"
                >
                  {inp.label}
                </label>
                <Input
                  id={`preview-modal-${inp.name}`}
                  type="text"
                  value={previewModal?.inputs[inp.name] ?? ""}
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
                  <p className="text-xs text-muted-foreground/70 italic">{inp.description}</p>
                ) : null}
              </div>
            ))}
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPreviewModal(null)}
            >
              Cancel
            </Button>
            <Button
              disabled={!(migration.requiredInputs ?? []).every((inp) => previewModal?.inputs[inp.name]?.trim())}
              onClick={() => {
                if (!previewModal) return;
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
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Breadcrumb */}
      <Link
        href={ROUTES.migrations}
        className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground/80 transition-colors"
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
            <h2 className="text-xl font-semibold tracking-tight text-foreground">{migration.name}</h2>
            <button
              onClick={() => setStepsOpen(true)}
              className="text-xs text-muted-foreground hover:text-foreground/80 transition-colors underline underline-offset-2 decoration-border hover:decoration-border-hover shrink-0"
            >
              {migration.steps.length} steps
            </button>
          </div>
          <p className="text-sm text-muted-foreground mt-2 leading-relaxed">
            {migration.description}
          </p>
        </div>
      </div>

      {/* Progress bar */}
      <ProgressBar candidates={candidates} />

      {/* Candidates table */}
      <section>
        <div className="flex items-center gap-2 mb-4">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-widest">
            {kindPluralCap}
          </h3>
          <span className="text-xs font-mono text-muted-foreground/70 bg-muted px-1.5 py-0.5 rounded">
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
