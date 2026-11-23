import { cn } from "cn";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Link } from "react-router";
import { BackToTop } from "@/components/BackToTop";
import { GridBackground } from "@/components/brand/Brand";
import { TopBar } from "@/components/layout/AuthLayout";
import { buttonClass } from "@/components/ui/primitives";
import { useMe } from "@/lib/auth";
import { groups, info, slug, splitIntro, type TagGroup } from "@/lib/openapi";
import { API_SPEC_URL, REPO_URL } from "@/pages/landing/links";
import { Markdown } from "@/pages/docs/Markdown";
import { MethodBadge, OperationCard } from "@/pages/docs/OperationCard";

/*
 * API reference untuk developer (/app/docs). Seluruh isinya dibangun dari api/openapi.yml, jadi dokumentasi,
 * Swagger UI, dan kontrak tidak pernah berbeda. Tidak ada desain Figma: disusun dari token dan komponen yang ada.
 */

type Env = "production" | "local";
const ENV_KEY = "linkforge:docs-env";
const SWAGGER_URL = import.meta.env.VITE_SWAGGER_URL;

function useEnv(): [Env, (e: Env) => void] {
  const [env, setEnv] = useState<Env>(() => {
    try {
      return localStorage.getItem(ENV_KEY) === "local" ? "local" : "production";
    } catch {
      return "production";
    }
  });
  const set = (next: Env) => {
    setEnv(next);
    try {
      localStorage.setItem(ENV_KEY, next);
    } catch {
      /* penyimpanan diblokir: pilihan berlaku untuk kunjungan ini saja */
    }
  };
  return [env, set];
}

