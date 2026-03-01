import { ToggleGroup, ToggleGroupItem } from "@/components/ui";

interface StatusFilterProps {
  counts: Record<string, number>;
  active: string;
  onChange: (filter: string) => void;
}

const filters = [
  { key: "all", label: "All", color: "text-foreground/80 bg-muted border-border-hover", inactive: "text-muted-foreground bg-transparent border-border/60 hover:border-border-hover" },
  { key: "running", label: "Running", color: "text-running bg-running/10 border-running/20", inactive: "text-muted-foreground bg-transparent border-border/60 hover:border-border-hover" },
  { key: "completed", label: "Completed", color: "text-completed bg-completed/10 border-completed/20", inactive: "text-muted-foreground bg-transparent border-border/60 hover:border-border-hover" },
  { key: "not_started", label: "Not started", color: "text-muted-foreground bg-muted-foreground/10 border-muted-foreground/20", inactive: "text-muted-foreground bg-transparent border-border/60 hover:border-border-hover" },
];

export function StatusFilter({ counts, active, onChange }: StatusFilterProps) {
  const total = Object.values(counts).reduce((a, b) => a + b, 0);

  return (
    <ToggleGroup
      type="single"
      value={active}
      onValueChange={(v) => { if (v) onChange(v); }}
    >
      {filters.map((f) => {
        const count = f.key === "all" ? total : (counts[f.key] ?? 0);
        const isActive = active === f.key;

        return (
          <ToggleGroupItem
            key={f.key}
            value={f.key}
            className={isActive ? f.color : f.inactive}
          >
            {f.label}
            <span className={`text-xs font-mono ${isActive ? "opacity-80" : "text-muted-foreground"}`}>
              {count}
            </span>
          </ToggleGroupItem>
        );
      })}
    </ToggleGroup>
  );
}
