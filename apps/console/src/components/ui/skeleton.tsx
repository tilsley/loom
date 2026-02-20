import { cn } from "@/lib/utils";

function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("bg-zinc-800/50 rounded-lg animate-pulse-subtle", className)} {...props} />
  );
}

function SkeletonCard({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <Skeleton className={cn("h-[76px]", className)} {...props} />;
}

function SkeletonText({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <Skeleton className={cn("h-4 w-48", className)} {...props} />;
}

export { Skeleton, SkeletonCard, SkeletonText };
