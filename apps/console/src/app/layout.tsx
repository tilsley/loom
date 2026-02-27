import type { Metadata } from "next";
import { Instrument_Sans, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { ThemeProvider } from "@/contexts/theme-context";
import { Sidebar } from "@/components/sidebar";
import { CommandPalette } from "@/components/command-palette";
import { Toaster, ErrorBoundary, TooltipProvider } from "@/components/ui";

const sans = Instrument_Sans({
  subsets: ["latin"],
  variable: "--font-instrument",
});

const mono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains",
  weight: ["400", "500"],
});

export const metadata: Metadata = {
  title: "Loom Console",
  description: "Migration orchestration dashboard",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body
        className={`${sans.variable} ${mono.variable} font-sans bg-zinc-950 text-zinc-100 min-h-screen antialiased`}
      >
        <TooltipProvider>
        <ThemeProvider>
          <div className="flex min-h-screen">
            <Sidebar />
            <main className="ml-60 w-[calc(100vw-15rem)] min-w-0 px-8 py-6">
              <ErrorBoundary>{children}</ErrorBoundary>
            </main>
          </div>
          <CommandPalette />
        </ThemeProvider>
        </TooltipProvider>
        <Toaster />
      </body>
    </html>
  );
}
