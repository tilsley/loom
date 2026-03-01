"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import type { StepState } from "@/lib/api";
import { cn } from "@/lib/utils";
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
  Button,
  buttonVariants,
} from "@/components/ui";

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

  // Non-collapsible: everything that isn't succeeded/merged
  const forcedIndices = useCallback(() => {
    const forced: string[] = [];
    for (let i = 0; i < results.length; i++) {
      const s = results[i].status;
      if (s !== "succeeded" && s !== "merged") {
        forced.push(String(i));
      }
    }
    return forced;
  }, [results]);

  const [value, setValue] = useState<string[]>(() => forcedIndices());

  const handleValueChange = useCallback(
    (next: string[]) => {
      // Re-add forced indices so non-collapsible steps can't be closed
      const forced = new Set(forcedIndices());
      const merged = new Set(next);
      for (const f of forced) merged.add(f);
      setValue(Array.from(merged));
    },
    [forcedIndices],
  );

  // Sync when results change (new steps appear, statuses change)
  useEffect(() => {
    setValue((prev) => {
      const forced = new Set(forcedIndices());
      const merged = new Set(prev);
      for (const f of forced) merged.add(f);
      return Array.from(merged);
    });
  }, [forcedIndices]);

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
        <p className="text-xs text-muted-foreground/70 mt-1">
          Steps will appear here as workers report progress
        </p>
      </div>
    );
  }

  return (
    <Accordion type="multiple" value={value} onValueChange={handleValueChange}>
      {results.map((r, i) => {
        const phase = r.status;
        const meta = r.metadata ?? {};
        const description = stepDescriptions?.get(r.stepName);
        const isLast = i === results.length - 1;
        const isActive = phase === "pending" || phase === "in_progress";
        const hasPR = phase === "pending" && Boolean(meta.prUrl);
        const hasReview = phase === "pending" && Boolean(meta.instructions);
        const isDone = phase === "succeeded" || phase === "merged";
        const isCollapsible = isDone;
        const isOpen = value.includes(String(i));

        const lineColor =
          phase === "succeeded" || phase === "merged"
            ? "bg-primary/25"
            : phase === "failed"
              ? "bg-destructive/20"
              : "bg-border";

        return (
          <AccordionItem
            key={i}
            value={String(i)}
            ref={i === lastActiveIndex ? activeRef : undefined}
            className={cn(
              "relative flex gap-5",
              !isOpen
                ? (!isLast && "pb-1")
                : cn(
                    !isActive && !isLast && "pb-6",
                    isActive && !isLast && "mb-6",
                  ),
              isActive && "px-3 py-3 rounded-lg",
              hasReview && "bg-pending/10 border border-pending/25",
              hasPR && "bg-completed/[0.07] border border-completed/20",
              phase === "in_progress" && "bg-running/[0.07] border border-running/20",
            )}
          >
            {/* Timeline column: dot + connecting line */}
            <div className="flex flex-col items-center flex-shrink-0 w-5">
              <TimelineDot phase={phase} meta={meta} />
              {!isLast && <div className={cn("w-px flex-1 mt-1", lineColor)} />}
            </div>

            {/* Content column */}
            <div className="flex-1 min-w-0 pt-px">
              {/* Trigger: always-visible row */}
              <AccordionTrigger
                className={cn(
                  "gap-4",
                  isCollapsible && "cursor-pointer group/step",
                  !isCollapsible && "pointer-events-none",
                )}
              >
                <div className="flex items-start gap-4 flex-1 min-w-0">
                  <div className="flex-1 min-w-0">
                    <span className={cn(
                      "font-medium font-mono block transition-colors",
                      !isOpen && isDone ? "text-sm text-muted-foreground group-hover/step:text-foreground/80" : "text-base text-foreground",
                    )}>
                      {isCollapsible ? (
                        <span className={cn(
                          "text-xs mr-0.5 select-none transition-colors",
                          !isOpen ? "text-muted-foreground/50 group-hover/step:text-muted-foreground" : "text-muted-foreground/70",
                        )}>
                          {isOpen ? "▾" : "▸"}
                        </span>
                      ) : null}
                      <span className="text-xs text-muted-foreground/50 mr-1.5 tabular-nums select-none">
                        {String(i + 1).padStart(2, "0")}.
                      </span>
                      {r.stepName}
                    </span>
                    {Boolean(description) && (
                      <span className={cn(
                        "text-muted-foreground mt-0.5 block",
                        !isOpen && isDone ? "text-xs" : "text-sm",
                      )}>
                        {description}
                      </span>
                    )}
                  </div>

                  <div className="flex flex-col items-end gap-1.5 shrink-0">
                    <PhaseLabel phase={phase} meta={meta} />
                  </div>
                </div>
              </AccordionTrigger>

              {/* Expandable content */}
              <AccordionContent>
                <div className="pt-1">
                  {/* Step description */}
                  {description ? (
                    <p className="text-sm text-muted-foreground mb-3">{description}</p>
                  ) : null}

                  {/* PR link */}
                  {meta.prUrl ? (
                    <div className="mb-2">
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
                    </div>
                  ) : null}

                  {/* Pending with PR: manual merge action */}
                  {hasPR && onComplete ? (
                    <div className="mt-2">
                      <MergeAction onMerge={() => onComplete(r.stepName, r.candidate.id, "merged")} />
                    </div>
                  ) : null}

                  {/* Failed step: retry action */}
                  {phase === "failed" && onRetry ? (
                    <div className="mt-2">
                      <RetryAction onRetry={() => onRetry(r.stepName, r.candidate.id)} />
                    </div>
                  ) : null}

                  {/* Pending with review instructions */}
                  {hasReview ? (
                    <div className="mt-2 space-y-3">
                      <div className="bg-pending/5 border border-pending/15 rounded-md px-3 py-2.5">
                        <div className="text-xs font-medium text-pending/70 uppercase tracking-widest mb-1.5">
                          Instructions
                        </div>
                        <ul className="space-y-1">
                          {meta.instructions?.split("\n").map((line, j) => (
                            <li key={j} className="text-sm text-foreground/80 font-mono">
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
                                className="inline-flex items-center gap-1.5 text-xs font-mono text-muted-foreground bg-muted px-2 py-0.5 rounded"
                              >
                                <span className="text-muted-foreground/70">{k}</span>
                                <span className="text-muted-foreground">{v}</span>
                              </span>
                            ))}
                          </div>
                        );
                      })()
                    : null}
                </div>
              </AccordionContent>
            </div>
          </AccordionItem>
        );
      })}
    </Accordion>
  );
}

