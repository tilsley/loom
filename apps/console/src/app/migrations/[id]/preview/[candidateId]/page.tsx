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
  type FileDiff,
} from "@/lib/api";
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
        const found = candidates.find((c) => c.id === candidateId);
        if (!found) {
          setLoadError(`Candidate "${candidateId}" not found`);
          return;
        }
        const c: Candidate = {
          id: found.id,
          kind: found.kind,
          metadata: found.metadata,
          files: found.files,
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

  const steps = migration?.steps ?? [];

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
            {requiredInputs.map((inp) => (
              <EditableLabel
                key={inp.name}
                label={inp.label}
                value={inputs[inp.name] ?? ""}
                onCommit={(val) => setInputs((v) => ({ ...v, [inp.name]: val }))}
              />
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

// --- Editable label ---

function EditableLabel({
  label,
  value,
  onCommit,
}: {
  label: string;
  value: string;
  onCommit: (newValue: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);

  function commit() {
    onCommit(draft.trim());
    setEditing(false);
  }

  function cancel() {
    setDraft(value);
    setEditing(false);
  }

  if (editing) {
    return (
      <div className="flex items-center gap-2">
        <span className="text-xs text-zinc-500 shrink-0">{label}:</span>
        <input
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") commit();
            if (e.key === "Escape") cancel();
          }}
          className="font-mono text-sm text-zinc-200 bg-zinc-800/60 border border-zinc-700 rounded px-2 py-0.5 focus:outline-none focus:border-zinc-500 min-w-0 flex-1 max-w-xs"
        />
        <button
          onClick={commit}
          title="Confirm (Enter)"
          className="text-emerald-400 hover:text-emerald-300 transition-colors shrink-0"
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
            <path d="M2 8l5 5 7-7" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
        <button
          onClick={cancel}
          title="Cancel (Escape)"
          className="text-zinc-600 hover:text-zinc-400 transition-colors shrink-0"
        >
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none">
            <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
          </svg>
        </button>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2 group">
      <span className="text-xs text-zinc-500 shrink-0">{label}:</span>
      <span className="font-mono text-sm text-zinc-200">
        {value || <span className="text-zinc-600 italic">not set</span>}
      </span>
      <button
        onClick={() => { setDraft(value); setEditing(true); }}
        title="Edit"
        className="text-zinc-700 hover:text-zinc-400 transition-colors opacity-0 group-hover:opacity-100"
      >
        <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
          <path d="M11.013 1.427a1.75 1.75 0 0 1 2.474 0l1.086 1.086a1.75 1.75 0 0 1 0 2.474l-8.61 8.61c-.21.21-.47.364-.756.445l-3.251.93a.75.75 0 0 1-.927-.928l.929-3.25c.081-.286.235-.547.445-.758l8.61-8.61Zm.176 4.823L9.75 4.81l-6.286 6.287.955.955 6.287-6.288Zm1.238-3.763a.25.25 0 0 0-.354 0L10.811 3.75l1.439 1.44 1.263-1.263a.25.25 0 0 0 0-.354Z" />
        </svg>
      </button>
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
  let i = m,
    j = n;
  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && a[i - 1] === b[j - 1]) {
      result.push({ type: "context", text: a[i - 1] });
      i--;
      j--;
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      result.push({ type: "add", text: b[j - 1] });
      j--;
    } else {
      result.push({ type: "remove", text: a[i - 1] });
      i--;
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
      if (skip > 0) {
        result.push({ type: "ellipsis", count: skip });
        skip = 0;
      }
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
      <div className="flex items-center gap-2 px-3 py-1.5 bg-zinc-900/80 border-b border-zinc-800/60">
        <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor" className="text-zinc-500 shrink-0">
          <path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z" />
        </svg>
        <span className="text-zinc-300 flex-1 truncate">{diff.path}</span>
        <span className="text-zinc-600 shrink-0">{diff.repo}</span>
        {diff.status === "new" ? (
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            new
          </span>
        ) : diff.status === "deleted" ? (
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-red-500/10 text-red-400 border border-red-500/20">
            deleted
          </span>
        ) : null}
      </div>
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

function DryRunStepResult({
  result,
}: {
  result: { stepName: string; skipped: boolean; error?: string; files?: FileDiff[] };
}) {
  if (result.skipped) {
    return <div className="ml-7 text-xs text-zinc-600 italic">Skipped — handled by another worker</div>;
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
