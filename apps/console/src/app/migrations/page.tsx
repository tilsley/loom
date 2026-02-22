"use client";

import Link from "next/link";
import { useMigrations } from "@/lib/hooks";
import { useRole } from "@/contexts/role-context";
import { ROUTES } from "@/lib/routes";
import { MigrationCard } from "@/components/migration-card";
import { buttonVariants, Skeleton } from "@/components/ui";

export default function MigrationsPage() {
  const { migrations, loading, error } = useMigrations();
  const { isAdmin } = useRole();

  return (
    <div className="space-y-8 animate-fade-in-up">
      {/* Section header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h2 className="text-xs font-medium text-zinc-500 uppercase tracking-widest">
            Registered Migrations
          </h2>
          {!loading && migrations.length > 0 && (
            <span className="text-xs font-mono text-zinc-600 bg-zinc-800/60 px-1.5 py-0.5 rounded">
              {migrations.length}
            </span>
          )}
        </div>
        {isAdmin ? (
          <Link
            href={ROUTES.newMigration}
            className={buttonVariants({ size: "sm", className: "gap-1.5" })}
          >
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
              <path
                d="M6 2v8M2 6h8"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
              />
            </svg>
            New
          </Link>
        ) : null}
      </div>

      {/* Error */}
      {Boolean(error) && (
        <div className="bg-red-500/8 border border-red-500/20 rounded-lg px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      {/* Migration list */}
      {loading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-[76px]" style={{ animationDelay: `${i * 150}ms` }} />
          ))}
        </div>
      ) : migrations.length === 0 ? (
        <div className="border border-dashed border-zinc-800 rounded-lg py-12 text-center">
          <div className="w-10 h-10 rounded-lg bg-zinc-900 flex items-center justify-center mx-auto mb-4">
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none" className="text-zinc-600">
              <path
                d="M3 6h12M3 9h9M3 12h12"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
              />
            </svg>
          </div>
          <p className="text-sm text-zinc-500">No migrations registered yet</p>
          <p className="text-xs text-zinc-600 mt-1">
            Workers can announce migrations via pub/sub, or{" "}
            <Link
              href={ROUTES.newMigration}
              className="text-teal-500 hover:text-teal-400 transition-colors"
            >
              register one manually
            </Link>
          </p>
        </div>
      ) : (
        <div className="grid gap-2 stagger-children">
          {migrations.map((m) => (
            <MigrationCard key={m.id} migration={m} />
          ))}
        </div>
      )}
    </div>
  );
}
