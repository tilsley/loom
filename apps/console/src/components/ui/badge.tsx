import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva("inline-flex items-center text-xs font-medium px-2 py-0.5 rounded border", {
  variants: {
    variant: {
      default: "bg-muted text-muted-foreground border-border",
      running: "bg-running/10 text-running border-running/20",
      completed: "bg-completed/10 text-completed border-completed/20",
    },
  },
  defaultVariants: {
    variant: "default",
  },
});

interface BadgeProps extends VariantProps<typeof badgeVariants> {
  children: React.ReactNode;
  className?: string;
}

export function Badge({ variant, className, children }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)}>{children}</span>;
}
