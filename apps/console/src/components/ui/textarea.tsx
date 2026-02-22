import { forwardRef, type TextareaHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className, ...props }, ref) => (
    <textarea
      className={cn(
        "w-full bg-zinc-900/50 border border-zinc-800 rounded-md px-3 py-2 text-sm placeholder:text-zinc-700 focus:outline-none focus:border-zinc-600 focus:bg-zinc-900/80 focus-visible:ring-1 focus-visible:ring-ring transition-all",
        className,
      )}
      ref={ref}
      {...props}
    />
  ),
);
Textarea.displayName = "Textarea";

export { Textarea };
