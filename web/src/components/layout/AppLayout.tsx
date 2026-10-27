import { cn } from "cn";
import type { ReactNode } from "react";
import { NavLink, Outlet, useNavigate } from "react-router";
import { GridBackground, Logo, ThemeToggle } from "@/components/brand/Brand";
import { Button } from "@/components/ui/primitives";
import { useLogout, useMe } from "@/lib/auth";

/*
 * App shell (Figma "Overview — Desktop" 16:278): sidebar 240px berborder kanan + area main.
 * Di layar sempit sidebar berubah jadi top bar + nav horizontal.
 */

const NAV = [
  { to: "/app/dashboard", label: "Overview" },
  { to: "/app/links", label: "Links" },
  { to: "/app/api-keys", label: "API keys" },
  { to: "/app/profile", label: "Profile" },
];

/** Nav item (Figma 15:297): marker 8px — aktif = blok ink + marker signal. */
function NavItem({ to, label, end }: { to: string; label: string; end?: boolean }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cn(
          "flex shrink-0 items-center gap-2.5 px-3 py-2.5 text-[15px] whitespace-nowrap",
          "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-signal",
          isActive ? "bg-ink font-semibold text-paper" : "bg-paper text-ink hover:bg-grid",
        )
      }
    >
      {({ isActive }) => (
        <>
          <span aria-hidden className={cn("size-2 shrink-0 border", isActive ? "border-signal bg-signal" : "border-ink")} />
          {label}
        </>
      )}
    </NavLink>
  );
}

function Account() {
  const { data: user } = useMe();
  const logout = useLogout();
  const navigate = useNavigate();
  return (
    <div className="flex flex-col gap-0.5 border-t border-ink px-3 pt-4">
      <p className="truncate text-sm font-semibold text-ink">{user?.name}</p>
      <p className="truncate text-[13px] text-graphite">{user?.email}</p>
      <div className="h-2.5" />
      <Button
        variant="outline"
        size="sm"
        className="self-start"
        loading={logout.isPending}
        onClick={() => logout.mutate(undefined, { onSettled: () => navigate("/app/login", { replace: true }) })}
      >
        Sign out
      </Button>
    </div>
  );
}

export function AppLayout() {
  return (
    <div className="flex min-h-dvh flex-col lg:flex-row">
      <GridBackground />
      {/* Desktop sidebar */}
      <aside className="sticky top-0 hidden h-dvh w-60 shrink-0 flex-col gap-1 border-r border-ink bg-paper px-4 py-5 lg:flex">
        <div className="flex items-center justify-between pt-1 pb-6 pl-3">
          <Logo />
          <ThemeToggle />
        </div>
        <nav aria-label="Main" className="flex flex-col gap-1">
          {NAV.map((item) => (
            <NavItem key={item.to} {...item} />
          ))}
        </nav>
        <div className="flex-1" />
        <Account />
      </aside>

      {/* Mobile top bar */}
      <header className="sticky top-0 z-10 flex flex-col border-b border-ink bg-paper lg:hidden">
        <div className="flex h-14 items-center justify-between px-4">
          <Logo />
          <ThemeToggle />
        </div>
        <nav aria-label="Main" className="flex gap-1 overflow-x-auto px-2 pb-2">
          {NAV.map((item) => (
            <NavItem key={item.to} {...item} />
          ))}
        </nav>
      </header>

      <main className="flex min-w-0 flex-1 flex-col gap-6 px-4 py-6 sm:px-10 sm:py-8">
        <Outlet />
      </main>
    </div>
  );
}

/** Page header (Figma "Page header"): judul 32px + subjudul graphite, aksi di kanan. */
export function PageHeader({ title, subtitle, actions }: { title: ReactNode; subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-4">
      <div className="flex min-w-0 flex-col gap-1.5">
        <h1 className="text-[32px] leading-[1.1] font-semibold tracking-[-0.64px] text-ink">{title}</h1>
        {subtitle && <p className="text-[15px] text-graphite">{subtitle}</p>}
      </div>
      {actions && <div className="flex items-start gap-2">{actions}</div>}
    </div>
  );
}

/** Panel berborder dengan header (Figma "Panel header": 16px semibold + keterangan graphite 13px). */
export function Panel({ title, aside, className, children }: { title?: ReactNode; aside?: ReactNode; className?: string; children: ReactNode }) {
  return (
    <section className={cn("flex flex-col overflow-clip border border-ink bg-paper", className)}>
      {title && (
        <div className="flex items-center justify-between gap-3 border-b border-ink px-5 py-3.5">
          <h2 className="text-base font-semibold text-ink">{title}</h2>
          {aside && <div className="text-[13px] text-graphite">{aside}</div>}
        </div>
      )}
      {children}
    </section>
  );
}

/** State kosong / error di dalam panel. */
export function PanelMessage({ children, tone = "muted" }: { children: ReactNode; tone?: "muted" | "error" }) {
  return <p className={cn("px-5 py-10 text-center text-sm", tone === "error" ? "text-stop" : "text-graphite")}>{children}</p>;
}
