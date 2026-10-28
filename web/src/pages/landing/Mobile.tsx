import type { ReactNode } from "react";
import { Link } from "react-router";
import barOriginalDark from "@/assets/landing/bar-original-dark.svg";
import barOriginalLight from "@/assets/landing/bar-original-light.svg";
import barShortDark from "@/assets/landing/bar-short-dark.svg";
import barShortLight from "@/assets/landing/bar-short-light.svg";
import lineDark from "@/assets/landing/line-dark.svg";
import lineLight from "@/assets/landing/line-light.svg";
import qrDark from "@/assets/landing/qr-dark.svg";
import qrLight from "@/assets/landing/qr-light.svg";
import sparklineDark from "@/assets/landing/sparkline-dark.svg";
import sparklineLight from "@/assets/landing/sparkline-light.svg";
import { Themed } from "@/components/brand/Icons";
import { MascotHero } from "@/components/brand/MascotHero";
import { buttonClass, Chip } from "@/components/ui/primitives";
import { Caption } from "@/pages/landing/Hero";
import { REPO_URL } from "@/pages/landing/links";

/*
 * Landing page versi mobile (Figma 14:142, 390px). Strukturnya berbeda dari desktop — stasiun vertikal,
 * daftar fitur berupa kartu, panel Fig. 3 berupa tumpukan state — jadi dibuat sebagai komponen terpisah
 * yang hanya tampil di bawah breakpoint lg. Id anchor diberi akhiran "-m" supaya tidak bentrok dengan desktop.
 */

const section = "px-4 pt-10 pb-10";
const h2 = "text-[32px] leading-[1.05] font-semibold tracking-[-0.8px] text-ink";
const lede = "text-[15px] leading-[1.5] text-graphite";

export function MobileHero() {
  return (
    <section aria-labelledby="hero-title-m" className="flex flex-col px-4 pt-10 pb-8">
      <h1 id="hero-title-m" className="text-[43px] leading-[1.05] font-semibold tracking-[-1.075px] text-ink">
        Shorten a link.
        <br />
        See who clicks it.
      </h1>
      <p className="mt-[18px] text-base leading-[1.5] text-graphite">
        LinkForge turns long URLs into short ones you control, with custom names, expiry dates, passwords, QR codes and live click analytics. Free and open source.
      </p>
      <div className="mt-6 flex flex-col gap-2.5">
        <Link to="/app/register" className={buttonClass({}, "w-full")}>
          Create free account
        </Link>
        <a href={REPO_URL} className={buttonClass({ variant: "outline" }, "w-full")}>
          Read the source
        </a>
      </div>

      <figure className="mt-9 flex flex-col gap-2 border border-ink bg-paper px-4 pt-5 pb-4 font-mono text-[11px] leading-[normal]">
        <div className="flex justify-between text-ink">
          <span>Original</span>
          <span>78 characters</span>
        </div>
        <Themed light={barOriginalLight} dark={barOriginalDark} className="block h-2.5 w-[300px] max-w-full" />
        <p className="leading-[1.6] break-words text-graphite">https://docs.google.com/presentation/d/1a9XkLmQ2vRt/edit?usp=sharing&amp;ouid=1042</p>
        <div className="h-3" aria-hidden />
        <div className="flex gap-4 text-signal-ink">
          <span>Short</span>
          <span>30 characters</span>
        </div>
        <Themed light={barShortLight} dark={barShortDark} className="block h-2.5 w-[115px]" />
        <p className="font-bold text-ink">linkforge.deaprima.my.id/pitch</p>
        <div className="h-2" aria-hidden />
        <figcaption className="font-sans">
          <Caption fig={1}>Same destination, 48 characters shorter. Bars drawn to scale.</Caption>
        </figcaption>
      </figure>

      <MascotHero className="mt-2 ml-auto size-[130px]" />
    </section>
  );
}

const stations = [
  { title: "Gateway", body: "Checks your rate limit and passes the request to the right service.", tag: "HTTP in, gRPC out" },
  { title: "Link service", body: "Finds the alias and sends the visitor to the destination with a 302.", tag: "302 redirect" },
  { title: "Kafka", body: "Records the click as an event, so the redirect never waits for analytics.", tag: "topic link.clicked" },
  { title: "Analytics", body: "Counts the click by country, browser and device and pushes it to your dashboard.", tag: "WebSocket to you" },
];

