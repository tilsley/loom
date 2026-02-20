import { useState } from "react";
import type { StepResult, Target } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui";

type Phase = "in_progress" | "open" | "merged" | "failed" | "completed" | "awaiting_review";

function getPhase(r: StepResult): Phase {
  if (!r.success) return "failed";
  const phase = r.metadata?.phase;
  if (
    phase === "in_progress" ||
    phase === "open" ||
    phase === "merged" ||
    phase === "awaiting_review"
  )
    return phase;
  return "completed";
}

function resolveFileUrls(urls: string[], target: Target): string[] {
  const appName = target.metadata?.appName ?? target.repo.split("/").pop() ?? "";
  return urls.map((u) => u.replace(/\{appName\}/g, appName).replace(/\{repo\}/g, target.repo));
}

export function StepTimeline({
  results,
  stepDescriptions,
  stepFiles,
  onComplete,
}: {
  results: StepResult[];
  stepDescriptions?: Map<string, string>;
  stepFiles?: Map<string, string[]>;
  onComplete?: (stepName: string, target: Target, success: boolean) => void;
}) {
  if (results.length === 0) {
    return (
      <div className="border border-dashed border-zinc-800 rounded-lg py-12 text-center">
        <div className="w-10 h-10 rounded-lg bg-zinc-900 flex items-center justify-center mx-auto mb-3">
          <svg
            className="w-5 h-5 text-zinc-600 animate-pulse-subtle"
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
        <p className="text-sm text-zinc-500">Waiting for worker callbacks...</p>
        <p className="text-xs text-zinc-600 mt-1">
          Steps will appear here as workers report progress
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-zinc-800/80 overflow-hidden">
      {results.map((r, i) => {
        const phase = getPhase(r);
        const description = stepDescriptions?.get(r.stepName);
        const files = stepFiles?.get(r.stepName)
          ? resolveFileUrls(stepFiles.get(r.stepName)!, r.target)
          : [];
        const isLast = i === results.length - 1;

        return (
          <div key={i} className={cn("flex flex-col", !isLast && "border-b border-zinc-800/60")}>
            {/* Main row */}
            <div
              className={cn(
                "flex items-center gap-4 px-4 py-3",
                phase === "failed" && "bg-red-500/5",
                phase === "awaiting_review" && "bg-blue-500/5",
              )}
            >
              {/* Icon */}
              <div className="shrink-0">
                <StepIcon phase={phase} />
              </div>

              {/* Name + description + files */}
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium text-zinc-100 truncate">{r.stepName}</div>
                {Boolean(description) && (
                  <div className="text-xs text-zinc-500 truncate mt-0.5">{description}</div>
                )}
                {files.length > 0 && (
                  <div className="flex flex-wrap gap-x-3 gap-y-0.5 mt-1">
                    {files.map((url) => {
                      const label = url.split("/blob/main/").pop() ?? url;
                      return (
                        <a
                          key={url}
                          href={url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-[11px] font-mono text-zinc-500 hover:text-teal-400 transition-colors"
                        >
                          <svg
                            width="11"
                            height="11"
                            viewBox="0 0 16 16"
                            fill="currentColor"
                            className="shrink-0 opacity-60"
                          >
                            <path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z" />
                          </svg>
                          {label}
                        </a>
                      );
                    })}
                  </div>
                )}
              </div>

              {/* Repo */}
              <span className="hidden sm:inline-flex items-center gap-1.5 text-xs font-mono text-zinc-500 shrink-0">
                <svg
                  width="12"
                  height="12"
                  viewBox="0 0 16 16"
                  fill="currentColor"
                  className="shrink-0"
                >
                  <path d="M2 2.5A2.5 2.5 0 0 1 4.5 0h8.75a.75.75 0 0 1 .75.75v12.5a.75.75 0 0 1-.75.75h-2.5a.75.75 0 0 1 0-1.5h1.75v-2h-8a1 1 0 0 0-.714 1.7.75.75 0 1 1-1.072 1.05A2.495 2.495 0 0 1 2 11.5Zm10.5-1h-8a1 1 0 0 0-1 1v6.708A2.486 2.486 0 0 1 4.5 9h8ZM5 12.25a.25.25 0 0 1 .25-.25h3.5a.25.25 0 0 1 .25.25v3.25a.25.25 0 0 1-.4.2l-1.45-1.087a.25.25 0 0 0-.3 0L5.4 15.7a.25.25 0 0 1-.4-.2Z" />
                </svg>
                {r.target.repo}
              </span>

              {/* PR button */}
              {r.metadata?.prUrl ? (
                <a
                  href={r.metadata.prUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs text-teal-400 bg-teal-500/10 border border-teal-500/20 rounded-md hover:bg-teal-500/15 hover:border-teal-500/30 transition-all shrink-0"
                >
                  Open PR
                  <svg
                    width="10"
                    height="10"
                    viewBox="0 0 12 12"
                    fill="none"
                    className="text-teal-500/60"
                  >
                    <path
                      d="M3.5 1.5h7v7M10 2L2 10"
                      stroke="currentColor"
                      strokeWidth="1.5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </a>
              ) : null}

              {/* Phase badge */}
              <PhaseLabel phase={phase} />
            </div>

            {/* Expanded: awaiting_review instructions + actions */}
            {phase === "awaiting_review" ? (
              <div className="px-4 pb-3 ml-10 space-y-3">
                {r.metadata?.instructions ? (
                  <div className="bg-blue-500/5 border border-blue-500/15 rounded-md px-3 py-2.5">
                    <div className="text-[10px] font-medium text-blue-400/70 uppercase tracking-widest mb-1.5">
                      Instructions
                    </div>
                    <ul className="space-y-1">
                      {r.metadata.instructions.split("\n").map((line, j) => (
                        <li key={j} className="text-sm text-blue-200/80 font-mono">
                          {line}
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : null}
                {onComplete ? (
                  <ReviewActions
                    onComplete={(success) => onComplete(r.stepName, r.target, success)}
                  />
                ) : null}
              </div>
            ) : null}

            {/* Extra metadata tags (rarely populated) */}
            {r.metadata
              ? (() => {
                  const extra = Object.entries(r.metadata).filter(
                    ([k]) =>
                      k !== "phase" && k !== "prUrl" && k !== "instructions" && k !== "commitSha",
                  );
                  if (extra.length === 0) return null;
                  return (
                    <div className="flex flex-wrap gap-1.5 px-4 pb-3 ml-10">
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
        );
      })}
    </div>
  );
}

function StepIcon({ phase }: { phase: Phase }) {
  const size = "w-5 h-5";

  switch (phase) {
    case "awaiting_review":
      return (
        <div className={`${size} flex items-center justify-center text-blue-400`}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
            <path d="M8 2c1.981 0 3.671.992 4.933 2.078 1.27 1.091 2.187 2.345 2.637 3.023a1.62 1.62 0 0 1 0 1.798c-.45.678-1.367 1.932-2.637 3.023C11.67 13.008 9.981 14 8 14c-1.981 0-3.671-.992-4.933-2.078C1.797 10.831.88 9.577.43 8.899a1.62 1.62 0 0 1 0-1.798c.45-.678 1.367-1.932 2.637-3.023C4.33 2.992 6.019 2 8 2ZM1.679 7.932a.12.12 0 0 0 0 .136c.411.622 1.241 1.75 2.366 2.717C5.176 11.758 6.527 12.5 8 12.5c1.473 0 2.825-.742 3.955-1.715 1.124-.967 1.954-2.096 2.366-2.717a.12.12 0 0 0 0-.136c-.412-.621-1.242-1.75-2.366-2.717C10.824 4.242 9.473 3.5 8 3.5c-1.473 0-2.824.742-3.955 1.715-1.124.967-1.954 2.096-2.366 2.717ZM8 10a2 2 0 1 1-.001-3.999A2 2 0 0 1 8 10Z" />
          </svg>
        </div>
      );

    case "in_progress":
      return (
        <div className={`${size} flex items-center justify-center`}>
          <svg className="w-4 h-4 text-amber-500 animate-spin" viewBox="0 0 16 16" fill="none">
            <circle
              cx="8"
              cy="8"
              r="6"
              stroke="currentColor"
              strokeWidth="2"
              strokeDasharray="28"
              strokeDashoffset="8"
              strokeLinecap="round"
            />
          </svg>
        </div>
      );

    case "open":
      return (
        <div className={`${size} flex items-center justify-center text-emerald-500`}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
            <path d="M1.5 3.25a2.25 2.25 0 1 1 3 2.122v5.256a2.251 2.251 0 1 1-1.5 0V5.372A2.25 2.25 0 0 1 1.5 3.25Zm5.677-.177L9.573.677A.25.25 0 0 1 10 .854V2.5h1A2.5 2.5 0 0 1 13.5 5v5.628a2.251 2.251 0 1 1-1.5 0V5a1 1 0 0 0-1-1h-1v1.646a.25.25 0 0 1-.427.177L7.177 3.427a.25.25 0 0 1 0-.354ZM3.75 2.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm0 9.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm8.25.75a.75.75 0 1 0 1.5 0 .75.75 0 0 0-1.5 0Z" />
          </svg>
        </div>
      );

    case "merged":
      return (
        <div className={`${size} flex items-center justify-center text-purple-400`}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
            <path d="M5.45 5.154A4.25 4.25 0 0 0 9.25 7.5h1.378a2.251 2.251 0 1 1 0 1.5H9.25A5.734 5.734 0 0 1 5 7.123v3.505a2.25 2.25 0 1 1-1.5 0V5.372a2.25 2.25 0 1 1 1.95-.218ZM4.25 13.5a.75.75 0 1 0 0-1.5.75.75 0 0 0 0 1.5Zm8.5-4.5a.75.75 0 1 0 0-1.5.75.75 0 0 0 0 1.5ZM5 3.25a.75.75 0 1 0 0 .005V3.25Z" />
          </svg>
        </div>
      );

    case "failed":
      return (
        <div className={`${size} flex items-center justify-center text-red-500`}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
            <path d="M3.72 3.72a.75.75 0 0 1 1.06 0L8 6.94l3.22-3.22a.749.749 0 0 1 1.275.326.749.749 0 0 1-.215.734L9.06 8l3.22 3.22a.749.749 0 0 1-.326 1.275.749.749 0 0 1-.734-.215L8 9.06l-3.22 3.22a.751.751 0 0 1-1.042-.018.751.751 0 0 1-.018-1.042L6.94 8 3.72 4.78a.75.75 0 0 1 0-1.06Z" />
          </svg>
        </div>
      );

    default:
      return (
        <div className={`${size} flex items-center justify-center text-emerald-500`}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
            <path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z" />
          </svg>
        </div>
      );
  }
}

function PhaseLabel({ phase }: { phase: Phase }) {
  const base = "text-[10px] font-medium px-2 py-0.5 rounded-full border shrink-0";

  switch (phase) {
    case "awaiting_review":
      return (
        <span className={`${base} text-blue-400 bg-blue-500/10 border-blue-500/20`}>review</span>
      );
    case "in_progress":
      return (
        <span className={`${base} text-amber-400 bg-amber-500/10 border-amber-500/20`}>
          running
        </span>
      );
    case "open":
      return (
        <span className={`${base} text-emerald-400 bg-emerald-500/10 border-emerald-500/20`}>
          PR open
        </span>
      );
    case "merged":
      return (
        <span className={`${base} text-purple-400 bg-purple-500/10 border-purple-500/20`}>
          merged
        </span>
      );
    case "failed":
      return <span className={`${base} text-red-400 bg-red-500/10 border-red-500/20`}>failed</span>;
    default:
      return (
        <span className={`${base} text-emerald-400 bg-emerald-500/10 border-emerald-500/20`}>
          done
        </span>
      );
  }
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
        {pending ? "Sending..." : "Approve"}
      </Button>
      <Button
        size="sm"
        variant="destructive"
        disabled={pending}
        onClick={() => handle(false)}
        className="bg-red-500/10 text-red-400 border-red-500/20 hover:bg-red-500/20"
      >
        Reject
      </Button>
    </div>
  );
}
