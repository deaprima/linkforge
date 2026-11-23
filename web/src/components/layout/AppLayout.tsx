import { cn } from "cn";
import type { ReactNode } from "react";
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router";
import { GridBackground, Logo, ThemeToggle } from "@/components/brand/Brand";
import { Button } from "@/components/ui/primitives";
import { useLogout, useMe } from "@/lib/auth";

/*
 * App shell (Figma "Overview — Desktop" 16:278): sidebar 240px berborder kanan + area main.
 * Di bawah lg (Figma "Overview — Mobile" 20:278): top bar 56px (logo, theme toggle, avatar inisial) + tab bar 64px di dasar layar.
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

/** Tab bar mobile (Figma 20:289): empat tab sama lebar, aktif = blok ink; marker 8px di atas label. */
function TabItem({ to, label }: { to: string; label: string }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        cn(
          "flex h-full min-w-0 flex-1 flex-col items-center justify-center gap-1.5 text-xs",
          "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-signal",
          isActive ? "bg-ink font-semibold text-paper" : "bg-paper text-ink",
        )
      }
    >
      {({ isActive }) => (
        <>
          <span aria-hidden className={cn("size-2 border", isActive ? "border-signal bg-signal" : "border-ink")} />
          {label}
        </>
      )}
    </NavLink>
  );
}

/** Avatar 32px berisi inisial (Figma "Account"); membuka Profile, tempat Sign out di mobile. */
function Avatar() {
  const { data: user } = useMe();
  const initials = (user?.name ?? "")
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0].toUpperCase())
    .join("");
  return (
    <Link
      to="/app/profile"
      aria-label="Profile"
      className="flex size-8 shrink-0 items-center justify-center rounded-full border border-ink text-xs font-semibold text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-signal"
    >
      {initials || "?"}
    </Link>
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

/** Detail link (Figma "Link detail — Mobile" 20:456) adalah halaman daun: tanpa tab bar. */
const LEAF_PAGE = /^\/app\/links\/(?!new$)[^/]+$/;

export function AppLayout() {
  const { pathname } = useLocation();
  const tabBar = !LEAF_PAGE.test(pathname);
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
      <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center justify-between border-b border-ink bg-paper px-4 lg:hidden">
        <Link to="/app/dashboard" aria-label="LinkForge home" className="focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-signal">
          <Logo />
        </Link>
        <div className="flex items-center gap-2.5">
          <ThemeToggle />
          <Avatar />
        </div>
      </header>

      <main className={cn("flex min-w-0 flex-1 flex-col gap-4 px-4 pt-6 sm:gap-6 sm:px-10 sm:pt-8 lg:pb-8", tabBar ? "pb-24" : "pb-8")}>
        <Outlet />
      </main>

      {/* Mobile tab bar */}
      {tabBar && (
        <nav aria-label="Main" className="fixed inset-x-0 bottom-0 z-10 flex h-16 border-t border-ink bg-paper lg:hidden">
          {NAV.map((item) => (
            <TabItem key={item.to} {...item} />
          ))}
        </nav>
      )}
    </div>
  );
}

/**
 * Page header (Figma "Page header"): judul + subjudul graphite, aksi di kanan. Di mobile judul 28px; subjudul pendek
 * ("42 links") tetap di bawah judul, yang panjang ("below") turun satu baris penuh di bawah header, atau disembunyikan.
 */
export function PageHeader({
  title,
  subtitle,
  mobileSubtitle = "inline",
  actions,
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  mobileSubtitle?: "inline" | "below" | "hidden";
  actions?: ReactNode;
}) {
  const subClass = "text-sm text-graphite sm:text-[15px]";
  return (
    <div className="flex flex-col gap-3 sm:gap-0">
      <div className="flex items-end justify-between gap-4">
        <div className="flex min-w-0 flex-col gap-1 sm:gap-1.5">
          <h1 className="text-[28px] leading-[1.1] font-semibold tracking-[-0.56px] text-ink sm:text-[32px] sm:tracking-[-0.64px]">{title}</h1>
          {subtitle && <p className={cn(subClass, mobileSubtitle !== "inline" && "hidden sm:block")}>{subtitle}</p>}
        </div>
        {actions && (
          <div className="flex shrink-0 items-start gap-2 [&>button]:h-9 [&>button]:px-3.5 [&>button]:text-sm sm:[&>button]:h-11 sm:[&>button]:px-5 sm:[&>button]:text-[15px]">{actions}</div>
        )}
      </div>
      {subtitle && mobileSubtitle === "below" && <p className={cn(subClass, "sm:hidden")}>{subtitle}</p>}
    </div>
  );
}

/** Panel berborder dengan header (Figma "Panel header": 16px semibold + keterangan graphite 13px). */
export function Panel({ title, aside, className, children }: { title?: ReactNode; aside?: ReactNode; className?: string; children: ReactNode }) {
  return (
    <section className={cn("flex flex-col overflow-clip border border-ink bg-paper", className)}>
      {title && (
        <div className="flex items-center justify-between gap-3 border-b border-ink px-4 py-3 sm:px-5 sm:py-3.5">
          <h2 className="text-[15px] font-semibold text-ink sm:text-base">{title}</h2>
          {aside && <div className="text-xs text-graphite sm:text-[13px]">{aside}</div>}
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
