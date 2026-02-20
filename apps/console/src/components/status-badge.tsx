const config: Record<string, { bg: string; text: string; dot: string }> = {
  RUNNING: {
    bg: "bg-amber-500/8 border-amber-500/20",
    text: "text-amber-400",
    dot: "bg-amber-400 animate-pulse-subtle",
  },
  COMPLETED: {
    bg: "bg-emerald-500/8 border-emerald-500/20",
    text: "text-emerald-400",
    dot: "bg-emerald-400",
  },
  FAILED: {
    bg: "bg-red-500/8 border-red-500/20",
    text: "text-red-400",
    dot: "bg-red-400",
  },
  PENDING: {
    bg: "bg-zinc-500/8 border-zinc-500/20",
    text: "text-zinc-400",
    dot: "bg-zinc-500",
  },
};

export function StatusBadge({ status }: { status: string }) {
  const c = config[status] ?? config.PENDING;
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-mono font-medium border ${c.bg} ${c.text}`}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${c.dot}`} />
      {status}
    </span>
  );
}
