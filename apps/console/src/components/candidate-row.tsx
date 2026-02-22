import { useState } from "react";
import Link from "next/link";
import type { Candidate, CandidateRun } from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { Button } from "@/components/ui";

interface CandidateRowProps {
  candidate: Candidate;
  candidateRun?: CandidateRun;
  runId?: string;
  onQueue: (candidate: Candidate) => Promise<void>;
  onDequeue: (runId: string) => Promise<void>;
  isRunning: boolean;
}

export function CandidateRow({ candidate, candidateRun, runId, onQueue, onDequeue, isRunning }: CandidateRowProps) {
  const isBlocked = candidateRun?.status === "running" || candidateRun?.status === "completed";
  const hasRun = !!runId;
  const [filesExpanded, setFilesExpanded] = useState(false);
  const fileGroups = candidate.files ?? [];
  const totalFiles = fileGroups.reduce((n, g) => n + g.files.length, 0);

  const row = (
    <div
      className={`grid grid-cols-[1fr_100px_1fr_80px] gap-x-2 px-4 py-3.5 ${
        filesExpanded ? "items-start" : "items-center"
      } ${hasRun ? "group-hover/row:bg-zinc-800/30" : ""}`}
    >
      {/* Candidate name */}
      <div className="flex items-center gap-2 min-w-0 flex-wrap">
        <CandidateStatusDot status={candidateRun?.status} />
        <span className="text-sm font-mono text-zinc-300 truncate">{candidate.id}</span>
        {totalFiles > 0 && (
          <button
            onClick={(e) => { e.preventDefault(); e.stopPropagation(); setFilesExpanded((v) => !v); }}
            className="inline-flex items-center gap-1.5 text-xs font-mono text-zinc-500 hover:text-zinc-300 transition-colors shrink-0"
          >
            <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor" className="opacity-60">
              <path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z" />
            </svg>
            {totalFiles} files
            <svg width="10" height="10" viewBox="0 0 12 12" fill="none" className={`transition-transform ${filesExpanded ? "rotate-180" : ""}`}>
              <path d="M2 4l4 4 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
        )}
      </div>

      {/* Status */}
      <div>
        {candidateRun ? (
          <CandidateStatusBadge status={candidateRun.status} />
        ) : (
          <span className="text-xs text-zinc-600">not started</span>
        )}
      </div>

      {/* Run ID + view link */}
      <div className="flex items-center gap-2 min-w-0">
        {hasRun ? (
          <>
            <span className="text-xs font-mono text-zinc-500 truncate">{runId}</span>
            <span className="inline-flex items-center gap-1 text-xs text-zinc-600 group-hover/row:text-teal-400 transition-colors shrink-0">
              View
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none">
                <path
                  d="M5 3l4 4-4 4"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </span>
          </>
        ) : (
          <span className="text-xs text-zinc-700">&mdash;</span>
        )}
      </div>

      {/* Actions — stop click from navigating */}
      <div className="flex justify-end" onClick={(e) => e.preventDefault()}>
        {candidateRun?.status === "queued" ? (
          <Button
            size="sm"
            variant="outline"
            onClick={(e) => {
              e.stopPropagation();
              void onDequeue(runId ?? "");
            }}
            disabled={isRunning}
            className="text-xs py-1 px-2.5 text-zinc-400 hover:text-red-400 hover:border-red-500/30"
          >
            {isRunning ? "..." : "Cancel"}
          </Button>
        ) : (
          <Button
            size="sm"
            variant={isBlocked ? "outline" : "default"}
            onClick={(e) => {
              e.stopPropagation();
              void onQueue(candidate);
            }}
            disabled={isRunning || isBlocked}
            className={`text-xs py-1 px-2.5 ${isBlocked ? "cursor-not-allowed" : ""}`}
          >
            {!isBlocked && (
              <svg width="8" height="8" viewBox="0 0 12 12" fill="none">
                <path d="M3 2l7 4-7 4V2z" fill="currentColor" />
              </svg>
            )}
            {isRunning
              ? "..."
              : candidateRun?.status === "completed"
                ? "Done"
                : candidateRun?.status === "running"
                  ? "Running"
                  : "Queue"}
          </Button>
        )}
      </div>
      {/* Files panel */}
      {filesExpanded && fileGroups.length > 0 ? <div
          className="col-span-4 border-t border-zinc-800/60 pt-2.5 pb-1 mt-1"
          onClick={(e) => { e.preventDefault(); e.stopPropagation(); }}
        >
          <div className="flex flex-wrap gap-x-6 gap-y-3">
            {fileGroups.map((group) => (
              <div key={group.name} className="min-w-0">
                <div className="flex items-center gap-1.5 mb-1">
                  <span className="text-xs font-medium text-zinc-500 uppercase tracking-wider">
                    {group.name}
                  </span>
                  <span className="text-xs font-mono text-zinc-700">{group.repo}</span>
                </div>
                <div className="space-y-0.5">
                  {group.files.map((f) => (
                    <a
                      key={f.path}
                      href={f.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 text-xs font-mono text-zinc-500 hover:text-teal-400 transition-colors"
                    >
                      <svg width="9" height="9" viewBox="0 0 16 16" fill="currentColor" className="shrink-0 opacity-50">
                        <path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z" />
                      </svg>
                      {f.path}
                    </a>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div> : null}
    </div>
  );

  if (hasRun) {
    return (
      <Link
        href={ROUTES.runDetail(runId ?? "")}
        className="group/row block bg-zinc-900/50 border border-zinc-800/80 hover:border-zinc-700 rounded-lg transition-all cursor-pointer"
      >
        {row}
      </Link>
    );
  }

  return <div className="bg-zinc-900/50 border border-zinc-800/80 rounded-lg">{row}</div>;
}

function CandidateStatusDot({ status }: { status?: string }) {
  const color =
    status === "running"
      ? "bg-amber-400"
      : status === "completed"
        ? "bg-emerald-400"
        : status === "queued"
          ? "bg-indigo-400"
          : "bg-teal-500/50";

  return (
    <span
      className={`w-1.5 h-1.5 rounded-full shrink-0 ${color} ${
        status === "running" ? "animate-pulse" : ""
      }`}
    />
  );
}

function CandidateStatusBadge({ status }: { status: string }) {
  const styles =
    status === "running"
      ? "bg-amber-500/10 text-amber-400 border-amber-500/20"
      : status === "completed"
        ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
        : status === "queued"
          ? "bg-indigo-500/10 text-indigo-400 border-indigo-500/20"
          : "bg-zinc-500/10 text-zinc-400 border-zinc-500/20";

  return (
    <span className={`text-xs font-medium px-2 py-0.5 rounded border ${styles}`}>
      {status}
    </span>
  );
}
