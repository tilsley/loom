"use client";

import { useState, useEffect, useRef } from "react";
import type { StepState } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button, buttonVariants } from "@/components/ui";

export function StepTimeline({
  results,
  stepDescriptions,
  onComplete,
  onRetry,
}: {
  results: StepState[];
  stepDescriptions?: Map<string, string>;
  onComplete?: (stepName: string, candidateId: string, status: string) => void;
  onRetry?: (stepName: string, candidateId: string) => void;
}) {
  const lastActiveIndex = results.reduce((acc, r, idx) => {
    const p = r.status;
    return p === "pending" || p === "in_progress" ? idx : acc;
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
        const phase = r.status;
        const meta = r.metadata ?? {};
        const description = stepDescriptions?.get(r.stepName);
        const isLast = i === results.length - 1;
        const isActive = phase === "pending" || phase === "in_progress";
        const hasPR = phase === "pending" && Boolean(meta.prUrl);
        const hasReview = phase === "pending" && Boolean(meta.instructions);

        const lineColor =
          phase === "succeeded" || phase === "merged"
            ? "bg-teal-500/25"
            : phase === "failed"
              ? "bg-red-500/20"
              : "bg-zinc-800";

        return (
          <div
            key={i}
            ref={i === lastActiveIndex ? activeRef : undefined}
            className={cn(
              "relative flex gap-5",
              !isActive && !isLast && "pb-6",
              isActive && !isLast && "mb-6",
              isActive && "px-3 py-3 rounded-lg",
              hasReview && "bg-blue-500/10 border border-blue-500/25",
              hasPR && "bg-emerald-500/[0.07] border border-emerald-500/20",
              phase === "in_progress" && "bg-amber-500/[0.07] border border-amber-500/20",
            )}
          >
            {/* Timeline column: dot + connecting line */}
            <div className="flex flex-col items-center flex-shrink-0 w-5">
              <TimelineDot phase={phase} meta={meta} />
              {!isLast && <div className={cn("w-px flex-1 mt-1", lineColor)} />}
            </div>

            {/* Content */}
            <div className="flex-1 min-w-0 pt-px">
              {/* Top row: name/description left, badges right */}
              <div className="flex items-start gap-4">
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

                <div className="flex flex-col items-end gap-1.5 shrink-0">
                  {meta.prUrl ? (
                    <a
                      href={meta.prUrl}
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
                  <PhaseLabel phase={phase} meta={meta} />
                </div>
              </div>

              {/* Pending with PR: manual merge action for local dev / no-webhook environments */}
              {hasPR && onComplete ? (
                <div className="mt-3">
                  <MergeAction onMerge={() => onComplete(r.stepName, r.candidate.id, "merged")} />
                </div>
              ) : null}

              {/* Failed step: retry action */}
              {phase === "failed" && onRetry ? (
                <div className="mt-3">
                  <RetryAction onRetry={() => onRetry(r.stepName, r.candidate.id)} />
                </div>
              ) : null}

              {/* Pending with review instructions: instructions + actions */}
              {hasReview ? (
                <div className="mt-3 space-y-3">
                  <div className="bg-blue-500/5 border border-blue-500/15 rounded-md px-3 py-2.5">
                    <div className="text-xs font-medium text-blue-400/70 uppercase tracking-widest mb-1.5">
                      Instructions
                    </div>
                    <ul className="space-y-1">
                      {meta.instructions?.split("\n").map((line, j) => (
                        <li key={j} className="text-sm text-blue-200/80 font-mono">
                          {line}
                        </li>
                      ))}
                    </ul>
                  </div>
                  {onComplete ? (
                    <ReviewActions
                      onComplete={(status) => onComplete(r.stepName, r.candidate.id, status)}
                    />
                  ) : null}
                </div>
              ) : null}

              {/* Extra metadata tags */}
              {r.metadata
                ? (() => {
                    const extra = Object.entries(r.metadata).filter(
                      ([k]) => k !== "prUrl" && k !== "instructions" && k !== "commitSha",
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

function TimelineDot({ phase, meta }: { phase: StepState["status"]; meta: Record<string, string> }) {
  const cls = "w-4 h-4 flex-shrink-0 mt-0.5";

  switch (phase) {
    case "succeeded":
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
    case "pending":
      if (meta.instructions) {
        // Awaiting human review
        return (
          <svg className={cn(cls, "text-blue-400")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M1 8s3-5.5 7-5.5S15 8 15 8s-3 5.5-7 5.5S1 8 1 8z" />
            <circle cx="8" cy="8" r="2" fill="currentColor" stroke="none" />
          </svg>
        );
      }
      if (meta.prUrl) {
        // PR open, awaiting merge
        return (
          <svg className={cn(cls, "text-emerald-400")} viewBox="0 0 16 16" fill="currentColor">
            <path d="M1.5 3.25a2.25 2.25 0 1 1 3 2.122v5.256a2.251 2.251 0 1 1-1.5 0V5.372A2.25 2.25 0 0 1 1.5 3.25Zm5.677-.177L9.573.677A.25.25 0 0 1 10 .854V2.5h1A2.5 2.5 0 0 1 13.5 5v5.628a2.251 2.251 0 1 1-1.5 0V5a1 1 0 0 0-1-1h-1v1.646a.25.25 0 0 1-.427.177L7.177 3.427a.25.25 0 0 1 0-.354ZM3.75 2.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm0 9.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm8.25.75a.75.75 0 1 0 1.5 0 .75.75 0 0 0-1.5 0Z" />
          </svg>
        );
      }
      // Generic pending
      return (
        <svg className={cn(cls, "text-zinc-400")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <circle cx="8" cy="8" r="6.5" strokeDasharray="28" strokeDashoffset="8" />
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

function PhaseLabel({ phase, meta }: { phase: StepState["status"]; meta: Record<string, string> }) {
  const base = "text-xs font-medium px-2.5 py-1 rounded-md border shrink-0";

  switch (phase) {
    case "pending":
      if (meta.instructions) {
        return <span className={cn(base, "text-blue-400 bg-blue-500/10 border-blue-500/20")}>Awaiting review</span>;
      }
      if (meta.prUrl) {
        return <span className={cn(base, "text-emerald-400 bg-emerald-500/10 border-emerald-500/20")}>PR open</span>;
      }
      return <span className={cn(base, "text-zinc-400 bg-zinc-500/10 border-zinc-500/20")}>Pending</span>;
    case "in_progress":
      return <span className={cn(base, "text-amber-400 bg-amber-500/10 border-amber-500/20")}>Running</span>;
    case "merged":
      return <span className={cn(base, "text-purple-400 bg-purple-500/10 border-purple-500/20")}>Merged</span>;
    case "failed":
      return <span className={cn(base, "text-red-400 bg-red-500/10 border-red-500/20")}>Failed</span>;
    default:
      return <span className={cn(base, "text-emerald-400 bg-emerald-500/10 border-emerald-500/20")}>Done</span>;
  }
}

function MergeAction({ onMerge }: { onMerge: () => void }) {
  const [isPending, setIsPending] = useState(false);

  return (
    <div className="flex items-center gap-3">
      <Button
        size="sm"
        variant="success"
        disabled={isPending}
        onClick={() => { setIsPending(true); onMerge(); }}
      >
        {isPending ? "Sending..." : "Mark as merged"}
      </Button>
      <span className="text-xs text-zinc-600">PR merged but no webhook? Use this to advance the run.</span>
    </div>
  );
}

function RetryAction({ onRetry }: { onRetry: () => void }) {
  const [isPending, setIsPending] = useState(false);

  return (
    <div className="flex items-center gap-3">
      <Button
        size="sm"
        variant="default"
        disabled={isPending}
        onClick={() => { setIsPending(true); onRetry(); }}
      >
        {isPending ? "Retrying..." : "Retry"}
      </Button>
      <span className="text-xs text-zinc-600">Re-dispatch this step to the worker.</span>
    </div>
  );
}

function ReviewActions({ onComplete }: { onComplete: (status: string) => void }) {
  const [isPending, setIsPending] = useState(false);

  const handle = (status: string) => {
    setIsPending(true);
    onComplete(status);
  };

  return (
    <div className="flex items-center gap-2">
      <Button size="sm" variant="success" disabled={isPending} onClick={() => handle("succeeded")}>
        {isPending ? "Sending..." : "Mark as done"}
      </Button>
      <Button size="sm" variant="danger" disabled={isPending} onClick={() => handle("failed")}>
        Mark as failed
      </Button>
    </div>
  );
}
