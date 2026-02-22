interface StatusFilterProps {
  counts: Record<string, number>;
  active: string;
  onChange: (filter: string) => void;
}

const filters = [
  { key: "all", label: "All", color: "text-zinc-300 bg-zinc-800/60 border-zinc-700" },
  { key: "running", label: "Running", color: "text-amber-400 bg-amber-500/10 border-amber-500/20" },
  { key: "queued", label: "Queued", color: "text-indigo-400 bg-indigo-500/10 border-indigo-500/20" },
  {
    key: "completed",
    label: "Completed",
    color: "text-emerald-400 bg-emerald-500/10 border-emerald-500/20",
  },
  { key: "not_started", label: "Not started", color: "text-slate-400 bg-slate-500/10 border-slate-500/20" },
];

export function StatusFilter({ counts, active, onChange }: StatusFilterProps) {
  const total = Object.values(counts).reduce((a, b) => a + b, 0);

  return (
    <div className="flex items-center gap-1.5">
      {filters.map((f) => {
        const count = f.key === "all" ? total : (counts[f.key] ?? 0);
        const isActive = active === f.key;

        return (
          <button
            key={f.key}
            onClick={() => onChange(f.key)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium border transition-all ${
              isActive
                ? f.color
                : "text-zinc-500 bg-transparent border-zinc-800/60 hover:border-zinc-700"
            }`}
          >
            {f.label}
            <span className={`text-xs font-mono ${isActive ? "opacity-80" : "text-zinc-600"}`}>
              {count}
            </span>
          </button>
        );
      })}
    </div>
  );
}
