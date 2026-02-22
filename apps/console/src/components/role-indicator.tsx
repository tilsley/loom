"use client";

import { useRole } from "@/contexts/role-context";
import { cn } from "@/lib/utils";

export function RoleIndicator() {
  const { role, setRole } = useRole();

  function toggle() {
    setRole(role === "admin" ? "operator" : "admin");
  }

  const isAdmin = role === "admin";

  return (
    <button
      onClick={toggle}
      className="group flex items-center gap-2 w-full px-3 py-2 rounded-md hover:bg-zinc-800/50 transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
    >
      <span
        className={cn("w-1.5 h-1.5 rounded-full shrink-0", isAdmin ? "bg-teal-400" : "bg-zinc-500")}
      />
      <span className={cn("text-xs font-medium", isAdmin ? "text-teal-400" : "text-zinc-400")}>
        {role}
      </span>
      <span className="text-xs text-zinc-600 opacity-0 group-hover:opacity-100 transition-opacity ml-auto">
        switch
      </span>
    </button>
  );
}
