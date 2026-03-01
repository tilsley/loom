"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { Candidate } from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { Button, Badge, TableRow, TableCell } from "@/components/ui";

interface CandidateRowProps {
  candidate: Candidate;
  migrationId?: string;
  onPreview: (candidate: Candidate) => void;
  onCancel?: (candidate: Candidate) => void;
  isRunning: boolean;
  metaColumns: string[];
  onMetaFilter: (key: string, value: string) => void;
  colCount: number;
}

export function CandidateRow({
  candidate,
  migrationId,
  onPreview,
  onCancel,
  isRunning,
  metaColumns,
  onMetaFilter,
  colCount,
}: CandidateRowProps) {
  const router = useRouter();
  const status = candidate.status;
  const hasRun = !!migrationId;
  const [filesExpanded, setFilesExpanded] = useState(false);
  const fileGroups = candidate.files ?? [];
  const totalFiles = fileGroups.reduce((n, g) => n + g.files.length, 0);

  function handleRowClick() {
    if (hasRun && migrationId) {
      router.push(ROUTES.candidateSteps(migrationId, candidate.id));
    }
  }

  return (
    <>
      <TableRow
        onClick={handleRowClick}
        className={hasRun ? "cursor-pointer hover:bg-muted/25" : ""}
      >
        {/* Candidate ID */}
        <TableCell>
          <div className="flex items-center gap-2">
            <CandidateStatusDot status={status} />
            <span className="text-sm font-mono text-foreground/80">{candidate.id}</span>
          </div>
        </TableCell>

        {/* Metadata columns */}
        {metaColumns.map((key) => {
          const val = candidate.metadata?.[key];
          return (
            <TableCell key={key}>
              {val ? (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onMetaFilter(key, val);
                  }}
                  className="text-xs font-mono text-muted-foreground hover:text-foreground hover:bg-muted px-1.5 py-0.5 rounded transition-colors"
                >
                  {val}
                </button>
              ) : (
                <span className="text-xs text-muted-foreground/50">—</span>
              )}
            </TableCell>
          );
        })}

        {/* Files */}
        <TableCell>
          {totalFiles > 0 ? (
            <button
              onClick={(e) => {
                e.stopPropagation();
                setFilesExpanded((v) => !v);
              }}
              className="inline-flex items-center gap-1 text-xs font-mono text-muted-foreground hover:text-foreground/80 transition-colors"
            >
              <svg width="11" height="11" viewBox="0 0 16 16" fill="currentColor" className="opacity-60 shrink-0">
                <path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z" />
              </svg>
              {totalFiles}
              <svg
                width="9"
                height="9"
                viewBox="0 0 12 12"
                fill="none"
                className={`transition-transform ${filesExpanded ? "rotate-180" : ""}`}
              >
                <path d="M2 4l4 4 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
          ) : (
            <span className="text-xs text-muted-foreground/50">—</span>
          )}
        </TableCell>

        {/* Status */}
        <TableCell>
          {status && status !== "not_started" ? (
            <Badge variant={status === "running" ? "running" : status === "completed" ? "completed" : "default"}>
              {status}
            </Badge>
          ) : (
            <span className="text-xs text-muted-foreground/50">—</span>
          )}
        </TableCell>

        {/* Actions */}
        <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
          {status === "running" && onCancel ? (
            <Button size="sm" variant="danger" onClick={() => onCancel(candidate)} className="text-xs py-1 px-2.5">
              Cancel
            </Button>
          ) : (
            <Button
              size="sm"
              variant={status === "completed" || status === "running" ? "outline" : "default"}
              onClick={() => onPreview(candidate)}
              disabled={isRunning || status === "completed" || status === "running"}
              className="text-xs py-1 px-2.5"
            >
              {isRunning ? "..." : status === "completed" ? "Done" : status === "running" ? "Running" : "Preview"}
            </Button>
          )}
        </TableCell>
      </TableRow>

      {/* Files expansion row */}
      {filesExpanded && fileGroups.length > 0 ? (
        <TableRow>
          <TableCell colSpan={colCount} className="bg-muted/50 py-3">
            <p className="text-xs text-muted-foreground mb-2.5">
              Files detected by the discovery scanner · the dry run shows what will change
            </p>
            <div className="flex flex-wrap gap-x-8 gap-y-3">
              {fileGroups.map((group) => (
                <div key={group.name} className="min-w-0">
                  <div className="flex items-center gap-1.5 mb-1">
                    <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{group.name}</span>
                    <span className="text-xs font-mono text-muted-foreground/70">{group.repo}</span>
                  </div>
                  <div className="space-y-0.5">
                    {group.files.map((f) => (
                      <a
                        key={f.path}
                        href={f.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-1 text-xs font-mono text-muted-foreground hover:text-primary transition-colors"
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
          </TableCell>
        </TableRow>
      ) : null}
    </>
  );
}

function CandidateStatusDot({ status }: { status?: string }) {
  const color =
    status === "running"
      ? "bg-running"
      : status === "completed"
        ? "bg-completed"
        : "bg-muted-foreground/50";

  return (
    <span
      className={`w-1.5 h-1.5 rounded-full shrink-0 ${color} ${
        status === "running" ? "animate-pulse" : ""
      }`}
    />
  );
}

