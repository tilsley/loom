"use client";

import { createContext, useContext, type ReactNode } from "react";
import { useMigrationPolling } from "@/lib/hooks";
import type { Migration } from "@/lib/api";

interface MigrationsContextValue {
  migrations: Migration[];
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

const MigrationsContext = createContext<MigrationsContextValue | null>(null);

export function MigrationsProvider({ children }: { children: ReactNode }) {
  const value = useMigrationPolling(30_000);
  return <MigrationsContext.Provider value={value}>{children}</MigrationsContext.Provider>;
}

export function useMigrationsContext(): MigrationsContextValue {
  const ctx = useContext(MigrationsContext);
  if (!ctx) throw new Error("useMigrationsContext must be used within MigrationsProvider");
  return ctx;
}
