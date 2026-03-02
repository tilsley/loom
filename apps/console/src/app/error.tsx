"use client";

import { useEffect } from "react";
import Link from "next/link";
import { ROUTES } from "@/lib/routes";
import { Button, buttonVariants } from "@/components/ui";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Route error:", error);
  }, [error]);

  return (
    <div className="flex flex-col items-center justify-center py-24 px-4 text-center animate-fade-in-up">
      <div className="w-14 h-14 rounded-xl bg-destructive/10 border border-destructive/20 flex items-center justify-center mb-5">
        <svg width="28" height="28" viewBox="0 0 24 24" fill="none" className="text-destructive">
          <path
            d="M12 9v4m0 4h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </div>
      <h2 className="text-lg font-semibold text-foreground mb-1">Something went wrong</h2>
      <p className="text-sm text-muted-foreground mb-6 max-w-sm">
        An unexpected error occurred. You can try again or return to migrations.
      </p>
      {process.env.NODE_ENV === "development" && (
        <pre className="text-xs font-mono text-destructive/70 bg-destructive/5 border border-destructive/10 rounded-lg px-4 py-3 mb-6 max-w-lg overflow-auto text-left">
          {error.message}
        </pre>
      )}
      <div className="flex gap-3">
        <Button onClick={reset}>Try Again</Button>
        <Link href={ROUTES.migrations} className={buttonVariants({ variant: "outline" })}>
          Return to Migrations
        </Link>
      </div>
    </div>
  );
}
