"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import {
  dryRun,
  startRun,
  ConflictError,
  type Migration,
  type Candidate,
  type DryRunResult,
} from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { getApplicableSteps } from "@/lib/steps";
import { prefillInputs, mergeInputsIntoCandidate } from "@/lib/inputs";
import { Button, Input, Sheet, SheetContent, SheetHeader, SheetFooter, SheetTitle } from "@/components/ui";
import { DryRunStepResult } from "@/components/file-diff-view";

interface PreviewPanelProps {
  open: boolean;
  migrationId: string;
  migration: Migration;
  candidate: Candidate;
  onClose: () => void;
}

export function PreviewPanel({ open, migrationId, migration, candidate, onClose }: PreviewPanelProps) {
  const router = useRouter();
  const requiredInputs = useMemo(
    () => migration.requiredInputs ?? [],
    [migration.requiredInputs],
  );

  const [inputs, setInputs] = useState<Record<string, string>>(() =>
    prefillInputs(requiredInputs, candidate),
  );

  const [dryRunResult, setDryRunResult] = useState<DryRunResult | null>(null);
  const [dryRunLoading, setDryRunLoading] = useState(false);
  const [dryRunError, setDryRunError] = useState<string | null>(null);
  const [executing, setExecuting] = useState(false);
  // When required inputs exist, dry run must be explicitly triggered by the user
  const [dryRunEnabled, setDryRunEnabled] = useState(requiredInputs.length === 0);

  const lastDryRunInputs = useRef<string>("");

  const allInputsFilled = requiredInputs.every((inp) => inputs[inp.name]?.trim());

  const candidateWithInputs = useMemo<Candidate>(
    () => mergeInputsIntoCandidate(candidate, requiredInputs, inputs),
    [candidate, inputs, requiredInputs],
  );

  const triggerDryRun = useCallback(
    (c: Candidate) => {
      const key = JSON.stringify(inputs);
      if (key === lastDryRunInputs.current) return;
      lastDryRunInputs.current = key;

      setDryRunLoading(true);
      setDryRunError(null);
      setDryRunResult(null);
      dryRun(migrationId, c)
        .then(setDryRunResult)
        .catch((e) => setDryRunError(e instanceof Error ? e.message : "Dry run failed"))
        .finally(() => setDryRunLoading(false));
    },
    [migrationId, inputs],
  );

  useEffect(() => {
    if (!dryRunEnabled) return;
    if (requiredInputs.length > 0 && !allInputsFilled) return;
    triggerDryRun(candidateWithInputs);
  }, [dryRunEnabled, candidateWithInputs, requiredInputs.length, allInputsFilled, triggerDryRun]);

  async function handleStart() {
    setExecuting(true);
    try {
      const inputsToSend = Object.keys(inputs).length > 0 ? inputs : undefined;
      await startRun(migrationId, candidate.id, inputsToSend);
      router.push(ROUTES.candidateSteps(migrationId, candidate.id));
      onClose();
    } catch (e) {
      if (e instanceof ConflictError) {
        toast.error("Candidate is already running or completed");
      } else {
        toast.error(e instanceof Error ? e.message : "Failed to execute");
      }
      setExecuting(false);
    }
  }

  const steps = getApplicableSteps(candidate, migration);

  const dryRunByStep = useMemo(() => {
    if (!dryRunResult) return new Map<string, DryRunResult["steps"][number]>();
    return new Map(dryRunResult.steps.map((s) => [s.stepName, s]));
  }, [dryRunResult]);

  const team = candidate.metadata?.team;

  return (
    <Sheet open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <SheetContent side="right" className="flex flex-col p-0">
        {/* Header */}
        <SheetHeader>
          <div className="min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <SheetTitle>{migration.name}</SheetTitle>
              <span className="text-muted-foreground/50 select-none">·</span>
              <span className="inline-flex items-center gap-1.5 text-xs font-mono bg-muted border border-border-hover/50 text-foreground/80 px-2 py-0.5 rounded-md">
                {team ? <span className="text-muted-foreground">{team} /</span> : null}
                {candidate.id}
              </span>
            </div>
            <div className="flex items-center gap-2 mt-1.5">
              <span className="inline-flex items-center gap-1.5 text-xs font-medium px-2 py-0.5 rounded border bg-running/10 text-running border-running/20">
                preview
              </span>
              {dryRunLoading ? (
                <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                  <svg className="animate-spin w-3 h-3" viewBox="0 0 16 16" fill="none">
                    <circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="2" strokeDasharray="28" strokeDashoffset="10" strokeLinecap="round" />
                  </svg>
                  Simulating…
                </span>
              ) : null}
              {requiredInputs.length > 0 && !allInputsFilled ? (
                <span className="text-xs text-muted-foreground/70 italic">Fill in all inputs to continue</span>
              ) : null}
            </div>
          </div>
        </SheetHeader>

        {/* Scrollable body */}
        <div className="flex-1 overflow-y-auto px-5 py-5 space-y-6">
          {/* Required inputs — always-visible form fields */}
          {requiredInputs.length > 0 ? (
            <section>
              <div className="text-xs font-medium text-muted-foreground uppercase tracking-widest mb-3">
                Required inputs
              </div>
              <div className="border border-border rounded-lg px-4 py-4 space-y-3">
                {requiredInputs.map((inp, i) => (
                  <div key={inp.name} className="flex items-start gap-3">
                    <label
                      htmlFor={`panel-input-${inp.name}`}
                      className="text-xs text-muted-foreground shrink-0 w-28 text-right pt-2"
                    >
                      {inp.label}
                    </label>
                    <div className="flex-1 space-y-1">
                      <Input
                        id={`panel-input-${inp.name}`}
                        type="text"
                        value={inputs[inp.name] ?? ""}
                        onChange={(e) => setInputs((v) => ({ ...v, [inp.name]: e.target.value }))}
                        placeholder={inp.label}
                        className="font-mono w-full"
                        autoFocus={i === 0}
                      />
                      {inp.description ? (
                        <p className="text-xs text-muted-foreground/70 italic">{inp.description}</p>
                      ) : null}
                    </div>
                  </div>
                ))}
              </div>
            </section>
          ) : null}

          {/* Dry run error */}
          {dryRunEnabled && dryRunError ? (
            <div className="border border-destructive/20 bg-destructive/5 rounded-lg px-4 py-4 flex items-start justify-between gap-4">
              <div className="space-y-1 min-w-0">
                <p className="text-sm font-medium text-destructive">Dry run failed</p>
                <p className="text-xs font-mono text-destructive/60 break-all">{dryRunError}</p>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  lastDryRunInputs.current = "";
                  triggerDryRun(candidateWithInputs);
                }}
                className="shrink-0"
              >
                Retry
              </Button>
            </div>
          ) : null}

          {/* Steps preview — only shown after inputs are confirmed */}
          {dryRunEnabled && steps.length > 0 ? (
            <section>
              <div className="flex items-center gap-2 mb-3">
                <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-widest">Steps</h2>
                <span className="text-xs font-mono text-muted-foreground/70 bg-muted px-1.5 py-0.5 rounded">
                  {steps.length}
                </span>
              </div>
              <div className="border border-border rounded-lg divide-y divide-border/60">
                {steps.map((step, i) => {
                  const config = step.config ? Object.entries(step.config) : [];
                  const instructions = step.config?.instructions;
                  const otherConfig = config.filter(([k]) => k !== "instructions");
                  const stepDryRun = dryRunByStep.get(step.name);

                  return (
                    <div key={step.name} className="px-4 py-4 space-y-2.5">
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex items-center gap-2.5">
                          <span className="text-xs font-mono text-muted-foreground/70 tabular-nums w-5 shrink-0 select-none">
                            {String(i + 1).padStart(2, "0")}.
                          </span>
                          <span className="text-sm font-medium font-mono text-foreground">{step.name}</span>
                        </div>
                        <span className="text-xs font-mono text-muted-foreground bg-muted px-2 py-0.5 rounded shrink-0">
                          {step.migratorApp}
                        </span>
                      </div>

                      {Boolean(step.description) && (
                        <p className="text-sm text-muted-foreground ml-7">{step.description}</p>
                      )}

                      {instructions ? (
                        <div className="ml-7 bg-pending/5 border border-pending/15 rounded-md px-3 py-2.5">
                          <div className="text-xs font-medium text-pending/70 uppercase tracking-widest mb-2">
                            Instructions
                          </div>
                          <ul className="space-y-1">
                            {instructions.split("\n").map((line, j) => (
                              <li key={j} className="text-sm text-foreground/80 font-mono">
                                {line}
                              </li>
                            ))}
                          </ul>
                        </div>
                      ) : null}

                      {otherConfig.length > 0 && (
                        <div className="ml-7 flex flex-wrap gap-1.5">
                          {otherConfig.map(([k, v]) => (
                            <span key={k} className="text-xs font-mono text-muted-foreground bg-muted/40 px-1.5 py-0.5 rounded">
                              {k}=<span className="text-muted-foreground">{v}</span>
                            </span>
                          ))}
                        </div>
                      )}

                      {(dryRunLoading && !stepDryRun) || stepDryRun ? (
                        <div className="ml-7 space-y-2">
                          <div className="flex items-center gap-2">
                            <span className="text-xs font-medium text-muted-foreground/70 uppercase tracking-widest">
                              Expected changes
                            </span>
                            <div className="flex-1 h-px bg-border" />
                          </div>
                          {dryRunLoading && !stepDryRun ? (
                            <div className="h-5 rounded bg-muted/40 animate-pulse w-48" />
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
          ) : null}
        </div>

        {/* Footer */}
        <SheetFooter>
          <Button variant="outline" size="sm" onClick={onClose}>
            Cancel
          </Button>
          {!dryRunEnabled ? (
            <Button
              onClick={() => {
                lastDryRunInputs.current = "";
                setDryRunEnabled(true);
              }}
              disabled={!allInputsFilled}
            >
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none">
                <path d="M8 1v7l4 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.5" />
              </svg>
              Run preview
            </Button>
          ) : (
            <Button
              onClick={() => void handleStart()}
              disabled={executing}
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
                  <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                    <path d="M3 2l7 4-7 4V2z" fill="currentColor" />
                  </svg>
                  Start
                </>
              )}
            </Button>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
