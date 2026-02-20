"use client";

import { useMemo, useState } from "react";
import type { RegisteredMigration, Target } from "@/lib/api";
import { useRole } from "@/contexts/role-context";
import { StatusFilter } from "./status-filter";
import { TargetRow } from "./target-row";
import { Button, Input } from "@/components/ui";

const PAGE_SIZE = 50;

interface TargetTableProps {
  migration: RegisteredMigration;
  onRun: (target: Target) => Promise<void>;
  onRunAll: () => Promise<void>;
  runningTarget: string | null;
}

export function TargetTable({ migration, onRun, onRunAll, runningTarget }: TargetTableProps) {
  const { isAdmin } = useRole();
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState("all");
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);
  const [runningAll, setRunningAll] = useState(false);

  const counts = useMemo(() => {
    const c = { running: 0, completed: 0, failed: 0, pending: 0 };
    for (const t of migration.targets) {
      const run = migration.targetRuns?.[t.repo];
      if (!run) c.pending++;
      else if (run.status === "completed") c.completed++;
      else if (run.status === "running") c.running++;
      else if (run.status === "failed") c.failed++;
      else c.pending++;
    }
    return c;
  }, [migration.targets, migration.targetRuns]);

  const filtered = useMemo(() => {
    let targets = migration.targets;

    // Search
    if (search.trim()) {
      const q = search.toLowerCase();
      targets = targets.filter((t) => t.repo.toLowerCase().includes(q));
    }

    // Status filter
    if (filter !== "all") {
      targets = targets.filter((t) => {
        const run = migration.targetRuns?.[t.repo];
        if (filter === "pending") return !run;
        return run?.status === filter;
      });
    }

    return targets;
  }, [migration.targets, migration.targetRuns, search, filter]);

  const visible = filtered.slice(0, visibleCount);
  const hasMore = filtered.length > visibleCount;

  async function handleRunAll() {
    setRunningAll(true);
    try {
      await onRunAll();
    } finally {
      setRunningAll(false);
    }
  }

  return (
    <div className="space-y-3">
      {/* Search + actions bar */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1">
          <svg
            width="14"
            height="14"
            viewBox="0 0 14 14"
            fill="none"
            className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-600"
          >
            <circle cx="6" cy="6" r="4" stroke="currentColor" strokeWidth="1.5" />
            <path d="M9 9l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
          <Input
            type="text"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setVisibleCount(PAGE_SIZE);
            }}
            placeholder="Search targets..."
            className="pl-9 py-1.5 font-mono"
          />
        </div>
        {isAdmin && counts.pending > 0 ? (
          <Button
            size="sm"
            onClick={() => void handleRunAll()}
            disabled={runningAll || runningTarget !== null}
          >
            <svg width="10" height="10" viewBox="0 0 12 12" fill="none">
              <path d="M3 2l7 4-7 4V2z" fill="currentColor" />
            </svg>
            {runningAll ? "Running..." : `Run All Pending (${counts.pending})`}
          </Button>
        ) : null}
      </div>

      {/* Filters */}
      <StatusFilter
        counts={counts}
        active={filter}
        onChange={(f) => {
          setFilter(f);
          setVisibleCount(PAGE_SIZE);
        }}
      />

      {/* Table header */}
      <div className="grid grid-cols-[1fr_100px_1fr_80px] gap-2 px-3 text-[10px] text-zinc-600 uppercase tracking-widest font-medium">
        <span>Target</span>
        <span>Status</span>
        <span>Run</span>
        <span className="text-right">Actions</span>
      </div>

      {/* Rows */}
      <div className="space-y-1">
        {visible.map((t) => (
          <TargetRow
            key={t.repo}
            target={t}
            targetRun={migration.targetRuns?.[t.repo]}
            onRun={onRun}
            isRunning={runningTarget === t.repo}
          />
        ))}
      </div>

      {/* Empty state */}
      {visible.length === 0 && (
        <div className="text-center py-8 text-[13px] text-zinc-600">
          No targets match the current filter.
        </div>
      )}

      {/* Show more */}
      {hasMore ? (
        <Button
          variant="outline"
          size="sm"
          onClick={() => setVisibleCount((c) => c + PAGE_SIZE)}
          className="w-full"
        >
          Show more ({filtered.length - visibleCount} remaining)
        </Button>
      ) : null}
    </div>
  );
}
