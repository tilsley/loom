"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useMigrations } from "@/lib/hooks";
import { cn } from "@/lib/utils";
import { ROUTES } from "@/lib/routes";
import { useTheme } from "@/contexts/theme-context";
import { RoleIndicator } from "./role-indicator";

const MAX_VISIBLE = 15;

function ThemeToggle() {
  const { theme, toggle } = useTheme();
  const isDark = theme === "dark";

  return (
    <button
      onClick={toggle}
      className={cn(
        "w-full flex items-center gap-2.5 px-2.5 py-2 rounded-md text-[13px] font-medium transition-colors",
        "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40",
      )}
      aria-label={isDark ? "Switch to light mode" : "Switch to dark mode"}
    >
      {isDark ? <SunIcon /> : <MoonIcon />}
      {isDark ? "Light mode" : "Dark mode"}
    </button>
  );
}

function SunIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" className="text-zinc-500 shrink-0">
      <circle cx="8" cy="8" r="3" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3.05 3.05l1.06 1.06M11.89 11.89l1.06 1.06M12.95 3.05l-1.06 1.06M4.11 11.89l-1.06 1.06"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" className="text-zinc-500 shrink-0">
      <path
        d="M13.5 9.5A6 6 0 0 1 6.5 2.5a6 6 0 1 0 7 7Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function Sidebar() {
  const pathname = usePathname();
  const { migrations } = useMigrations();

  const navItems = [
    { href: ROUTES.dashboard, label: "Dashboard", icon: DashboardIcon },
    { href: ROUTES.migrations, label: "Migrations", icon: MigrationsIcon },
  ];

  const visibleMigrations = migrations.slice(0, MAX_VISIBLE);
  const hiddenCount = migrations.length - MAX_VISIBLE;

  return (
    <aside className="fixed left-0 top-0 bottom-0 w-60 bg-zinc-950 border-r border-zinc-800/80 flex flex-col z-40">
      {/* Branding */}
      <div className="px-5 py-4">
        <Link href={ROUTES.dashboard} className="flex items-center gap-3 group">
          <div className="w-7 h-7 rounded-md bg-gradient-to-br from-teal-400 to-teal-600 flex items-center justify-center">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" className="text-zinc-950">
              <path
                d="M2 4h10M2 7h7M2 10h10"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
              />
            </svg>
          </div>
          <span className="text-[15px] font-semibold tracking-tight text-zinc-100 group-hover:text-white transition-colors">
            Loom
          </span>
          <span className="text-[10px] font-mono font-medium text-teal-400/70 bg-teal-400/8 px-1.5 py-0.5 rounded tracking-wide uppercase">
            console
          </span>
        </Link>
      </div>

      {/* Nav */}
      <nav className="flex-1 px-3 py-2 overflow-y-auto">
        <ul className="space-y-0.5">
          {navItems.map((item) => {
            const isActive =
              item.href === ROUTES.dashboard
                ? pathname === ROUTES.dashboard
                : pathname.startsWith(item.href);

            return (
              <li key={item.href}>
                <Link
                  href={item.href}
                  className={cn(
                    "flex items-center gap-2.5 px-2.5 py-2 rounded-md text-[13px] font-medium transition-colors",
                    isActive
                      ? "bg-zinc-800/60 text-zinc-100"
                      : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40",
                  )}
                >
                  <item.icon active={isActive} />
                  {item.label}
                </Link>

                {/* Migration sub-list */}
                {item.href === ROUTES.migrations && migrations.length > 0 && (
                  <ul className="mt-1 ml-4 space-y-0.5">
                    {visibleMigrations.map((m) => {
                      const mActive = pathname === ROUTES.migrationDetail(m.id);
                      return (
                        <li key={m.id}>
                          <Link
                            href={ROUTES.migrationDetail(m.id)}
                            className={cn(
                              "block px-2.5 py-1.5 rounded-md text-[12px] truncate transition-colors",
                              mActive
                                ? "text-teal-400 bg-teal-500/8"
                                : "text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/30",
                            )}
                            title={m.name}
                          >
                            {m.name}
                          </Link>
                        </li>
                      );
                    })}
                    {hiddenCount > 0 && (
                      <li className="px-2.5 py-1 text-[11px] text-zinc-600">+{hiddenCount} more</li>
                    )}
                  </ul>
                )}
              </li>
            );
          })}
        </ul>
      </nav>

      {/* Footer */}
      <div className="px-3 py-3 border-t border-zinc-800/60 space-y-2">
        <ThemeToggle />
        <RoleIndicator />
      </div>
    </aside>
  );
}

function DashboardIcon({ active }: { active: boolean }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      className={active ? "text-zinc-200" : "text-zinc-500"}
    >
      <rect x="2" y="2" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <rect x="9" y="2" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <rect x="2" y="9" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <rect x="9" y="9" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  );
}

function MigrationsIcon({ active }: { active: boolean }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      className={active ? "text-zinc-200" : "text-zinc-500"}
    >
      <path
        d="M3 5h10M3 8h7M3 11h10"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}
