import Link from "next/link";
import type { Migration } from "@/lib/api";

function timeAgo(date: string): string {
  const seconds = Math.floor((Date.now() - new Date(date).getTime()) / 1000);
  if (seconds < 60) return "just now";
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

export function MigrationCard({ migration }: { migration: Migration }) {
  const kindPlural = (migration.candidates[0]?.kind ?? "candidate") + "s";
  const runCount = migration.candidates.filter(
    (c) => c.status === "running" || c.status === "completed",
  ).length;
  const hasRuns = runCount > 0;
  const doneCount = migration.candidates.filter((c) => c.status === "completed").length;

  return (
    <Link
      href={`/migrations/${migration.id}`}
      className="group relative block rounded-lg border border-zinc-800/80 bg-[var(--color-surface)] hover:border-zinc-700 hover:bg-[var(--color-surface-raised)] transition-all duration-200"
    >
      {/* Left accent */}
      <div className="absolute left-0 top-3 bottom-3 w-[3px] rounded-full bg-teal-500/40 group-hover:bg-teal-400/70 transition-colors" />

      <div className="pl-5 pr-4 py-3.5">
        {/* Top row: name + slug */}
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h3 className="text-[14px] font-semibold text-zinc-100 group-hover:text-white truncate transition-colors">
              {migration.name}
            </h3>
            <p className="text-xs text-zinc-500 mt-0.5 line-clamp-1">
              {migration.description}
            </p>
          </div>
          <span className="shrink-0 text-xs font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
            {migration.id}
          </span>
        </div>

        {/* Bottom row: stats */}
        <div className="flex items-center gap-3 mt-3">
          <Stat label={kindPlural} value={migration.candidates.length} />
          <StatDivider />
          <Stat label="steps" value={migration.steps.length} />
          <StatDivider />
          <Stat
            label={runCount === 1 ? "run" : "runs"}
            value={runCount}
            accent={hasRuns}
          />
          {doneCount > 0 && (
            <>
              <StatDivider />
              <Stat label="done" value={doneCount} accent />
            </>
          )}
          <div className="flex-1" />
          <span className="text-xs text-zinc-600 font-mono">
            {timeAgo(migration.createdAt)}
          </span>
          <svg
            width="14"
            height="14"
            viewBox="0 0 14 14"
            fill="none"
            className="text-zinc-600 group-hover:text-zinc-400 group-hover:translate-x-0.5 transition-all"
          >
            <path
              d="M5 3l4 4-4 4"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </div>
      </div>
    </Link>
  );
}

function Stat({ label, value, accent }: { label: string; value: number; accent?: boolean }) {
  return (
    <div className="flex items-center gap-1.5">
      <span
        className={`text-sm font-mono font-medium ${
          accent ? "text-teal-400" : "text-zinc-300"
        }`}
      >
        {value}
      </span>
      <span className="text-xs text-zinc-500">{label}</span>
    </div>
  );
}

function StatDivider() {
  return <div className="w-px h-3 bg-zinc-800" />;
}
