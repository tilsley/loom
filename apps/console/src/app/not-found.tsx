import Link from "next/link";
import { ROUTES } from "@/lib/routes";
import { buttonVariants } from "@/components/ui";

export default function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center py-24 px-4 text-center animate-fade-in-up">
      <div className="w-14 h-14 rounded-xl bg-zinc-800/50 border border-zinc-700/50 flex items-center justify-center mb-5">
        <span className="text-2xl font-mono font-bold text-zinc-500">404</span>
      </div>
      <h2 className="text-lg font-semibold text-zinc-100 mb-1">Page Not Found</h2>
      <p className="text-sm text-zinc-500 mb-6 max-w-sm">
        The page you&apos;re looking for doesn&apos;t exist or has been moved.
      </p>
      <Link href={ROUTES.dashboard} className={buttonVariants({ variant: "outline" })}>
        Return to Dashboard
      </Link>
    </div>
  );
}
