"use client";

import { useRole, type Role } from "@/contexts/role-context";

interface RoleGateProps {
  require: Role;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

export function RoleGate({ require, children, fallback }: RoleGateProps) {
  const { role } = useRole();

  if (role !== require) {
    return fallback ?? null;
  }

  return children;
}
