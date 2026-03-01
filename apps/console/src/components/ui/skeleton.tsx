import { cn } from "@/lib/utils";

function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("bg-muted/50 rounded-lg animate-pulse-subtle", className)} {...props} />
  );
}

export { Skeleton };
