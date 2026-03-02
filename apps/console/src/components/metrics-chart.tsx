"use client";

import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { TimelinePoint } from "@/lib/api";

interface MetricsChartProps {
  data: TimelinePoint[];
}

export function MetricsChart({ data }: MetricsChartProps) {
  if (data.length === 0) {
    return (
      <div className="h-64 flex items-center justify-center text-sm text-muted-foreground">
        No timeline data yet
      </div>
    );
  }

  const formatted = data.map((d) => ({
    ...d,
    date: d.date.slice(5), // MM-DD
  }));

  return (
    <ResponsiveContainer width="100%" height={264}>
      <AreaChart data={formatted} margin={{ top: 8, right: 8, left: -16, bottom: 0 }}>
        <defs>
          <linearGradient id="grad-started" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="var(--color-pending)" stopOpacity={0.3} />
            <stop offset="95%" stopColor="var(--color-pending)" stopOpacity={0} />
          </linearGradient>
          <linearGradient id="grad-completed" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="var(--color-completed)" stopOpacity={0.3} />
            <stop offset="95%" stopColor="var(--color-completed)" stopOpacity={0} />
          </linearGradient>
          <linearGradient id="grad-failed" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="var(--color-destructive)" stopOpacity={0.3} />
            <stop offset="95%" stopColor="var(--color-destructive)" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
        <XAxis
          dataKey="date"
          tick={{ fontSize: 11, fill: "var(--color-muted-foreground)" }}
          tickLine={false}
          axisLine={false}
        />
        <YAxis
          tick={{ fontSize: 11, fill: "var(--color-muted-foreground)" }}
          tickLine={false}
          axisLine={false}
          allowDecimals={false}
        />
        <Tooltip
          contentStyle={{
            background: "var(--color-card)",
            border: "1px solid var(--color-border)",
            borderRadius: "6px",
            fontSize: "12px",
            padding: "8px 12px",
            boxShadow: "0 2px 8px rgba(0,0,0,0.15)",
            color: "var(--color-foreground)",
          }}
        />
        <Area
          type="monotone"
          dataKey="started"
          stroke="var(--color-pending)"
          fill="url(#grad-started)"
          strokeWidth={1.5}
        />
        <Area
          type="monotone"
          dataKey="completed"
          stroke="var(--color-completed)"
          fill="url(#grad-completed)"
          strokeWidth={1.5}
        />
        <Area
          type="monotone"
          dataKey="failed"
          stroke="var(--color-destructive)"
          fill="url(#grad-failed)"
          strokeWidth={1.5}
        />
      </AreaChart>
    </ResponsiveContainer>
  );
}
