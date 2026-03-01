import { forwardRef, type InputHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => (
    <input
      className={cn(
        "w-full bg-card/50 border border-border rounded-md px-3 py-2 text-sm placeholder:text-muted-foreground/50 focus:outline-none focus:border-border-hover focus:bg-card/80 focus-visible:ring-1 focus-visible:ring-ring transition-all",
        className,
      )}
      ref={ref}
      {...props}
    />
  ),
);
Input.displayName = "Input";

export { Input };
