import Link from "next/link";
import type { Target, TargetRun } from "@/lib/api";
import { ROUTES } from "@/lib/routes";
import { Button } from "@/components/ui";

interface TargetRowProps {
  target: Target;
  targetRun?: TargetRun;
  onRun: (target: Target) => Promise<void>;
  isRunning: boolean;
}

export function TargetRow({ target, targetRun, onRun, isRunning }: TargetRowProps) {
  const isBlocked = targetRun?.status === "running" || targetRun?.status === "completed";
  const hasRun = !!targetRun?.runId;

  const row = (
    <div
      className={`grid grid-cols-[1fr_100px_1fr_80px] items-center gap-2 px-3 py-2.5 ${
        hasRun ? "group-hover/row:bg-zinc-800/30" : ""
      }`}
    >
      {/* Target name */}
      <div className="flex items-center gap-2 min-w-0">
        <TargetStatusDot status={targetRun?.status} />
        <span className="text-[13px] font-mono text-zinc-300 truncate">{target.repo}</span>
        {target.metadata && Object.keys(target.metadata).length > 0 ? (
          <div className="flex gap-1.5 ml-1">
            {Object.entries(target.metadata).map(([k, v]) => (
              <span
                key={k}
                className="text-[10px] font-mono text-zinc-500 bg-zinc-800/40 px-1.5 py-0.5 rounded"
              >
                {k}=<span className="text-zinc-400">{v}</span>
              </span>
            ))}
          </div>
        ) : null}
      </div>

      {/* Status */}
      <div>
        {targetRun ? (
          <TargetStatusBadge status={targetRun.status} />
        ) : (
          <span className="text-[10px] text-zinc-600">pending</span>
        )}
      </div>

      {/* Run ID + view link */}
      <div className="flex items-center gap-2 min-w-0">
        {hasRun ? (
          <>
            <span className="text-[11px] font-mono text-zinc-500 truncate">{targetRun.runId}</span>
            <span className="inline-flex items-center gap-1 text-[11px] text-zinc-600 group-hover/row:text-teal-400 transition-colors shrink-0">
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
          <span className="text-[11px] text-zinc-700">&mdash;</span>
        )}
      </div>

      {/* Actions — stop click from navigating */}
      <div className="flex justify-end" onClick={(e) => e.preventDefault()}>
        <Button
          size="sm"
          variant={isBlocked ? "outline" : "default"}
          onClick={(e) => {
            e.stopPropagation();
            void onRun(target);
          }}
          disabled={isRunning || isBlocked}
          className={`text-[11px] py-1 px-2.5 ${isBlocked ? "cursor-not-allowed" : ""}`}
        >
          {!isBlocked && (
            <svg width="8" height="8" viewBox="0 0 12 12" fill="none">
              <path d="M3 2l7 4-7 4V2z" fill="currentColor" />
            </svg>
          )}
          {isRunning
            ? "..."
            : targetRun?.status === "completed"
              ? "Done"
              : targetRun?.status === "running"
                ? "Running"
                : "Run"}
        </Button>
      </div>
    </div>
  );

  if (hasRun) {
    return (
      <Link
        href={ROUTES.runDetail(targetRun.runId)}
        className="group/row block bg-zinc-900/50 border border-zinc-800/80 hover:border-zinc-700 rounded-lg transition-all cursor-pointer"
      >
        {row}
      </Link>
    );
  }

  return <div className="bg-zinc-900/50 border border-zinc-800/80 rounded-lg">{row}</div>;
}

function TargetStatusDot({ status }: { status?: string }) {
  const color =
    status === "running"
      ? "bg-amber-400"
      : status === "completed"
        ? "bg-emerald-400"
        : status === "failed"
          ? "bg-red-400"
          : "bg-teal-500/50";

  return (
    <span
      className={`w-1.5 h-1.5 rounded-full shrink-0 ${color} ${
        status === "running" ? "animate-pulse" : ""
      }`}
    />
  );
}

function TargetStatusBadge({ status }: { status: string }) {
  const styles =
    status === "running"
      ? "bg-amber-500/10 text-amber-400 border-amber-500/20"
      : status === "completed"
        ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
        : "bg-red-500/10 text-red-400 border-red-500/20";

  return (
    <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded border ${styles}`}>
      {status}
    </span>
  );
}
