"use client";

import { useEffect, useMemo, useState } from "react";
import type { Migration, Candidate } from "@/lib/api";
import { getCandidateCounts } from "@/lib/stats";
import { discoverMetadataColumns, filterCandidates } from "@/lib/filtering";
import { pluralizeKind } from "@/lib/formatting";
import { StatusFilter } from "./status-filter";
import { CandidateRow } from "./candidate-row";
import { Button, Input, Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui";

const PAGE_SIZE = 50;

interface CandidateTableProps {
  migration: Migration;
  onPreview: (candidate: Candidate) => void;
  onCancel?: (candidate: Candidate) => void;
  runningCandidate: string | null;
  filter: string;
  onFilterChange: (v: string) => void;
}

export function CandidateTable({
  migration,
  onPreview,
  onCancel,
  runningCandidate,
  filter,
  onFilterChange,
}: CandidateTableProps) {
  const kind = (migration.candidates ?? [])[0]?.kind ?? "candidate";
  const kindPlural = pluralizeKind(kind);
  const kindCap = kind.charAt(0).toUpperCase() + kind.slice(1);
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);
  const [columnFilters, setColumnFilters] = useState<Record<string, string>>({});

  useEffect(() => {
    setVisibleCount(PAGE_SIZE);
  }, [columnFilters, filter]);

  const counts = useMemo(
    () => getCandidateCounts(migration.candidates ?? []),
    [migration.candidates],
  );

  const metaColumns = useMemo(
    () => discoverMetadataColumns(migration.candidates ?? []),
    [migration.candidates],
  );

  function getMetaLabel(key: string) {
    return migration.requiredInputs?.find((i) => i.name === key)?.label ?? key;
  }

  function setColumnFilter(key: string, value: string) {
    setColumnFilters((prev) => ({ ...prev, [key]: value }));
  }

  function handleCellFilter(key: string, value: string) {
    setColumnFilters((prev) => ({
      ...prev,
      [key]: prev[key] === value ? "" : value,
    }));
  }

  const filtered = useMemo(
    () => filterCandidates(migration.candidates ?? [], columnFilters, metaColumns, filter),
    [migration.candidates, columnFilters, metaColumns, filter],
  );

  const visible = filtered.slice(0, visibleCount);
  const hasMore = filtered.length > visibleCount;
  const colCount = 1 + metaColumns.length + 1 + 1 + 1;
  const anyColumnFilter = Object.values(columnFilters).some((v) => v.trim());

  return (
    <div className="space-y-3">
      {/* Controls row: status filter + clear */}
      <div className="flex items-center gap-2 flex-wrap">
        <StatusFilter counts={counts} active={filter} onChange={onFilterChange} />
        {anyColumnFilter ? (
          <>
            <div className="w-px h-4 bg-border-hover/50 shrink-0 mx-1" />
            <button
              onClick={() => setColumnFilters({})}
              className="text-xs text-muted-foreground hover:text-foreground/80 transition-colors"
            >
              Clear filters
            </button>
          </>
        ) : null}
      </div>

      {/* Table */}
      <div className="rounded-lg border border-border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>{kindCap}</TableHead>
              {metaColumns.map((key) => (
                <TableHead key={key}>{getMetaLabel(key)}</TableHead>
              ))}
              <TableHead>Files</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
            <TableRow className="border-b border-border/60 bg-muted/50">
              <TableHead className="py-1.5">
                <Input
                  value={columnFilters.id ?? ""}
                  onChange={(e) => setColumnFilter("id", e.target.value)}
                  placeholder={`Search ${kindPlural}…`}
                  className="h-7 text-xs py-1 font-mono bg-transparent border-border-hover/50"
                />
              </TableHead>
              {metaColumns.map((key) => (
                <TableHead key={key} className="py-1.5">
                  <Input
                    value={columnFilters[key] ?? ""}
                    onChange={(e) => setColumnFilter(key, e.target.value)}
                    placeholder="Filter…"
                    className="h-7 text-xs py-1 font-mono bg-transparent border-border-hover/50"
                  />
                </TableHead>
              ))}
              <TableHead />
              <TableHead />
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {renderRows(visible)}
            {visible.length === 0 && (
              <TableRow>
                <TableCell colSpan={colCount} className="py-8 text-center text-muted-foreground">
                  No {kindPlural} match the current filter.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

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

  function renderRows(candidates: Candidate[]) {
    return candidates.map((c) => (
      <CandidateRow
        key={c.id}
        candidate={c}
        migrationId={
          c.status === "running" || c.status === "completed" ? migration.id : undefined
        }
        onPreview={onPreview}
        onCancel={onCancel}
        isRunning={runningCandidate === c.id}
        metaColumns={metaColumns}
        onMetaFilter={handleCellFilter}
        colCount={colCount}
      />
    ));
  }
}
