import type { Candidate } from "@/lib/api";

export function discoverMetadataColumns(candidates: Candidate[]): string[] {
  const keys = new Set<string>();
  for (const c of candidates) {
    if (!c.metadata) continue;
    for (const k of Object.keys(c.metadata)) keys.add(k);
  }
  return Array.from(keys).sort();
}

export function filterCandidates(
  candidates: Candidate[],
  columnFilters: Record<string, string>,
  metaColumns: string[],
  statusFilter: string,
): Candidate[] {
  let result = candidates;

  if (columnFilters.id?.trim()) {
    const q = columnFilters.id.toLowerCase();
    result = result.filter((c) => c.id.toLowerCase().includes(q));
  }

  for (const key of metaColumns) {
    const val = columnFilters[key]?.trim();
    if (!val) continue;
    result = result.filter((c) => c.metadata?.[key]?.toLowerCase().includes(val.toLowerCase()));
  }

  if (statusFilter !== "all") {
    result = result.filter((t) => {
      if (statusFilter === "not_started") return t.status === "not_started";
      return t.status === statusFilter;
    });
  }

  return result;
}