function TimelineDot({ phase, meta }: { phase: StepState["status"]; meta: Record<string, string> }) {
  const cls = "w-4 h-4 flex-shrink-0 mt-0.5";

  switch (phase) {
    case "succeeded":
      return (
        <svg className={cn(cls, "text-primary")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="6.5" />
          <path d="M5 8l2 2 4-4" />
        </svg>
      );
    case "merged":
      return (
        <svg className={cn(cls, "text-merged")} viewBox="0 0 16 16" fill="currentColor">
          <path d="M5 3.25a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0zm0 2.122a2.25 2.25 0 1 0-1.5 0v.878A2.25 2.25 0 0 0 5.75 8.5h1.5v2.128a2.251 2.251 0 1 0 1.5 0V8.5h1.5a2.25 2.25 0 0 0 2.25-2.25v-.878a2.25 2.25 0 1 0-1.5 0v.878a.75.75 0 0 1-.75.75h-4.5A.75.75 0 0 1 5 6.25v-.878zm3.75 7.378a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0zm3-8.75a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0z" />
        </svg>
      );
    case "in_progress":
      return (
        <svg className={cn(cls, "text-running animate-spin")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <circle cx="8" cy="8" r="6.5" strokeDasharray="30" strokeDashoffset="10" />
        </svg>
      );
    case "pending":
      if (meta.instructions) {
        return (
          <svg className={cn(cls, "text-pending")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M1 8s3-5.5 7-5.5S15 8 15 8s-3 5.5-7 5.5S1 8 1 8z" />
            <circle cx="8" cy="8" r="2" fill="currentColor" stroke="none" />
          </svg>
        );
      }
      if (meta.prUrl) {
        return (
          <svg className={cn(cls, "text-completed")} viewBox="0 0 16 16" fill="currentColor">
            <path d="M1.5 3.25a2.25 2.25 0 1 1 3 2.122v5.256a2.251 2.251 0 1 1-1.5 0V5.372A2.25 2.25 0 0 1 1.5 3.25Zm5.677-.177L9.573.677A.25.25 0 0 1 10 .854V2.5h1A2.5 2.5 0 0 1 13.5 5v5.628a2.251 2.251 0 1 1-1.5 0V5a1 1 0 0 0-1-1h-1v1.646a.25.25 0 0 1-.427.177L7.177 3.427a.25.25 0 0 1 0-.354ZM3.75 2.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm0 9.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm8.25.75a.75.75 0 1 0 1.5 0 .75.75 0 0 0-1.5 0Z" />
          </svg>
        );
      }
      return (
        <svg className={cn(cls, "text-muted-foreground")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <circle cx="8" cy="8" r="6.5" strokeDasharray="28" strokeDashoffset="8" />
        </svg>
      );
    case "failed":
      return (
        <svg className={cn(cls, "text-destructive")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <circle cx="8" cy="8" r="6.5" />
          <path d="M5.5 5.5l5 5M10.5 5.5l-5 5" />
        </svg>
      );
    default:
      return (
        <svg className={cn(cls, "text-muted-foreground")} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
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
        return <span className={cn(base, "text-pending bg-pending/10 border-pending/20")}>Awaiting review</span>;
      }
      if (meta.prUrl) {
        return <span className={cn(base, "text-completed bg-completed/10 border-completed/20")}>PR open</span>;
      }
      return <span className={cn(base, "text-muted-foreground bg-muted border-border")}>Pending</span>;
    case "in_progress":
      return <span className={cn(base, "text-running bg-running/10 border-running/20")}>Running</span>;
    case "merged":
      return <span className={cn(base, "text-merged bg-merged/10 border-merged/20")}>Merged</span>;
    case "failed":
      return <span className={cn(base, "text-destructive bg-destructive/10 border-destructive/20")}>Failed</span>;
    default:
      return <span className={cn(base, "text-completed bg-completed/10 border-completed/20")}>Done</span>;
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
      <span className="text-xs text-muted-foreground/70">PR merged but no webhook? Use this to advance the run.</span>
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
      <span className="text-xs text-muted-foreground/70">Re-dispatch this step to the worker.</span>
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
