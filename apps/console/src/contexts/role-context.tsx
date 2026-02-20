"use client";

import { createContext, useContext, useEffect, useState } from "react";

export type Role = "admin" | "operator";

interface RoleContextValue {
  role: Role;
  setRole: (role: Role) => void;
  isAdmin: boolean;
}

const RoleContext = createContext<RoleContextValue | null>(null);

const STORAGE_KEY = "loom-role";

export function RoleProvider({ children }: { children: React.ReactNode }) {
  const [role, setRole] = useState<Role>("operator");
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "admin" || stored === "operator") {
      setRole(stored);
    }
    setHydrated(true);
  }, []);

  function handleSetRole(r: Role) {
    setRole(r);
    localStorage.setItem(STORAGE_KEY, r);
  }

  if (!hydrated) return null;

  return (
    <RoleContext.Provider value={{ role, setRole: handleSetRole, isAdmin: role === "admin" }}>
      {children}
    </RoleContext.Provider>
  );
}

export function useRole(): RoleContextValue {
  const ctx = useContext(RoleContext);
  if (!ctx) throw new Error("useRole must be used within RoleProvider");
  return ctx;
}
