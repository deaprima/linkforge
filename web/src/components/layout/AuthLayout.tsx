import type { ReactNode } from "react";
import { Link } from "react-router";
import { GridBackground, Logo, ThemeToggle } from "@/components/brand/Brand";
import { Kliko, type KlikoPose } from "@/components/brand/Kliko";

/** Layout Sign in / Create account (Figma 19:465, 19:555): top bar + panel 440px dengan Kliko di tengah atasnya. */
export function TopBar({ children }: { children?: ReactNode }) {
  return (
    <header className="flex h-16 shrink-0 items-center justify-between border-b border-ink bg-paper px-4 sm:px-10">
      <Link to="/" aria-label="LinkForge home" className="focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-signal">
        <Logo />
      </Link>
      <div className="flex items-center gap-3">
        <ThemeToggle />
        {children}
      </div>
    </header>
  );
}

export function AuthLayout({ pose, title, subtitle, footer, children }: { pose: KlikoPose; title: string; subtitle: string; footer: ReactNode; children: ReactNode }) {
  return (
    <div className="flex min-h-dvh flex-col">
      <GridBackground />
      <TopBar />
      <main className="flex flex-1 flex-col items-center justify-center px-4 py-10">
        <div className="flex w-full max-w-[440px] flex-col">
          <Kliko pose={pose} className="mx-auto w-[110px]" />
          <section className="flex w-full flex-col border border-ink bg-paper">
            <div className="flex flex-col gap-1.5 border-b border-ink px-7 pt-6 pb-5">
              <h1 className="text-[26px] font-semibold tracking-[-0.52px] text-ink">{title}</h1>
              <p className="text-sm leading-[1.45] text-graphite">{subtitle}</p>
            </div>
            <div className="flex flex-col gap-4 p-7">{children}</div>
            <div className="flex items-start justify-center gap-1.5 border-t border-ink px-7 py-4 text-sm">{footer}</div>
          </section>
        </div>
      </main>
    </div>
  );
}

/** Pesan error tingkat form (di dalam panel), memakai warna stop. */
export function FormError({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="border-[1.5px] border-stop px-3 py-2 text-[13px] leading-[1.4] text-stop">
      {children}
    </p>
  );
}
