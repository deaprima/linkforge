import { Fragment, type ReactNode } from "react";
import { Link } from "react-router";
import arrowDark from "@/assets/landing/arrow-dark.svg";
import arrowLight from "@/assets/landing/arrow-light.svg";
import qrDark from "@/assets/landing/qr-dark.svg";
import qrLight from "@/assets/landing/qr-light.svg";
import sparklineDark from "@/assets/landing/sparkline-dark.svg";
import sparklineLight from "@/assets/landing/sparkline-light.svg";
import { Themed } from "@/components/brand/Icons";
import { Kliko } from "@/components/brand/Kliko";
import { buttonClass, Chip } from "@/components/ui/primitives";
import { Caption } from "@/pages/landing/Hero";

// scroll-mt-20: anchor (#features) berhenti di bawah nav yang sticky (72px)
const section = "scroll-mt-20 px-gutter py-[72px]";
const h2 = "text-[44px] leading-[1.05] font-semibold tracking-[-1.1px] text-ink";

// How it works (Figma 13:104)

const stations = [
  { title: "Gateway", body: "Checks your rate limit and passes the request to the right service.", tag: "HTTP in, gRPC out" },
  { title: "Link service", body: "Finds the alias and sends the visitor to the destination with a 302.", tag: "302 redirect" },
  { title: "Kafka", body: "Records the click as an event, so the redirect never waits for analytics.", tag: "topic link.clicked" },
  { title: "Analytics", body: "Counts the click by country, browser and device and pushes it to your dashboard.", tag: "WebSocket to you" },
];

export function HowItWorks() {
  return (
    <section id="how-it-works" aria-labelledby="how-title" className={`${section} flex flex-col gap-10`}>
      <div className="flex items-end justify-between gap-10">
        <h2 id="how-title" className={`${h2} w-[620px] max-w-full`}>
          What happens when someone clicks
        </h2>
        <p className="text-base leading-[1.5] text-graphite w-[400px] max-w-[40%]">
          Four separate Go services, each doing one job. The visitor is redirected before the click is even counted.
        </p>
      </div>
      <ol className="flex items-center">
        {stations.map((s, i) => (
          <Fragment key={s.title}>
            {i > 0 && <Themed light={arrowLight} dark={arrowDark} className="h-5 w-10" />}
            <li className="flex flex-1 flex-col gap-1.5 self-stretch border border-ink bg-paper p-5">
              <span className="mb-2.5 flex size-7 items-center justify-center rounded-full border-[1.5px] border-ink text-[13px] font-semibold text-ink">{i + 1}</span>
              <h3 className="text-lg leading-[normal] font-semibold text-ink">{s.title}</h3>
              <p className="text-sm leading-[1.45] text-graphite">{s.body}</p>
              <p className="pt-2 font-mono text-[11px] text-ink">{s.tag}</p>
            </li>
          </Fragment>
        ))}
      </ol>
      <Caption fig={2}>Request path for one click on linkforge.deaprima.my.id/pitch</Caption>
    </section>
  );
}

// Features (Figma 13:148)

const features: { name: string; body: string; example: ReactNode }[] = [
  { name: "Custom alias", body: "Choose the word after the slash, or let LinkForge generate one.", example: <Chip className="leading-[normal]">/pitch</Chip> },
  { name: "Expiry date", body: "The link stops working on the date you set. Visitors see that it has expired.", example: <Chip className="leading-[normal]">31 Dec 2026</Chip> },
  { name: "Password", body: "Visitors enter a password before they are sent on.", example: <Chip className="leading-[normal]">••••••••</Chip> },
  { name: "QR code", body: "Download as PNG or SVG at any size.", example: <span className="flex"><Themed light={qrLight} dark={qrDark} className="size-16" /></span> },
  { name: "Click analytics", body: "Daily clicks, countries, browsers and devices, updated live.", example: <span className="flex"><Themed light={sparklineLight} dark={sparklineDark} className="h-9 w-[120px]" /></span> },
  { name: "API keys", body: "Shorten from scripts and CI. One key per tool; revoke any key on its own.", example: <Chip className="leading-[normal]">lf_sk_a3f9…</Chip> },
];

const th = "border-l border-ink px-4 py-2.5 text-left text-[13px] leading-[1.45] font-semibold text-ink first:border-l-0";
const td = "border-l border-ink px-4 py-[18px] first:border-l-0";

