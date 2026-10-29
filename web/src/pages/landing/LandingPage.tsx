import { cn } from "cn";
import { useEffect, useState, type ReactNode } from "react";
import { Link } from "react-router";
import menuDark from "@/assets/landing/menu-dark.svg";
import menuLight from "@/assets/landing/menu-light.svg";
import rulerDark from "@/assets/landing/ruler-dark.svg";
import rulerLight from "@/assets/landing/ruler-light.svg";
import rulerMobileDark from "@/assets/landing/ruler-m-dark.svg";
import rulerMobileLight from "@/assets/landing/ruler-m-light.svg";
import { BackToTop } from "@/components/BackToTop";
import { Logo, ThemeToggle } from "@/components/brand/Brand";
import { Themed } from "@/components/brand/Icons";
import { buttonClass } from "@/components/ui/primitives";
import { useMe } from "@/lib/auth";
import { Hero } from "@/pages/landing/Hero";
import { REPO_URL } from "@/pages/landing/links";
import { MobileCta, MobileDevelopers, MobileFeatures, MobileHero, MobileHowItWorks } from "@/pages/landing/Mobile";
import { Cta, Developers, Features, HowItWorks } from "@/pages/landing/Sections";

/*
 * Landing page (Figma halaman "Landing", 13:2 desktop / 14:142 mobile): gambar teknik satu lembar —
 * nav, penggaris, hero + Fig. 1, bagian bernomor, CTA, dan "title block" di dasar lembar.
 * Desktop dan mobile punya susunan berbeda di Figma, jadi tiap bagian punya dua komponen: desktop (lg ke atas)
 * dan mobile (di bawah lg).
 */

const navLinks = [
  { label: "How it works", href: "#how-it-works", mobileHref: "#how-it-works-m" },
  { label: "Features", href: "#features", mobileHref: "#features-m" },
  { label: "Developers", href: "#developers", mobileHref: "#developers-m" },
];

