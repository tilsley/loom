"use client";

import { useMemo, useState } from "react";
import type { RegisteredMigration, Candidate } from "@/lib/api";
import { StatusFilter } from "./status-filter";
import { CandidateRow } from "./candidate-row";
import { Button, Input } from "@/components/ui";

const PAGE_SIZE = 50;

interface CandidateTableProps {
  migration: RegisteredMigration;
  onQueue: (candidate: Candidate) => Promise<void>;
  onDequeue: (runId: string) => Promise<void>;
  runningCandidate: string | null;
}

export function CandidateTable({ migration, onQueue, onDequeue, runningCandidate }: CandidateTableProps) {
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState("all");
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);
  const [groupBy, setGroupBy] = useState<string | null>(null);

  const counts = useMemo(() => {
    const c = { running: 0, completed: 0, not_started: 0, queued: 0 };
    for (const t of migration.candidates) {
      const run = migration.candidateRuns?.[t.id];
      if (!run) c.not_started++;
      else if (run.status === "completed") c.completed++;
      else if (run.status === "running") c.running++;
      else if (run.status === "queued") c.queued++;
      else c.not_started++;
    }
    return c;
  }, [migration.candidates, migration.candidateRuns]);

  // Derive groupable keys from metadata across all candidates
  const groupKeys = useMemo(() => {
    const keys = new Set<string>();
    for (const c of migration.candidates) {
      if (!c.metadata) continue;
      for (const k of Object.keys(c.metadata)) {
        keys.add(k);
      }
    }
    return Array.from(keys).sort();
  }, [migration.candidates]);

  const filtered = useMemo(() => {
    let candidates = migration.candidates;

    if (search.trim()) {
      const q = search.toLowerCase();
      candidates = candidates.filter((t) => t.id.toLowerCase().includes(q));
    }

    if (filter !== "all") {
      candidates = candidates.filter((t) => {
        const run = migration.candidateRuns?.[t.id];
        if (filter === "not_started") return !run;
        return run?.status === filter;
      });
    }

    return candidates;
  }, [migration.candidates, migration.candidateRuns, search, filter]);

  // Group filtered candidates by the active key, or null for flat list
  const groups = useMemo<[string, Candidate[]][] | null>(() => {
    if (!groupBy) return null;
    const map = new Map<string, Candidate[]>();
    for (const c of filtered) {
      const val = c.metadata?.[groupBy] ?? "—";
      if (!map.has(val)) map.set(val, []);
      map.get(val)?.push(c);
    }
    // Sort groups alphabetically, "—" last
    return Array.from(map.entries()).sort(([a], [b]) => {
      if (a === "—") return 1;
      if (b === "—") return -1;
      return a.localeCompare(b);
    });
  }, [filtered, groupBy]);

  const visible = filtered.slice(0, visibleCount);
  const hasMore = filtered.length > visibleCount;

  const columnHeader = (
    <div className="grid grid-cols-[1fr_100px_1fr_80px] gap-2 px-4 text-xs text-zinc-600 uppercase tracking-widest font-medium">
      <span>Candidate</span>
      <span>Status</span>
      <span>Run</span>
      <span className="text-right">Actions</span>
    </div>
  );

  return (
    <div className="space-y-3">
      {/* Search bar */}
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
            placeholder="Search candidates..."
            className="pl-9 py-1.5 font-mono"
          />
        </div>
      </div>

      {/* Controls row: status filter + group-by */}
      <div className="flex items-center gap-2 flex-wrap">
        <StatusFilter
          counts={counts}
          active={filter}
          onChange={(f) => {
            setFilter(f);
            setVisibleCount(PAGE_SIZE);
          }}
        />

        {groupKeys.length > 0 && (
          <>
            <div className="w-px h-4 bg-zinc-700/50 shrink-0 mx-1" />
            {groupKeys.map((key) => {
              const isActive = groupBy === key;
              return (
                <button
                  key={key}
                  onClick={() => setGroupBy(isActive ? null : key)}
                  className={`inline-flex items-center gap-1 text-xs font-mono px-2 py-1 rounded-full border transition-all ${
                    isActive
                      ? "bg-teal-500/10 text-teal-400 border-teal-500/30"
                      : "text-zinc-500 bg-transparent border-zinc-800/60 hover:border-zinc-700 hover:text-zinc-300"
                  }`}
                >
                  <svg width="10" height="10" viewBox="0 0 12 12" fill="none" className="opacity-70">
                    <rect x="1" y="1" width="4" height="4" rx="0.75" stroke="currentColor" strokeWidth="1.2" />
                    <rect x="7" y="1" width="4" height="4" rx="0.75" stroke="currentColor" strokeWidth="1.2" />
                    <rect x="1" y="7" width="4" height="4" rx="0.75" stroke="currentColor" strokeWidth="1.2" />
                    <rect x="7" y="7" width="4" height="4" rx="0.75" stroke="currentColor" strokeWidth="1.2" />
                  </svg>
                  {key}
                  {isActive ? <span className="opacity-50">×</span> : null}
                </button>
              );
            })}
          </>
        )}
      </div>

      {/* Grouped view */}
      {groups !== null ? (
        <div className="space-y-5">
          {groups.length === 0 ? (
            <div className="text-center py-8 text-sm text-zinc-600">
              No candidates match the current filter.
            </div>
          ) : (
            groups.map(([groupValue, candidates]) => (
              <div key={groupValue}>
                <div className="flex items-baseline gap-2 mb-2">
                  <span className="text-xs font-medium text-zinc-300">{groupValue}</span>
                  <span className="text-xs text-zinc-600">
                    {candidates.length} candidate{candidates.length !== 1 ? "s" : ""}
                  </span>
                </div>
                {columnHeader}
                <div className="space-y-1 mt-2">
                  {candidates.map((c) => (
                    <CandidateRow
                      key={c.id}
                      candidate={c}
                      candidateRun={migration.candidateRuns?.[c.id]}
                      runId={migration.candidateRuns?.[c.id] ? `${migration.id}__${c.id}` : undefined}
                      onQueue={onQueue}
                      onDequeue={onDequeue}
                      isRunning={runningCandidate === c.id || runningCandidate === `${migration.id}__${c.id}`}
                    />
                  ))}
                </div>
              </div>
            ))
          )}
        </div>
      ) : (
        <>
          {/* Flat view */}
          {columnHeader}

          <div className="space-y-1">
            {visible.map((t) => (
              <CandidateRow
                key={t.id}
                candidate={t}
                candidateRun={migration.candidateRuns?.[t.id]}
                runId={migration.candidateRuns?.[t.id] ? `${migration.id}__${t.id}` : undefined}
                onQueue={onQueue}
                onDequeue={onDequeue}
                isRunning={runningCandidate === t.id || runningCandidate === `${migration.id}__${t.id}`}
              />
            ))}
          </div>

          {visible.length === 0 && (
            <div className="text-center py-8 text-sm text-zinc-600">
              No candidates match the current filter.
            </div>
          )}

          {hasMore ? <Button
              variant="outline"
              size="sm"
              onClick={() => setVisibleCount((c) => c + PAGE_SIZE)}
              className="w-full"
            >
              Show more ({filtered.length - visibleCount} remaining)
            </Button> : null}
        </>
      )}
    </div>
  );
}