/** Id section yang sedang dibaca, untuk menandai item di sidebar. */
function useActiveId(ids: string[]): string | null {
  const [active, setActive] = useState<string | null>(null);
  const key = ids.join("|");
  useEffect(() => {
    if (typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
        if (visible) setActive(visible.target.id);
      },
      { rootMargin: "-80px 0px -65% 0px" },
    );
    for (const id of key.split("|")) {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, [key]);
  return active;
}

function NavLink({ href, active, children }: { href: string; active: boolean; children: ReactNode }) {
  return (
    <a
      href={href}
      aria-current={active ? "location" : undefined}
      className={cn(
        "flex items-center gap-3 border-l-2 py-2.5 pr-3 pl-4 text-[17px] leading-[1.3]",
        "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-signal",
        active ? "border-signal bg-signal-soft font-semibold text-ink" : "border-transparent text-graphite hover:text-ink",
      )}
    >
      {children}
    </a>
  );
}

function SideNav({ intro, tagGroups, active }: { intro: { id: string; title: string }[]; tagGroups: TagGroup[]; active: string | null }) {
  return (
    <nav aria-label="API reference" className="flex flex-col gap-8">
      <div className="flex flex-col">
        <p className="mb-2 pl-4 text-sm font-semibold tracking-wide text-ink uppercase">Guide</p>
        {intro.map((s) => (
          <NavLink key={s.id} href={`#${s.id}`} active={active === s.id}>
            {s.title}
          </NavLink>
        ))}
      </div>
      {tagGroups.map((g) => (
        <div key={g.name} className="flex flex-col">
          <p className="mb-2 pl-4 text-sm font-semibold tracking-wide text-ink uppercase">{g.name}</p>
          {g.operations.map((op) => (
            <NavLink key={op.id} href={`#${op.id}`} active={active === op.id}>
              <MethodBadge method={op.method} className="w-14 px-0.5 text-[11px]" />
              <span className="min-w-0">{op.summary}</span>
            </NavLink>
          ))}
        </div>
      ))}
    </nav>
  );
}

function EnvSwitch({ env, onChange }: { env: Env; onChange: (e: Env) => void }) {
  const options: { value: Env; label: string }[] = [
    { value: "production", label: "Production" },
    { value: "local", label: "Local" },
  ];
  return (
    <div role="radiogroup" aria-label="Base URL for examples" className="flex border border-ink">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={env === o.value}
          onClick={() => onChange(o.value)}
          className={cn(
            "cursor-pointer px-4 py-2 text-[15px] focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-signal",
            env === o.value ? "bg-ink font-semibold text-paper" : "bg-paper text-ink hover:bg-grid",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function DocsPage() {
  const [env, setEnv] = useEnv();
  const { data: user, isPending } = useMe();
  const intro = useMemo(() => splitIntro(info.description), []);
  const tagGroups = useMemo(groups, []);
  const ids = useMemo(() => [...intro.sections.map((s) => s.id), ...tagGroups.flatMap((g) => g.operations.map((o) => o.id))], [intro, tagGroups]);
  const active = useActiveId(ids);
  const baseUrl = (env === "local" ? "http://localhost:8080" : "https://linkforge.deaprima.my.id") + "/api/v1";

  useEffect(() => {
    document.title = "API reference · LinkForge";
    // Halaman dimuat malas, jadi browser belum menemukan anchor (#createLink) saat navigasi; scroll manual sekali
    const target = window.location.hash && document.getElementById(decodeURIComponent(window.location.hash.slice(1)));
    if (target) target.scrollIntoView();
  }, []);

  return (
    <div className="flex min-h-dvh flex-col">
      <GridBackground />
      <TopBar className="sticky top-0 z-20">
        <Link to="/" className={buttonClass({ variant: "outline", size: "sm" }, "hidden sm:inline-flex")}>
          Home
        </Link>
        {user ? (
          <Link to="/app/dashboard" className={buttonClass({ size: "sm" })}>
            Dashboard
          </Link>
        ) : (
          <Link to="/app/login" className={buttonClass({ size: "sm" }, isPending ? "invisible" : "")}>
            Sign in
          </Link>
        )}
      </TopBar>

      <div className="mx-auto flex w-full max-w-[1520px] flex-1 gap-12 px-4 py-8 sm:px-10 sm:py-14">
        <aside className="sticky top-24 hidden max-h-[calc(100dvh-7rem)] w-[350px] shrink-0 self-start overflow-y-auto lg:block">
          <SideNav intro={intro.sections} tagGroups={tagGroups} active={active} />
        </aside>

        <main className="flex min-w-0 max-w-[1000px] flex-1 flex-col gap-14">
          <header className="flex flex-col gap-4">
            <p className="font-mono text-sm text-graphite">
              API reference · v{info.version}
            </p>
            <h1 className="text-[44px] leading-[1.05] font-semibold tracking-[-1.2px] text-ink sm:text-[64px]">{info.title}</h1>
            <Markdown text={intro.lead} className="flex max-w-[760px] flex-col gap-3 text-xl leading-[1.55] text-graphite" />
            <div className="flex flex-wrap items-center gap-3 pt-1">
              <EnvSwitch env={env} onChange={setEnv} />
              <code className="font-mono text-sm break-all text-ink">{baseUrl}</code>
            </div>
            <div className="flex flex-wrap gap-3">
              {SWAGGER_URL && (
                <a href={SWAGGER_URL} target="_blank" rel="noreferrer noopener" className={buttonClass({})}>
                  Try it in Swagger UI
                </a>
              )}
              <a href={API_SPEC_URL} className={buttonClass({ variant: "outline" })}>
                OpenAPI spec
              </a>
              <a href={REPO_URL} className={buttonClass({ variant: "outline" })}>
                GitHub
              </a>
            </div>
          </header>

          <details className="border border-ink bg-paper lg:hidden">
            <summary className="cursor-pointer px-4 py-3.5 text-base font-semibold text-ink focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-signal">Jump to</summary>
            <div className="border-t border-ink p-3">
              <SideNav intro={intro.sections} tagGroups={tagGroups} active={active} />
            </div>
          </details>

          {intro.sections.map((s) => (
            <section key={s.id} id={s.id} aria-labelledby={`${s.id}-title`} className="flex scroll-mt-24 flex-col gap-4">
              <h2 id={`${s.id}-title`} className="text-[36px] leading-[1.1] font-semibold tracking-[-0.72px] text-ink">
                {s.title}
              </h2>
              <Markdown text={s.body} />
            </section>
          ))}

          {tagGroups.map((g) => {
            const gid = `tag-${slug(g.name)}`;
            return (
              <section key={g.name} id={gid} aria-labelledby={`${gid}-title`} className="flex scroll-mt-24 flex-col gap-5">
                <div className="flex flex-col gap-2 border-t border-ink pt-8">
                  <h2 id={`${gid}-title`} className="text-[36px] leading-[1.1] font-semibold tracking-[-0.72px] text-ink">
                    {g.name}
                  </h2>
                  {g.description && <p className="text-lg text-graphite">{g.description}</p>}
                </div>
                {g.operations.map((op) => (
                  <OperationCard key={op.id} op={op} env={env} />
                ))}
              </section>
            );
          })}

          <footer className="border-t border-ink pt-6 pb-4 text-base text-graphite">
            {info.license && (
              <p>
                Released under the{" "}
                <a href={info.license.url} className="font-semibold text-ink underline">
                  {info.license.name} license
                </a>
                . Found a mistake in these docs?{" "}
                <a href={`${REPO_URL}/issues`} className="font-semibold text-ink underline">
                  Open an issue
                </a>
                .
              </p>
            )}
          </footer>
        </main>
      </div>
      <BackToTop />
    </div>
  );
}
