"use client";

import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useRouter } from "next/navigation";
import { Command } from "cmdk";
import { useMigrations } from "@/lib/hooks";
import { ROUTES } from "@/lib/routes";

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const router = useRouter();
  const { migrations } = useMigrations();
  const openRef = useRef(false);

  useEffect(() => {
    openRef.current = open;
  }, [open]);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setOpen((prev) => !prev);
      } else if (e.key === "Escape" && openRef.current) {
        setOpen(false);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  function navigate(href: string) {
    router.push(href);
    setOpen(false);
  }

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-[60]" aria-label="Command palette">
      <div
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={() => setOpen(false)}
      />
      <div className="absolute left-1/2 top-[18vh] -translate-x-1/2 w-full max-w-[560px] px-4">
        <Command
          className="bg-zinc-900 border border-zinc-700/80 rounded-xl shadow-2xl overflow-hidden"
          onKeyDown={(e) => {
            if (e.key === "Escape") setOpen(false);
          }}
        >
          <div className="flex items-center gap-2.5 px-4 border-b border-zinc-800">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" className="text-zinc-500 shrink-0">
              <circle cx="6" cy="6" r="4" stroke="currentColor" strokeWidth="1.5" />
              <path d="M9 9l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
            <Command.Input
              placeholder="Go to migration, search…"
              className="flex-1 bg-transparent py-3.5 text-sm text-zinc-100 placeholder:text-zinc-600 outline-none"
            />
            <kbd className="text-[11px] text-zinc-600 bg-zinc-800 px-1.5 py-0.5 rounded font-mono shrink-0">
              esc
            </kbd>
          </div>

          <Command.List className="max-h-72 overflow-y-auto py-1.5">
            <Command.Empty className="py-8 text-center text-sm text-zinc-600">
              No results found
            </Command.Empty>

            <Command.Group
              heading="Navigate"
              className="[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-[10px] [&_[cmdk-group-heading]]:font-semibold [&_[cmdk-group-heading]]:text-zinc-600 [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-widest"
            >
              <PaletteItem
                icon={<DashboardIcon />}
                hint="Home"
                onSelect={() => navigate(ROUTES.dashboard)}
              >
                Dashboard
              </PaletteItem>
              <PaletteItem
                icon={<MigrationsIcon />}
                hint="List"
                onSelect={() => navigate(ROUTES.migrations)}
              >
                All Migrations
              </PaletteItem>
            </Command.Group>

            {migrations.length > 0 ? (
              <Command.Group
                heading="Migrations"
                className="[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-[10px] [&_[cmdk-group-heading]]:font-semibold [&_[cmdk-group-heading]]:text-zinc-600 [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-widest"
              >
                {migrations.map((m) => (
                  <PaletteItem
                    key={m.id}
                    icon={<MigrationItemIcon />}
                    hint={`${m.candidates?.length ?? 0} targets`}
                    onSelect={() => navigate(ROUTES.migrationDetail(m.id))}
                  >
                    {m.name}
                  </PaletteItem>
                ))}
              </Command.Group>
            ) : null}
          </Command.List>

          <div className="flex items-center gap-4 px-3 py-2 border-t border-zinc-800/80 text-[11px] text-zinc-600">
            <span className="flex items-center gap-1">
              <kbd className="bg-zinc-800 px-1 py-0.5 rounded font-mono">↑</kbd>
              <kbd className="bg-zinc-800 px-1 py-0.5 rounded font-mono">↓</kbd>
              navigate
            </span>
            <span className="flex items-center gap-1">
              <kbd className="bg-zinc-800 px-1 py-0.5 rounded font-mono">↵</kbd>
              select
            </span>
            <span className="flex items-center gap-1">
              <kbd className="bg-zinc-800 px-1.5 py-0.5 rounded font-mono">⌘K</kbd>
              toggle
            </span>
          </div>
        </Command>
      </div>
    </div>,
    document.body,
  );
}

function PaletteItem({
  children,
  icon,
  hint,
  onSelect,
}: {
  children: React.ReactNode;
  icon?: React.ReactNode;
  hint?: string;
  onSelect: () => void;
}) {
  return (
    <Command.Item
      onSelect={onSelect}
      className="flex items-center gap-2.5 px-3 py-2 mx-1.5 rounded-lg text-sm text-zinc-300 cursor-pointer data-[selected=true]:bg-zinc-800 data-[selected=true]:text-zinc-100 transition-colors"
    >
      {icon ? <span className="text-zinc-500 shrink-0">{icon}</span> : null}
      <span className="flex-1 min-w-0 truncate">{children}</span>
      {hint ? <span className="text-xs text-zinc-600 shrink-0">{hint}</span> : null}
    </Command.Item>
  );
}

function DashboardIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
      <rect x="2" y="2" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <rect x="9" y="2" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <rect x="2" y="9" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <rect x="9" y="9" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  );
}

function MigrationsIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
      <path d="M3 5h10M3 8h7M3 11h10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function MigrationItemIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
      <path d="M8 2l6 4v4l-6 4L2 10V6l6-4z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
    </svg>
  );
}
