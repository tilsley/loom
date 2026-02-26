"use client";

import type { FileDiff } from "@/lib/api";

type DiffLine = { type: "add" | "remove" | "context"; text: string };

export function computeDiff(before: string, after: string): DiffLine[] {
  if (!before) {
    return after ? after.split("\n").map((text) => ({ type: "add", text })) : [];
  }
  const a = before.split("\n");
  const b = after.split("\n");
  const m = a.length;
  const n = b.length;
  const dp = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = 1; i <= m; i++)
    for (let j = 1; j <= n; j++)
      dp[i][j] = a[i - 1] === b[j - 1] ? dp[i - 1][j - 1] + 1 : Math.max(dp[i - 1][j], dp[i][j - 1]);

  const result: DiffLine[] = [];
  let i = m,
    j = n;
  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && a[i - 1] === b[j - 1]) {
      result.push({ type: "context", text: a[i - 1] });
      i--;
      j--;
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      result.push({ type: "add", text: b[j - 1] });
      j--;
    } else {
      result.push({ type: "remove", text: a[i - 1] });
      i--;
    }
  }
  return result.reverse();
}

const CONTEXT_LINES = 2;

type EllipsisEntry = { type: "ellipsis"; count: number };

export function collapseContext(lines: DiffLine[]): Array<DiffLine | EllipsisEntry> {
  const show = new Array(lines.length).fill(false);
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].type !== "context") {
      for (let k = Math.max(0, i - CONTEXT_LINES); k <= Math.min(lines.length - 1, i + CONTEXT_LINES); k++)
        show[k] = true;
    }
  }
  const result: Array<DiffLine | EllipsisEntry> = [];
  let skip = 0;
  for (let i = 0; i < lines.length; i++) {
    if (show[i]) {
      if (skip > 0) {
        result.push({ type: "ellipsis", count: skip });
        skip = 0;
      }
      result.push(lines[i]);
    } else {
      skip++;
    }
  }
  if (skip > 0) result.push({ type: "ellipsis", count: skip });
  return result;
}

export function FileDiffView({ diff }: { diff: FileDiff }) {
  const lines = computeDiff(diff.before ?? "", diff.after);
  const collapsed = collapseContext(lines);

  return (
    <div className="border border-zinc-800/60 rounded-md overflow-hidden text-xs font-mono">
      <div className="flex items-center gap-2 px-3 py-1.5 bg-zinc-900/80 border-b border-zinc-800/60">
        <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor" className="text-zinc-500 shrink-0">
          <path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z" />
        </svg>
        <span className="text-zinc-300 flex-1 truncate">{diff.path}</span>
        <span className="text-zinc-600 shrink-0">{diff.repo}</span>
        {diff.status === "new" ? (
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            new
          </span>
        ) : diff.status === "deleted" ? (
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-red-500/10 text-red-400 border border-red-500/20">
            deleted
          </span>
        ) : null}
      </div>
      <div className="overflow-x-auto">
        {collapsed.map((line, idx) => {
          if (line.type === "ellipsis") {
            return (
              <div key={idx} className="px-3 py-0.5 text-zinc-600 bg-zinc-900/30 select-none">
                ··· {line.count} unchanged {line.count === 1 ? "line" : "lines"}
              </div>
            );
          }
          const bg = line.type === "add" ? "bg-emerald-500/10" : line.type === "remove" ? "bg-red-500/10" : "";
          const color = line.type === "add" ? "text-emerald-300" : line.type === "remove" ? "text-red-300" : "text-zinc-500";
          const prefix = line.type === "add" ? "+" : line.type === "remove" ? "-" : " ";
          return (
            <div key={idx} className={`flex ${bg} px-3 py-px`}>
              <span className={`w-4 shrink-0 select-none ${color} opacity-60`}>{prefix}</span>
              <span className={`${color} whitespace-pre`}>{line.text}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function DryRunStepResult({
  result,
}: {
  result: { stepName: string; skipped: boolean; error?: string; files?: FileDiff[] };
}) {
  if (result.skipped) {
    return <div className="text-xs text-zinc-600 italic">Skipped — handled by another worker</div>;
  }
  if (result.error) {
    return (
      <div className="text-xs font-mono text-red-400 bg-red-500/8 border border-red-500/20 rounded-md px-3 py-2">
        {result.error}
      </div>
    );
  }
  if (!result.files?.length) {
    return <div className="text-xs text-zinc-600 italic">No file changes</div>;
  }
  return (
    <div className="space-y-2">
      {result.files.map((f, i) => (
        <FileDiffView key={i} diff={f} />
      ))}
    </div>
  );
}
