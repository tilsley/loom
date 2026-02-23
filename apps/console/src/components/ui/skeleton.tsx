import { cn } from "@/lib/utils";

function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("bg-zinc-800/50 rounded-lg animate-pulse-subtle", className)} {...props} />
  );
}

export { Skeleton };