export function MobileHowItWorks() {
  return (
    <section id="how-it-works-m" aria-labelledby="how-title-m" className={section}>
      <h2 id="how-title-m" className={h2}>
        What happens when someone clicks
      </h2>
      <p className={`mt-3 ${lede}`}>Four separate Go services, each doing one job.</p>
      <ol className="mt-9 flex flex-col">
        {stations.map((s, i) => (
          <li key={s.title} className="flex flex-col">
            {i > 0 && <Themed light={lineLight} dark={lineDark} className="ml-[33px] block h-5 w-px" />}
            <div className="flex flex-col gap-1.5 border border-ink bg-paper p-5">
              <span className="mb-2.5 flex size-7 items-center justify-center rounded-full border-[1.5px] border-ink text-[13px] font-semibold text-ink">{i + 1}</span>
              <h3 className="text-lg leading-[normal] font-semibold text-ink">{s.title}</h3>
              <p className="text-sm leading-[1.45] text-graphite">{s.body}</p>
              <p className="pt-2 font-mono text-[11px] text-ink">{s.tag}</p>
            </div>
          </li>
        ))}
      </ol>
      <div className="mt-3">
        <Caption fig={2}>Request path for one click</Caption>
      </div>
    </section>
  );
}

const features: { name: string; body: string; example: ReactNode }[] = [
  { name: "Custom alias", body: "Choose the word after the slash, or let LinkForge generate one.", example: <Chip className="leading-[normal]">/pitch</Chip> },
  { name: "Expiry date", body: "The link stops working on the date you set.", example: <Chip className="leading-[normal]">31 Dec 2026</Chip> },
  { name: "Password", body: "Visitors enter a password before they are sent on.", example: <Chip className="leading-[normal]">••••••••</Chip> },
  { name: "QR code", body: "Download as PNG or SVG at any size.", example: <Themed light={qrLight} dark={qrDark} className="block size-16" /> },
  { name: "Click analytics", body: "Daily clicks, countries, browsers and devices, updated live.", example: <Themed light={sparklineLight} dark={sparklineDark} className="block h-9 w-[120px]" /> },
  { name: "API keys", body: "Shorten from scripts and CI. Revoke any key on its own.", example: <Chip className="leading-[normal]">lf_sk_a3f9…</Chip> },
];

export function MobileFeatures() {
  return (
    <section id="features-m" aria-labelledby="features-title-m" className={section}>
      <h2 id="features-title-m" className={h2}>
        What you get
      </h2>
      <p className={`mt-3 ${lede}`}>Every feature is free, from the dashboard or the API.</p>
      <ol className="mt-8 border border-ink bg-paper">
        {features.map((f, i) => (
          <li key={f.name} className="flex gap-3.5 border-t border-ink p-4 first:border-t-0">
            <span className="text-[15px] leading-[normal] font-semibold text-ink">{i + 1}</span>
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <h3 className="text-base leading-[normal] font-semibold text-ink">{f.name}</h3>
              <p className="text-sm leading-[1.45] text-graphite">{f.body}</p>
              <div className="mt-3 flex">{f.example}</div>
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}

const c = "text-graphite";
const s = "text-signal-ink";

export function MobileDevelopers() {
  return (
    <section id="developers-m" aria-labelledby="dev-title-m" className={section}>
      <h2 id="dev-title-m" className={h2}>
        Shorten from your terminal
      </h2>
      <p className={`mt-3 ${lede}`}>
        Create an API key in the dashboard and send it in the <code className="font-mono text-xs text-ink">X-API-Key</code> header.
      </p>
      <div className="mt-8 overflow-x-auto border border-ink bg-paper p-4">
        <pre className="font-mono text-[11px] leading-[1.7] whitespace-pre text-ink">
          <code>
            <span className={c}># Shorten a link</span>
            {"\ncurl -X POST \\\n  …/api/v1/links \\\n  -H "}
            <span className={s}>"X-API-Key: lf_sk_a3f9••••"</span>
            {" \\\n  -d '{\"original_url"}
            <span className={s}>":"</span>
            {"…\",\n       \"alias"}
            <span className={s}>":"</span>
            {"pitch\"}'\n\n"}
            <span className={c}># 201 Created</span>
            {"\n{ \"alias"}
            <span className={s}>": "</span>
            {"pitch\",\n  \"short_url"}
            <span className={s}>": "</span>
            {"…/pitch\" }"}
          </code>
        </pre>
      </div>
    </section>
  );
}

export function MobileCta() {
  return (
    <section aria-labelledby="cta-title-m" className="px-4 py-10">
      <div className="flex flex-col items-center border border-ink bg-paper px-4 py-8 text-center">
      <MascotHero className="size-[120px]" />
      <h2 id="cta-title-m" className="mt-3 text-[30px] leading-[1.05] font-semibold tracking-[-0.75px] text-ink">
        Make your first short link
      </h2>
      <p className="mt-3 text-[15px] text-graphite">Sign up with email or Google. No card needed.</p>
      <Link to="/app/register" className={buttonClass({}, "mt-8 w-full")}>
        Create free account
      </Link>
      </div>
    </section>
  );
}
