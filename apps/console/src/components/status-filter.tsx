interface StatusFilterProps {
  counts: Record<string, number>;
  active: string;
  onChange: (filter: string) => void;
}

const filters = [
  { key: "all", label: "All", color: "text-zinc-300 bg-zinc-800/60 border-zinc-700" },
  { key: "running", label: "Running", color: "text-amber-400 bg-amber-500/10 border-amber-500/20" },
  {
    key: "completed",
    label: "Completed",
    color: "text-emerald-400 bg-emerald-500/10 border-emerald-500/20",
  },
  { key: "failed", label: "Failed", color: "text-red-400 bg-red-500/10 border-red-500/20" },
  { key: "not_started", label: "Not started", color: "text-zinc-400 bg-zinc-700/30 border-zinc-600/30" },
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
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-medium border transition-all ${
              isActive
                ? f.color
                : "text-zinc-500 bg-transparent border-zinc-800/60 hover:border-zinc-700"
            }`}
          >
            {f.label}
            <span className={`text-[10px] font-mono ${isActive ? "opacity-80" : "text-zinc-600"}`}>
              {count}
            </span>
          </button>
        );
      })}
    </div>
  );
}
