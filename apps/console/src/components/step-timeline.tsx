"use client";

import { useState, useEffect, useRef } from "react";
import type { StepResult } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button, buttonVariants } from "@/components/ui";

type Phase = "in_progress" | "open" | "merged" | "failed" | "completed" | "awaiting_review";

function getPhase(r: StepResult): Phase {
  return r.status as Phase;
}

export function StepTimeline({
  results,
  stepDescriptions,
  stepInstructions,
  onComplete,
  onRetry,
}: {
  results: StepResult[];
  stepDescriptions?: Map<string, string>;
  stepInstructions?: Map<string, string>;
  onComplete?: (stepName: string, candidateId: string, success: boolean) => void;
  onRetry?: (stepName: string, candidateId: string) => void;
}) {
  // Hooks must be declared before any early return
  const lastActiveIndex = results.reduce((acc, r, idx) => {
    const p = getPhase(r);
    return p === "open" || p === "in_progress" || p === "awaiting_review" ? idx : acc;
  }, -1);

  const activeRef = useRef<HTMLDivElement>(null);
  const hasScrolled = useRef(false);

  useEffect(() => {
    if (!hasScrolled.current && activeRef.current) {
      hasScrolled.current = true;
      activeRef.current.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
  }, [results]);

  if (results.length === 0) {
    return (
      <div className="border border-dashed border-border rounded-lg py-12 text-center">
        <div className="w-10 h-10 rounded-lg bg-card flex items-center justify-center mx-auto mb-3">
          <svg
            className="w-5 h-5 text-muted-foreground animate-pulse"
            viewBox="0 0 16 16"
            fill="none"
          >
            <circle
              cx="8"
              cy="8"
              r="6"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeDasharray="28"
              strokeDashoffset="8"
              strokeLinecap="round"
            />
          </svg>
        </div>
        <p className="text-sm text-muted-foreground">Waiting for worker callbacks...</p>
        <p className="text-xs text-zinc-600 mt-1">
          Steps will appear here as workers report progress
        </p>
      </div>
    );
  }

  return (
    <div>
      {results.map((r, i) => {
        const phase = getPhase(r);
        const description = stepDescriptions?.get(r.stepName);
        const instructions = stepInstructions?.get(r.stepName);
        const isLast = i === results.length - 1;
        const isActive = phase === "open" || phase === "in_progress" || phase === "awaiting_review";

        const lineColor =
          phase === "completed" || phase === "merged"
            ? "bg-teal-500/25"
            : phase === "open"
              ? "bg-emerald-500/20"
              : phase === "failed"
                ? "bg-red-500/20"
                : "bg-zinc-800";

        return (
          <div
            key={i}
            ref={i === lastActiveIndex ? activeRef : undefined}
            className={cn(
              "relative flex gap-5",
              // Non-active: pb-6 is the inter-step gap (line runs through it)
              !isActive && !isLast && "pb-6",
              // Active: box gets its own padding; mb-6 creates the gap outside the border
              isActive && !isLast && "mb-6",
              isActive && "px-3 py-3 rounded-lg",
              phase === "awaiting_review" && "bg-blue-500/10 border border-blue-500/25",
              phase === "open" && "bg-emerald-500/[0.07] border border-emerald-500/20",
              phase === "in_progress" && "bg-amber-500/[0.07] border border-amber-500/20",
            )}
          >
            {/* Timeline column: dot + connecting line */}
            <div className="flex flex-col items-center flex-shrink-0 w-5">
              <TimelineDot phase={phase} />
              {!isLast && <div className={cn("w-px flex-1 mt-1", lineColor)} />}
            </div>

            {/* Content */}
            <div className="flex-1 min-w-0 pt-px">
              {/* Top row: name/description left, badges right */}
              <div className="flex items-start gap-4">
                {/* Left: name + description */}
                <div className="flex-1 min-w-0">
                  <span className="text-base font-medium font-mono text-foreground block">
                    <span className="text-xs text-zinc-700 mr-1.5 tabular-nums select-none">
                      {String(i + 1).padStart(2, "0")}.
                    </span>
                    {r.stepName}
                  </span>
                  {Boolean(description) && (
                    <span className="text-sm text-muted-foreground mt-0.5 block">{description}</span>
                  )}
                </div>

                {/* Right: View PR (top, beside name) then phase label (below, beside description) */}
                <div className="flex flex-col items-end gap-1.5 shrink-0">
                  {r.metadata?.prUrl ? (
                    <a
                      href={r.metadata.prUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={cn(buttonVariants({ variant: "default", size: "sm" }))}
                    >
                      View PR
                      <svg width="10" height="10" viewBox="0 0 12 12" fill="none" className="opacity-70">
                        <path d="M3.5 1.5h7v7M10 2L2 10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </a>
                  ) : null}
                  <PhaseLabel phase={phase} />
                </div>
              </div>

              {/* Open PR: manual merge action for local dev / no-webhook environments */}
              {phase === "open" && onComplete ? (
                <div className="mt-3">
                  <MergeAction onMerge={() => onComplete(r.stepName, r.candidate.id, true)} />
                </div>
              ) : null}

              {/* Failed step: retry action */}
              {phase === "failed" && onRetry ? (
                <div className="mt-3">
                  <RetryAction onRetry={() => onRetry(r.stepName, r.candidate.id)} />
                </div>
              ) : null}

              {/* Awaiting review: instructions + actions — full width below header row */}
              {phase === "awaiting_review" ? (
                <div className="mt-3 space-y-3">
                  {instructions ? (
                    <div className="bg-blue-500/5 border border-blue-500/15 rounded-md px-3 py-2.5">
                      <div className="text-xs font-medium text-blue-400/70 uppercase tracking-widest mb-1.5">
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
                  {onComplete ? (
                    <ReviewActions
                      onComplete={(success) => onComplete(r.stepName, r.candidate.id, success)}
                    />
                  ) : null}
                </div>
              ) : null}

              {/* Extra metadata tags — full width below header row */}
              {r.metadata
                ? (() => {
                    const extra = Object.entries(r.metadata).filter(
                      ([k]) =>
                        k !== "phase" && k !== "prUrl" && k !== "instructions" && k !== "commitSha",
                    );
                    if (extra.length === 0) return null;
                    return (
                      <div className="flex flex-wrap gap-1.5 mt-2">
                        {extra.map(([k, v]) => (
                          <span
                            key={k}
                            className="inline-flex items-center gap-1.5 text-xs font-mono text-zinc-500 bg-zinc-800/50 px-2 py-0.5 rounded"
                          >
                            <span className="text-zinc-600">{k}</span>
                            <span className="text-zinc-400">{v}</span>
                          </span>
                        ))}
                      </div>
                    );
                  })()
                : null}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function TimelineDot({ phase }: { phase: Phase }) {
  const cls = "w-4 h-4 flex-shrink-0 mt-0.5";

  switch (phase) {
    case "completed":
      return (
        <svg className={cn(cls, "text-teal-500")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="6.5" />
          <path d="M5 8l2 2 4-4" />
        </svg>
      );
    case "merged":
      return (
        <svg className={cn(cls, "text-purple-400")} viewBox="0 0 16 16" fill="currentColor">
          <path d="M5 3.25a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0zm0 2.122a2.25 2.25 0 1 0-1.5 0v.878A2.25 2.25 0 0 0 5.75 8.5h1.5v2.128a2.251 2.251 0 1 0 1.5 0V8.5h1.5a2.25 2.25 0 0 0 2.25-2.25v-.878a2.25 2.25 0 1 0-1.5 0v.878a.75.75 0 0 1-.75.75h-4.5A.75.75 0 0 1 5 6.25v-.878zm3.75 7.378a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0zm3-8.75a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0z" />
        </svg>
      );
    case "in_progress":
      return (
        <svg className={cn(cls, "text-amber-400 animate-spin")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <circle cx="8" cy="8" r="6.5" strokeDasharray="30" strokeDashoffset="10" />
        </svg>
      );
    case "awaiting_review":
      return (
        <svg className={cn(cls, "text-blue-400")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M1 8s3-5.5 7-5.5S15 8 15 8s-3 5.5-7 5.5S1 8 1 8z" />
          <circle cx="8" cy="8" r="2" fill="currentColor" stroke="none" />
        </svg>
      );
    case "open":
      return (
        <svg className={cn(cls, "text-emerald-400")} viewBox="0 0 16 16" fill="currentColor">
          <path d="M1.5 3.25a2.25 2.25 0 1 1 3 2.122v5.256a2.251 2.251 0 1 1-1.5 0V5.372A2.25 2.25 0 0 1 1.5 3.25Zm5.677-.177L9.573.677A.25.25 0 0 1 10 .854V2.5h1A2.5 2.5 0 0 1 13.5 5v5.628a2.251 2.251 0 1 1-1.5 0V5a1 1 0 0 0-1-1h-1v1.646a.25.25 0 0 1-.427.177L7.177 3.427a.25.25 0 0 1 0-.354ZM3.75 2.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm0 9.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm8.25.75a.75.75 0 1 0 1.5 0 .75.75 0 0 0-1.5 0Z" />
        </svg>
      );
    case "failed":
      return (
        <svg className={cn(cls, "text-red-400")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <circle cx="8" cy="8" r="6.5" />
          <path d="M5.5 5.5l5 5M10.5 5.5l-5 5" />
        </svg>
      );
    default:
      return (
        <svg className={cn(cls, "text-zinc-500")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="6.5" />
          <path d="M5 8l2 2 4-4" />
        </svg>
      );
  }
}

function PhaseLabel({ phase }: { phase: Phase }) {
  const base = "text-xs font-medium px-2.5 py-1 rounded-md border shrink-0";

  switch (phase) {
    case "awaiting_review":
      return <span className={cn(base, "text-blue-400 bg-blue-500/10 border-blue-500/20")}>Awaiting review</span>;
    case "in_progress":
      return <span className={cn(base, "text-amber-400 bg-amber-500/10 border-amber-500/20")}>Running</span>;
    case "open":
      return <span className={cn(base, "text-emerald-400 bg-emerald-500/10 border-emerald-500/20")}>PR open</span>;
    case "merged":
      return <span className={cn(base, "text-purple-400 bg-purple-500/10 border-purple-500/20")}>Merged</span>;
    case "failed":
      return <span className={cn(base, "text-red-400 bg-red-500/10 border-red-500/20")}>Failed</span>;
    default:
      return <span className={cn(base, "text-emerald-400 bg-emerald-500/10 border-emerald-500/20")}>Done</span>;
  }
}

function MergeAction({ onMerge }: { onMerge: () => void }) {
  const [pending, setPending] = useState(false);

  return (
    <div className="flex items-center gap-3">
      <Button
        size="sm"
        variant="success"
        disabled={pending}
        onClick={() => { setPending(true); onMerge(); }}
      >
        {pending ? "Sending..." : "Mark as merged"}
      </Button>
      <span className="text-xs text-zinc-600">PR merged but no webhook? Use this to advance the run.</span>
    </div>
  );
}

function RetryAction({ onRetry }: { onRetry: () => void }) {
  const [pending, setPending] = useState(false);

  return (
    <div className="flex items-center gap-3">
      <Button
        size="sm"
        variant="default"
        disabled={pending}
        onClick={() => { setPending(true); onRetry(); }}
      >
        {pending ? "Retrying..." : "Retry"}
      </Button>
      <span className="text-xs text-zinc-600">Re-dispatch this step to the worker.</span>
    </div>
  );
}

function ReviewActions({ onComplete }: { onComplete: (success: boolean) => void }) {
  const [pending, setPending] = useState(false);

  const handle = (success: boolean) => {
    setPending(true);
    onComplete(success);
  };

  return (
    <div className="flex items-center gap-2">
      <Button size="sm" variant="success" disabled={pending} onClick={() => handle(true)}>
        {pending ? "Sending..." : "Mark as done"}
      </Button>
      <Button size="sm" variant="danger" disabled={pending} onClick={() => handle(false)}>
        Mark as failed
      </Button>
    </div>
  );
}
