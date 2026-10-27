import { GridBackground } from "@/components/brand/Brand";
import { TopBar } from "@/components/layout/AuthLayout";

/** Placeholder sementara untuk layar yang belum diimplementasi dari Figma. */
export function PendingPage({ title, embedded }: { title: string; embedded?: boolean }) {
  const note = <p className="border border-ink bg-paper px-6 py-4 font-mono text-sm text-graphite">{title} — not implemented yet</p>;
  if (embedded) return <div className="flex flex-1 items-center justify-center">{note}</div>;
  return (
    <div className="flex min-h-dvh flex-col">
      <GridBackground />
      <TopBar />
      <main className="flex flex-1 items-center justify-center p-10">{note}</main>
    </div>
  );
}
