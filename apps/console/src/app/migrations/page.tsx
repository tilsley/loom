"use client";

import { useMigrations } from "@/lib/hooks";
import { MigrationCard } from "@/components/migration-card";
import { Skeleton } from "@/components/ui";

export default function MigrationsPage() {
  const { migrations, loading, error } = useMigrations();

  return (
    <div className="space-y-8 animate-fade-in-up">
      {/* Section header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-widest">
            Registered Migrations
          </h2>
          {!loading && migrations.length > 0 && (
            <span className="text-xs font-mono text-muted-foreground/70 bg-muted px-1.5 py-0.5 rounded">
              {migrations.length}
            </span>
          )}
        </div>
      </div>

      {/* Error */}
      {Boolean(error) && (
        <div className="bg-destructive/8 border border-destructive/20 rounded-lg px-4 py-3 text-sm text-destructive">
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
        <div className="border border-dashed border-border rounded-lg py-12 px-8 text-center space-y-6">
          <div>
            <div className="w-10 h-10 rounded-lg bg-card flex items-center justify-center mx-auto mb-4">
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none" className="text-muted-foreground/70">
                <path
                  d="M3 6h12M3 9h9M3 12h12"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                />
              </svg>
            </div>
            <p className="text-sm text-muted-foreground font-medium">No migrations registered yet</p>
            <p className="text-xs text-muted-foreground/70 mt-1 max-w-sm mx-auto">
              Migrators announce themselves on startup by posting to the registry. Start a migrator to register its migrations.
            </p>
          </div>
          <div className="max-w-md mx-auto text-left">
            <p className="text-[11px] font-medium text-muted-foreground/70 uppercase tracking-widest mb-2">
              Or register manually
            </p>
            <div className="bg-card border border-border rounded-lg overflow-hidden">
              <div className="flex items-center gap-2 px-3 py-2 border-b border-border/60">
                <span className="text-[10px] font-mono text-muted-foreground/70 uppercase tracking-widest">shell</span>
              </div>
              <pre className="text-xs font-mono text-muted-foreground px-4 py-3 overflow-x-auto leading-relaxed whitespace-pre">{`curl -X POST http://localhost:8080/registry/announce \\
  -H 'Content-Type: application/json' \\
  -d '{
    "id": "my-migration",
    "name": "My Migration",
    "migratorUrl": "http://localhost:9090",
    "steps": [{ "name": "step-1", "migratorApp": "my-app" }]
  }'`}</pre>
            </div>
          </div>
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