export function Features() {
  return (
    <section id="features" aria-labelledby="features-title" className={`${section} flex flex-col gap-10`}>
      <div className="flex items-end justify-between gap-10">
        <h2 id="features-title" className={h2}>
          What you get
        </h2>
        <p className="text-base leading-[1.5] text-graphite w-[400px] max-w-[40%]">Every feature is free. Use them from the dashboard or the API.</p>
      </div>
      <div className="overflow-x-auto border border-ink bg-paper">
        <table className="w-full min-w-[720px] table-fixed border-collapse">
          <colgroup>
            <col className="w-16" />
            <col className="w-[200px]" />
            <col />
            <col className="w-[260px]" />
          </colgroup>
          <thead>
            <tr>
              <th className={`${th} text-center`}>Item</th>
              <th className={th}>Feature</th>
              <th className={th}>What it does</th>
              <th className={th}>Example</th>
            </tr>
          </thead>
          <tbody>
            {features.map((f, i) => (
              <tr key={f.name} className="border-t border-ink">
                <td className={`${td} text-center text-[15px] leading-[1.45] font-semibold text-ink`}>{i + 1}</td>
                <th scope="row" className={`${td} text-left text-base leading-[1.45] font-semibold text-ink`}>
                  {f.name}
                </th>
                <td className={`${td} text-[15px] leading-[1.45] text-graphite`}>{f.body}</td>
                <td className={td}>{f.example}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

// Developers (Figma 13:267)

const c = "text-graphite"; // komentar
const s = "text-signal-ink"; // string

export function Developers() {
  return (
    <section id="developers" aria-labelledby="dev-title" className={`${section} flex items-start gap-16`}>
      <div className="flex w-[420px] shrink-0 flex-col items-start gap-5">
        <h2 id="dev-title" className={h2}>
          Shorten from your terminal
        </h2>
        <p className="text-[17px] leading-[1.5] text-graphite">
          Create an API key in the dashboard and send it in the <code className="font-mono text-sm text-ink">X-API-Key</code> header. Each key is shown once, stored only as a hash, and can be revoked without touching your other keys.
        </p>
        <Link to="/app/docs" className={buttonClass({ variant: "outline" }, "mt-2")}>
          Read the API reference
        </Link>
      </div>
      <div className="min-w-0 flex-1 overflow-x-auto border border-ink bg-paper p-6">
        <pre className="font-mono text-xs leading-[1.75] whitespace-pre text-ink">
          <code>
            <span className={c}># Shorten a link with an API key</span>
            {"\n"}curl -X POST https://linkforge.deaprima.my.id/api/v1/links \{"\n"}
            {"  "}-H <span className={s}>"X-API-Key: lf_sk_a3f9••••••••••••"</span> \{"\n"}
            {"  "}-H <span className={s}>"Content-Type: application/json"</span> \{"\n"}
            {"  "}-d <span className={s}>{`'{"original_url": "https://docs.google.com/…", "alias": "pitch"}'`}</span>
            {"\n\n"}
            <span className={c}># 201 Created</span>
            {"\n"}
            {`{
  "success": true,
  "data": {
    "alias": "pitch",
    "short_url": "https://linkforge.deaprima.my.id/pitch",
    "has_password": false,
    "expired_at": null
  }
}`}
          </code>
        </pre>
      </div>
    </section>
  );
}

// Call to action (Figma 13:276)

export function Cta() {
  return (
    <section aria-labelledby="cta-title" className="px-gutter py-12">
      <div className="flex items-center justify-between gap-6 border border-ink bg-paper px-10 py-8">
      <div className="flex items-center gap-7">
        <Kliko pose="success" className="w-[150px]" />
        <div className="flex flex-col gap-2.5">
          <h2 id="cta-title" className="text-[40px] leading-[1.05] font-semibold tracking-[-1px] text-ink">
            Make your first short link
          </h2>
          <p className="text-base text-graphite">Sign up with email or Google. No card, no trial period.</p>
        </div>
      </div>
      <div className="flex gap-3">
        <Link to="/app/register" className={buttonClass({})}>
          Create free account
        </Link>
        <Link to="/app/login" className={buttonClass({ variant: "outline" })}>
          Sign in
        </Link>
      </div>
      </div>
    </section>
  );
}