function Nav() {
  const [open, setOpen] = useState(false);
  const { data: user, isPending } = useMe();
  // Selama status login belum diketahui, tombol tamu tetap dirender tapi tak tampak: layout tidak bergeser
  // dan pengguna yang sudah login tidak sempat melihat "Create free account" berkedip.
  const pendingHide = isPending ? "invisible" : "";
  return (
    <div className="sticky top-0 z-30">
      <header className="flex h-[60px] items-center justify-between border-b border-ink bg-paper px-4 lg:h-[72px] lg:px-gutter">
        <Link to="/" aria-label="LinkForge home" className="focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-signal">
          <Logo />
        </Link>
        <nav aria-label="Sections" className="hidden items-center gap-7 text-[15px] lg:flex">
          {navLinks.map((l) => (
            <a key={l.label} href={l.href} className="hover:underline">
              {l.label}
            </a>
          ))}
          <Link to="/app/docs" className="hover:underline">
            Docs
          </Link>
          <a href={REPO_URL} className="hover:underline">
            GitHub
          </a>
        </nav>
        <div className="flex items-center gap-2.5">
          <ThemeToggle />
          {user ? (
            <Link to="/app/dashboard" className={buttonClass({ size: "sm" }, "hidden lg:inline-flex")}>
              Dashboard
            </Link>
          ) : (
            <>
              <Link to="/app/login" className={buttonClass({ variant: "outline", size: "sm" }, cn("hidden lg:inline-flex", pendingHide))}>
                Sign in
              </Link>
              <Link to="/app/register" className={buttonClass({ size: "sm" }, cn("hidden lg:inline-flex", pendingHide))}>
                Create free account
              </Link>
            </>
          )}
          <button
            type="button"
            aria-label={open ? "Close menu" : "Open menu"}
            aria-expanded={open}
            aria-controls="landing-menu"
            onClick={() => setOpen((o) => !o)}
            className="cursor-pointer focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-signal lg:hidden"
          >
            <Themed light={menuLight} dark={menuDark} className="block size-7" />
          </button>
        </div>
      </header>
      {open && (
        <div id="landing-menu" className="absolute inset-x-0 top-full z-20 flex flex-col gap-4 border-b border-ink bg-paper px-4 py-4 lg:hidden">
          {navLinks.map((l) => (
            <a key={l.label} href={l.mobileHref} onClick={() => setOpen(false)} className="text-[15px]">
              {l.label}
            </a>
          ))}
          <Link to="/app/docs" className="text-[15px]">
            Docs
          </Link>
          <a href={REPO_URL} className="text-[15px]">
            GitHub
          </a>
          <div className="flex flex-col gap-2.5 pt-1">
            {user ? (
              <Link to="/app/dashboard" className={buttonClass({}, "w-full")}>
                Dashboard
              </Link>
            ) : (
              <>
                <Link to="/app/register" className={buttonClass({}, "w-full")}>
                  Create free account
                </Link>
                <Link to="/app/login" className={buttonClass({ variant: "outline" }, "w-full")}>
                  Sign in
                </Link>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

/** Penggaris di bawah nav (Figma "Ruler"): tick berukuran tetap, dipotong kalau layar lebih sempit. */
function Ruler() {
  return (
    <div aria-hidden className="overflow-hidden bg-paper px-[15px] lg:px-gutter">
      <Themed light={rulerMobileLight} dark={rulerMobileDark} className="block h-3.5 w-[334px] max-w-none lg:hidden" />
      <Themed light={rulerLight} dark={rulerDark} className="hidden h-[18px] w-[1310px] max-w-none lg:block" />
    </div>
  );
}

/** Garis pemisah antar bagian: selebar isi (bukan layar penuh), ujungnya diberi tanda + seperti penggaris. */
function Divider() {
  const plus = (
    <svg viewBox="0 0 13 13" className="size-[13px] shrink-0 text-ink">
      <path d="M6.5 0v13M0 6.5h13" stroke="currentColor" strokeWidth="1" fill="none" />
    </svg>
  );
  return (
    <div aria-hidden className="px-4 lg:px-gutter">
      <div className="-mx-[6.5px] flex items-center gap-2">
        {plus}
        <div className="flex-1 border-t border-ink" />
        {plus}
      </div>
    </div>
  );
}

/** Satu bagian punya versi desktop dan mobile; hanya salah satunya yang tampil. */
function Split({ desktop, mobile }: { desktop: ReactNode; mobile: ReactNode }) {
  return (
    <>
      <div className="hidden lg:block">{desktop}</div>
      <div className="lg:hidden">{mobile}</div>
    </>
  );
}

/** "Title block" di dasar lembar gambar teknik. */
function TitleCell({ label, className, children }: { label: string; className?: string; children: ReactNode }) {
  return (
    <div className={`flex min-w-0 flex-col gap-1 border-ink px-4 py-3 ${className ?? ""}`}>
      <p className="text-[11px] text-graphite">{label}</p>
      <div className="text-[15px] whitespace-nowrap text-ink">{children}</div>
    </div>
  );
}

function TitleBlock() {
  return (
    <footer className="grid grid-cols-2 border border-ink bg-paper lg:flex">
      <TitleCell label="Project" className="col-span-2 border-b lg:flex-[2.2] lg:border-r lg:border-b-0">
        <span className="font-semibold">LinkForge URL shortener</span>
      </TitleCell>
      <TitleCell label="Built by" className="border-r lg:flex-[1.3]">
        <span className="font-semibold">Dea Prima</span>
      </TitleCell>
      <TitleCell label="Stack" className="hidden lg:flex lg:flex-1 lg:border-r">
        Go, gRPC, Kafka
      </TitleCell>
      <TitleCell label="Links" className="order-last col-span-2 border-t lg:order-none lg:col-span-1 lg:flex-1 lg:border-t-0 lg:border-r">
        <span className="flex gap-3 lg:gap-4">
          <a href={REPO_URL} className="hover:underline">
            GitHub
          </a>
          <Link to="/app/docs" className="hover:underline">
            API
          </Link>
        </span>
      </TitleCell>
      <TitleCell label="Rev." className="lg:flex-[0.5]">
        <span className="font-semibold">1.0</span>
      </TitleCell>
    </footer>
  );
}

export function LandingPage() {
  useEffect(() => {
    document.title = "LinkForge — Shorten a link. See who clicks it.";
  }, []);
  // leading-[normal]: Figma memakai line-height "normal" kecuali disebut lain (Tailwind/body default 1.5)
  return (
    <div className="leading-[normal]">
      <div className="sheet-grid">
        <Nav />
        <Ruler />
        <main>
          <Split desktop={<Hero />} mobile={<MobileHero />} />
          <Divider />
          <Split desktop={<HowItWorks />} mobile={<MobileHowItWorks />} />
          <Divider />
          <Split desktop={<Features />} mobile={<MobileFeatures />} />
          <Divider />
          <Split desktop={<Developers />} mobile={<MobileDevelopers />} />
          <Divider />
          <Split desktop={<Cta />} mobile={<MobileCta />} />
        </main>
        <div className="px-4 pb-6 lg:px-gutter lg:pb-10">
          <TitleBlock />
        </div>
        <BackToTop />
      </div>
    </div>
  );
}
