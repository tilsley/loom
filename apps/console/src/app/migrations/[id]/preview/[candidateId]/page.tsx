"use client";

import { useEffect, useState, useMemo, useRef, useCallback } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import {
  getMigration,
  getCandidates,
  dryRun,
  startRun,
  ConflictError,
  type Migration,
  type Candidate,
  type DryRunResult,
} from "@/lib/api";
import { Input } from "@/components/ui";
import { DryRunStepResult } from "@/components/file-diff-view";
import { ROUTES } from "@/lib/routes";

export default function PreviewPage() {
  const { id, candidateId } = useParams<{ id: string; candidateId: string }>();
  const router = useRouter();
  const searchParams = useSearchParams();

  const [migration, setMigration] = useState<Migration | null>(null);
  const [candidate, setCandidate] = useState<Candidate | null>(null);
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [dryRunResult, setDryRunResult] = useState<DryRunResult | null>(null);
  const [dryRunLoading, setDryRunLoading] = useState(false);
  const [dryRunError, setDryRunError] = useState<string | null>(null);
  const [executing, setExecuting] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  // Track last inputs used for dry run to avoid redundant re-runs
  const lastDryRunInputs = useRef<string>("");

  // Load migration + candidate on mount
  useEffect(() => {
    Promise.all([getMigration(id), getCandidates(id)])
      .then(([mig, candidates]) => {
        setMigration(mig);
        const found = (candidates ?? []).find((c) => c.id === candidateId);
        if (!found) {
          setLoadError(`Candidate "${candidateId}" not found`);
          return;
        }
        const c: Candidate = {
          id: found.id,
          kind: found.kind,
          metadata: found.metadata,
          files: found.files,
          steps: found.steps,
        };
        setCandidate(c);

        // Pre-fill inputs: URL params (from preview modal) take precedence over candidate metadata
        const required = mig.requiredInputs ?? [];
        if (required.length > 0) {
          const prefilled: Record<string, string> = {};
          for (const inp of required) {
            const urlVal = searchParams.get(inp.name);
            prefilled[inp.name] = urlVal !== null ? urlVal : (found.metadata?.[inp.name] ?? "");
          }
          setInputs(prefilled);
        }
      })
      .catch((e) => setLoadError(e instanceof Error ? e.message : "Failed to load"));
  }, [id, candidateId, searchParams]);

  const requiredInputs = useMemo(() => migration?.requiredInputs ?? [], [migration]);
  const allInputsFilled = requiredInputs.every((inp) => inputs[inp.name]?.trim());

  // Build the candidate with inputs merged into metadata (for dry-run and execute)
  const candidateWithInputs = useMemo<Candidate | null>(() => {
    if (!candidate) return null;
    if (requiredInputs.length === 0) return candidate;
    const merged = { ...(candidate.metadata ?? {}), ...inputs };
    return { ...candidate, metadata: merged };
  }, [candidate, inputs, requiredInputs]);

  // Auto-trigger dry run once migration + candidate are loaded and all inputs are filled
  const triggerDryRun = useCallback(
    (c: Candidate) => {
      const key = JSON.stringify(inputs);
      if (key === lastDryRunInputs.current) return;
      lastDryRunInputs.current = key;

      setDryRunLoading(true);
      setDryRunError(null);
      dryRun(id, c)
        .then(setDryRunResult)
        .catch((e) => setDryRunError(e instanceof Error ? e.message : "Dry run failed"))
        .finally(() => setDryRunLoading(false));
    },
    [id, inputs],
  );

  useEffect(() => {
    if (!candidateWithInputs || !migration) return;
    if (requiredInputs.length > 0 && !allInputsFilled) return;
    triggerDryRun(candidateWithInputs);
  }, [candidateWithInputs, migration, requiredInputs.length, allInputsFilled, triggerDryRun]);

  function handleRetry() {
    if (!candidateWithInputs) return;
    lastDryRunInputs.current = "";
    triggerDryRun(candidateWithInputs);
  }

  async function handleStart() {
    if (!candidate || !migration) return;
    setExecuting(true);
    try {
      const inputsToSend = Object.keys(inputs).length > 0 ? inputs : undefined;
      await startRun(id, candidateId, inputsToSend);
      router.push(ROUTES.candidateSteps(id, candidateId));
    } catch (e) {
      if (e instanceof ConflictError) {
        toast.error("Candidate is already running or completed");
      } else {
        toast.error(e instanceof Error ? e.message : "Failed to execute");
      }
      setExecuting(false);
    }
  }

  // Use the candidate's own step list if present — it was computed at discovery
  // time and contains only the steps applicable to this specific candidate.
  // Fall back to the migration-level template for display if not set.
  const steps = (candidate?.steps?.length ? candidate.steps : migration?.steps) ?? [];

  const dryRunByStep = useMemo(() => {
    if (!dryRunResult) return new Map<string, DryRunResult["steps"][number]>();
    return new Map(dryRunResult.steps.map((s) => [s.stepName, s]));
  }, [dryRunResult]);

  if (loadError) {
    return (
      <div className="space-y-6 animate-fade-in-up">
        <Link
          href={ROUTES.migrationDetail(id)}
          className="inline-flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M7 3L4 6l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          Back
        </Link>
        <div className="bg-red-500/8 border border-red-500/20 rounded-lg px-4 py-3 text-sm text-red-400">
          {loadError}
        </div>
      </div>
    );
  }

  if (!migration || !candidate) {
    return (
      <div className="space-y-6 animate-fade-in-up text-sm text-zinc-600">
        Loading…
      </div>
    );
  }

  const team = candidate.metadata?.team;

  return (
    <div className="space-y-6 animate-fade-in-up">
      {/* Header */}
      <div className="flex items-center gap-2 flex-wrap">
        <Link
          href={ROUTES.migrationDetail(id)}
          className="inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-300 transition-colors shrink-0"
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M7 3L4 6l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          {migration.name}
        </Link>
        <span className="text-zinc-700 select-none">·</span>
        <span className="inline-flex items-center gap-1.5 text-xs font-mono bg-zinc-800/60 border border-zinc-700/50 text-zinc-300 px-2 py-0.5 rounded-md">
          {team ? <span className="text-zinc-500">{team} /</span> : null}
          {candidate.id}
        </span>
        <span className="flex-1" />
        <span className="inline-flex items-center gap-1.5 text-xs font-medium px-2 py-0.5 rounded border bg-amber-500/10 text-amber-400 border-amber-500/20">
          preview
        </span>
      </div>

      {/* Inputs */}
      {requiredInputs.length > 0 ? (
        <section className="w-fit min-w-[700px] mx-auto">
          <div className="border border-zinc-800/80 rounded-lg px-5 py-4 space-y-3">
            <div className="text-xs font-medium text-zinc-500 uppercase tracking-widest">
              Required inputs
            </div>
            {requiredInputs.map((inp, i) => (
              <div key={inp.name} className="flex items-center gap-3">
                <label
                  htmlFor={`preview-input-${inp.name}`}
                  className="text-xs text-zinc-500 shrink-0 w-28 text-right"
                >
                  {inp.label}
                </label>
                <Input
                  id={`preview-input-${inp.name}`}
                  type="text"
                  value={inputs[inp.name] ?? ""}
                  onChange={(e) => setInputs((v) => ({ ...v, [inp.name]: e.target.value }))}
                  placeholder={inp.label}
                  className="font-mono flex-1 max-w-xs"
                  autoFocus={i === 0}
                />
              </div>
            ))}
          </div>
        </section>
      ) : null}

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
                <path d="M8 1v7l4 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.5" />
              </svg>
              Dry run
            </span>
            {dryRunLoading ? (
              <span className="inline-flex items-center gap-1.5 text-xs text-zinc-500">
                <svg className="animate-spin w-3 h-3" viewBox="0 0 16 16" fill="none">
                  <circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="2" strokeDasharray="28" strokeDashoffset="10" strokeLinecap="round" />
                </svg>
                Simulating…
              </span>
            ) : null}
            {requiredInputs.length > 0 && !allInputsFilled ? (
              <span className="text-xs text-zinc-600 italic">Fill inputs to preview</span>
            ) : null}
          </div>
          {dryRunError ? (
            <div className="border border-red-500/20 bg-red-500/5 rounded-lg px-5 py-5 flex items-start justify-between gap-4">
              <div className="space-y-1 min-w-0">
                <p className="text-sm font-medium text-red-400">Dry run failed</p>
                <p className="text-xs font-mono text-red-400/60 break-all">{dryRunError}</p>
              </div>
              <button
                onClick={handleRetry}
                className="shrink-0 text-xs text-zinc-400 hover:text-zinc-200 border border-zinc-700 hover:border-zinc-500 rounded px-3 py-1.5 transition-colors"
              >
                Retry
              </button>
            </div>
          ) : (
          <div className="border border-zinc-800/80 rounded-lg divide-y divide-zinc-800/60">
            {steps.map((step, i) => {
              const config = step.config ? Object.entries(step.config) : [];
              const instructions = step.config?.instructions;
              const otherConfig = config.filter(([k]) => k !== "instructions");
              const stepDryRun = dryRunByStep.get(step.name);

              return (
                <div key={step.name} className="px-5 py-4 space-y-2.5">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2.5">
                      <span className="text-xs font-mono text-zinc-600 tabular-nums w-5 shrink-0 select-none">
                        {String(i + 1).padStart(2, "0")}.
                      </span>
                      <span className="text-base font-medium font-mono text-zinc-100">{step.name}</span>
                    </div>
                    <span className="text-xs font-mono text-zinc-500 bg-zinc-800/60 px-2 py-0.5 rounded shrink-0">
                      {step.migratorApp}
                    </span>
                  </div>

                  {Boolean(step.description) && (
                    <p className="text-sm text-zinc-400 ml-7">{step.description}</p>
                  )}

                  {instructions ? (
                    <div className="ml-7 bg-blue-500/5 border border-blue-500/15 rounded-md px-3 py-2.5">
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
                    </div>
                  ) : null}

                  {otherConfig.length > 0 && (
                    <div className="ml-7 flex flex-wrap gap-1.5">
                      {otherConfig.map(([k, v]) => (
                        <span key={k} className="text-xs font-mono text-zinc-500 bg-zinc-800/40 px-1.5 py-0.5 rounded">
                          {k}=<span className="text-zinc-400">{v}</span>
                        </span>
                      ))}
                    </div>
                  )}

                  {(dryRunLoading && !stepDryRun) || (stepDryRun && !stepDryRun.skipped) ? (
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
          )}
        </section>
      ) : (
        <div className="text-sm text-zinc-600 italic">Loading step definitions…</div>
      )}

      {/* Start / Back actions */}
      <div className="w-fit min-w-[700px] mx-auto flex items-center justify-between pt-2 pb-6">
        <Link
          href={ROUTES.migrationDetail(id)}
          className="text-sm text-zinc-500 hover:text-zinc-300 transition-colors"
        >
          Back
        </Link>
        <button
          onClick={() => void handleStart()}
          disabled={executing || (requiredInputs.length > 0 && !allInputsFilled)}
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
              Start
            </>
          )}
        </button>
      </div>
    </div>
  );
}

