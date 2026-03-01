"use client";

import { Toaster as SonnerToaster } from "sonner";
import { useTheme } from "@/contexts/theme-context";

export function Toaster() {
  const { theme } = useTheme();
  return (
    <SonnerToaster
      position="top-right"
      theme={theme}
      toastOptions={{
        style: {
          background: "var(--color-card)",
          border: "1px solid var(--color-border)",
          color: "var(--color-card-foreground)",
          fontSize: "13px",
        },
      }}
    />
  );
}
