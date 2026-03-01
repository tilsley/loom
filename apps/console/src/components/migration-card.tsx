import Link from "next/link";
import type { Migration } from "@/lib/api";
import { getMigrationRunStats } from "@/lib/stats";
import { timeAgo, pluralizeKind } from "@/lib/formatting";

export function MigrationCard({ migration }: { migration: Migration }) {
  const candidates = migration.candidates ?? [];
  const kindPlural = pluralizeKind(candidates[0]?.kind);
  const { runCount, doneCount } = getMigrationRunStats(candidates);
  const hasRuns = runCount > 0;

  return (
    <Link
      href={`/migrations/${migration.id}`}
      className="group relative block rounded-lg border border-border bg-[var(--color-surface)] hover:border-border-hover hover:bg-[var(--color-surface-raised)] transition-all duration-200"
    >
      {/* Left accent */}
      <div className="absolute left-0 top-3 bottom-3 w-[3px] rounded-full bg-primary/40 group-hover:bg-primary/70 transition-colors" />

      <div className="pl-5 pr-4 py-3.5">
        {/* Top row: name + slug */}
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h3 className="text-[14px] font-semibold text-foreground truncate transition-colors">
              {migration.name}
            </h3>
            <p className="text-xs text-muted-foreground mt-0.5 line-clamp-1">
              {migration.description}
            </p>
          </div>
          <span className="shrink-0 text-xs font-mono text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
            {migration.id}
          </span>
        </div>

        {/* Bottom row: stats */}
        <div className="flex items-center gap-3 mt-3">
          <Stat label={kindPlural} value={candidates.length} />
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
          <span className="text-xs text-muted-foreground font-mono">
            {timeAgo(migration.createdAt)}
          </span>
          <svg
            width="14"
            height="14"
            viewBox="0 0 14 14"
            fill="none"
            className="text-muted-foreground/70 group-hover:text-muted-foreground group-hover:translate-x-0.5 transition-all"
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
          accent ? "text-primary" : "text-foreground/80"
        }`}
      >
        {value}
      </span>
      <span className="text-xs text-muted-foreground">{label}</span>
    </div>
  );
}

function StatDivider() {
  return <div className="w-px h-3 bg-border" />;
}
