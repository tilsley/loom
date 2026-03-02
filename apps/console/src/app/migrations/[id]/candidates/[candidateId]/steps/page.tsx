"use client";

import { useEffect, useState, useCallback, useMemo, useRef } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import {
  getCandidateSteps,
  getCandidates,
  getMigration,
  completeStep,
  retryStep,
  updateInputs,
  type Candidate,
  type CandidateStepsResponse,
  type Migration,
} from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { buildStepDescriptionMap, calculateStepProgress, getApplicableSteps } from "@/lib/steps";
import { prefillInputs } from "@/lib/inputs";
import { StepTimeline } from "@/components/step-timeline";
import {
  Button,
  Input,
  Skeleton,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  Tooltip,
  buttonVariants,
} from "@/components/ui";

export default function CandidateStepsPage() {
  const { id, candidateId } = useParams<{ id: string; candidateId: string }>();
  const [stepsData, setStepsData] = useState<CandidateStepsResponse | null>(null);
  const [migration, setMigration] = useState<Migration | null>(null);
  const [candidate, setCandidate] = useState<Candidate | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notFound, setNotFound] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Input dialog state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [inputValues, setInputValues] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

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

  const fetchCandidate = useCallback(() => {
    getCandidates(id).then((candidates) => {
      const c = candidates.find((c) => c.id === candidateId);
      if (c) setCandidate(c);
    }).catch(() => {});
  }, [id, candidateId]);

  useEffect(() => {
    void poll();
    fetchCandidate();
    intervalRef.current = setInterval(() => {
      void poll();
      fetchCandidate();
    }, 2000);
    return stopPolling;
  }, [poll, fetchCandidate, stopPolling]);

  useEffect(() => {
    getMigration(id).then(setMigration).catch(() => {});
  }, [id]);

  const requiredInputs = migration?.requiredInputs;
  const hasRequiredInputs = requiredInputs && requiredInputs.length > 0;

  const openDialog = useCallback(() => {
    if (!requiredInputs || !candidate) return;
    setInputValues(prefillInputs(requiredInputs, candidate));
    setDialogOpen(true);
  }, [requiredInputs, candidate]);

  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      await updateInputs(id, candidateId, inputValues);
      setDialogOpen(false);
      toast.success("Inputs updated");
      fetchCandidate();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save inputs");
    } finally {
      setSaving(false);
    }
  }, [id, candidateId, inputValues, fetchCandidate]);

  const stepDescriptions = useMemo(
    () => buildStepDescriptionMap(getApplicableSteps(candidate, migration)),
    [candidate, migration],
  );

  const progress = useMemo(
    () =>
      stepsData && migration
        ? calculateStepProgress(stepsData, getApplicableSteps(candidate, migration).length)
        : null,
    [stepsData, migration, candidate],
  );

  // Temporal workflow ID used for event callbacks — derived from migration + candidate IDs
  const runId = `${id}__${candidateId}`;

  return (
    <div className="space-y-8 animate-fade-in-up">
      {notFound ? (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <div className="w-14 h-14 rounded-xl bg-muted/50 border border-border-hover/50 flex items-center justify-center mb-5">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" className="text-muted-foreground">
              <path
                d="M12 8v4m0 4h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </div>
          <h2 className="text-lg font-semibold text-foreground mb-1">Workflow Not Found</h2>
          <p className="text-sm text-muted-foreground mb-2 max-w-md">
            No active or completed workflow found for this candidate. This typically happens when
            the orchestration engine is restarted and ephemeral state is lost.
          </p>
          <p className="text-xs font-mono text-muted-foreground/70 mb-6 break-all max-w-md">{candidateId}</p>
          <Link href={ROUTES.migrationDetail(id)} className={buttonVariants({ variant: "outline" })}>
            Back to Migration
          </Link>
        </div>
      ) : (
        <>
          {/* Sticky header */}
          <div className="sticky top-6 z-10 bg-background pb-6 space-y-5">
            {/* Breadcrumb + actions */}
            <div className="flex items-center gap-2 flex-wrap">
              <Link
                href={ROUTES.migrationDetail(id)}
                className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground/80 transition-colors shrink-0"
              >
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                  <path d="M7 3L4 6l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                {migration?.name ?? id}
              </Link>
              <span className="text-muted-foreground/50 select-none">·</span>
              <span className="inline-flex items-center gap-1.5 text-xs font-mono bg-muted border border-border-hover/50 text-foreground/80 px-2 py-0.5 rounded-md">
                {candidateId}
              </span>
              {candidate?.metadata ? Object.entries(candidate.metadata).map(([k, v]) => (
                <span
                  key={k}
                  className="inline-flex items-center gap-1 text-xs font-mono text-muted-foreground bg-muted/50 border border-border-hover/30 px-2 py-0.5 rounded-md"
                >
                  <span className="text-muted-foreground/60">{k}</span>
                  <span>{v}</span>
                </span>
              )) : null}
              {stepsData ? (
                <>
                  <span className="flex-1" />
                  {hasRequiredInputs ? (
                    <button
                      type="button"
                      onClick={openDialog}
                      className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground/80 bg-muted hover:bg-muted border border-border-hover/50 px-2.5 py-1 rounded-md transition-colors shrink-0"
                    >
                      <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                        <path d="M8.5 1.5l2 2M1.5 8.5l6-6 2 2-6 6H1.5v-2Z" stroke="currentColor" strokeWidth="1" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                      Edit inputs
                    </button>
                  ) : null}
                  <span className={`text-xs font-medium px-2.5 py-1 rounded-md border shrink-0 ${
                    stepsData.status === "completed"
                      ? "text-completed bg-completed/10 border-completed/20"
                      : "text-running bg-running/10 border-running/20"
                  }`}>
                    {stepsData.status}
                  </span>
                </>
              ) : null}
            </div>

            {error ? (
              <div className="bg-destructive/8 border border-destructive/20 rounded-lg px-4 py-3 text-sm text-destructive">
                {error}
              </div>
            ) : null}

            {/* Loading skeleton */}
            {!stepsData && !error ? (
              <div className="space-y-4">
                <Skeleton className="h-1 w-full rounded-full" />
                <Skeleton className="h-32 w-full" />
              </div>
            ) : null}

            {/* Progress */}
            {progress ? (
              <div className="flex items-center gap-3">
                <div className="flex-1 h-1 bg-muted rounded-full overflow-hidden">
                  <div
                    className="h-full bg-primary/50 rounded-full transition-all duration-500"
                    style={{ width: `${(progress.done / progress.total) * 100}%` }}
                  />
                </div>
                <span className="text-xs text-muted-foreground font-mono tabular-nums shrink-0">
                  {progress.done} / {progress.total}
                </span>
                {progress.activeStepName ? (
                  <>
                    <span className="text-muted-foreground/50 select-none">·</span>
                    <span className="text-xs text-muted-foreground font-mono truncate max-w-[200px]">
                      {progress.activeStepName}
                    </span>
                  </>
                ) : null}
              </div>
            ) : null}
          </div>

          {/* Step timeline */}
          {stepsData ? (
            <div className="border border-border rounded-lg p-5 max-w-2xl mx-auto">
              <StepTimeline
                results={stepsData.steps}
                stepDescriptions={stepDescriptions}
                onComplete={(stepName, candidateId, status) => {
                  void (async () => {
                    await completeStep(runId, stepName, candidateId, status);
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
          ) : null}
        </>
      )}

      {/* Edit inputs dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <div className="flex items-center gap-2">
              <DialogTitle>Edit inputs</DialogTitle>
              <Tooltip content="These values are passed to each step at runtime. Changes take effect on the next dispatch.">
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" className="text-muted-foreground/70 hover:text-muted-foreground cursor-default transition-colors">
                  <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.5" />
                  <path d="M8 7v5M8 5h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                </svg>
              </Tooltip>
            </div>
          </DialogHeader>
          <div className="px-5 py-5 space-y-4">
            {requiredInputs?.map((inp, i) => (
              <div key={inp.name} className="space-y-1.5">
                <label
                  htmlFor={`input-${inp.name}`}
                  className="text-xs font-medium text-muted-foreground"
                >
                  {inp.label}
                </label>
                <Input
                  id={`input-${inp.name}`}
                  type="text"
                  value={inputValues[inp.name] ?? ""}
                  onChange={(e) =>
                    setInputValues((prev) => ({ ...prev, [inp.name]: e.target.value }))
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
            <Button variant="outline" size="sm" onClick={() => setDialogOpen(false)} disabled={saving}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => void handleSave()} disabled={saving}>
              {saving ? "Saving…" : "Save inputs"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
