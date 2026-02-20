"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { listMigrations, type RegisteredMigration } from "./api";

interface UseMigrationsResult {
  migrations: RegisteredMigration[];
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

export function useMigrations(): UseMigrationsResult {
  const [migrations, setMigrations] = useState<RegisteredMigration[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refetch = useCallback(async () => {
    try {
      const res = await listMigrations();
      setMigrations(res.migrations);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refetch();
  }, [refetch]);

  return { migrations, loading, error, refetch };
}

export function useMigrationPolling(intervalMs = 5000): UseMigrationsResult {
  const [migrations, setMigrations] = useState<RegisteredMigration[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const refetch = useCallback(async () => {
    try {
      const res = await listMigrations();
      setMigrations(res.migrations);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refetch();
    intervalRef.current = setInterval(() => {
      void refetch();
    }, intervalMs);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [refetch, intervalMs]);

  return { migrations, loading, error, refetch };
}
