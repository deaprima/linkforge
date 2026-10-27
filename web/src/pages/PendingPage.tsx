import { GridBackground } from "@/components/brand/Brand";
import { TopBar } from "@/components/layout/AuthLayout";

/** Placeholder sementara untuk layar yang belum diimplementasi dari Figma. */
export function PendingPage({ title }: { title: string }) {
  return (
    <div className="flex min-h-dvh flex-col">
      <GridBackground />
      <TopBar />
      <main className="flex flex-1 items-center justify-center p-10">
        <p className="border border-ink bg-paper px-6 py-4 font-mono text-sm text-graphite">{title} — not implemented yet</p>
      </main>
    </div>
  );
}
