import { useEffect, type ReactNode } from "react";
import { Link } from "react-router";
import { GridBackground } from "@/components/brand/Brand";
import { TopBar } from "@/components/layout/AuthLayout";
import { buttonClass } from "@/components/ui/primitives";
import { REPO_URL } from "@/pages/landing/links";

/*
 * Ketentuan & disclaimer (/app/terms). Halaman publik: dibaca sebelum punya akun, jadi ada di luar guard auth.
 * Isinya sengaja singkat dan jujur soal apa yang dikumpulkan; ubah teks ini setiap kali perilaku data berubah.
 */

const UPDATED = "29 October 2026";

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-2.5 border-t border-ink pt-6">
      <h2 className="text-xl font-semibold tracking-[-0.2px] text-ink">{title}</h2>
      <div className="flex flex-col gap-3 text-[15px] leading-[1.6] text-graphite">{children}</div>
    </section>
  );
}

export function TermsPage() {
  useEffect(() => {
    document.title = "Terms & disclaimer · LinkForge";
  }, []);
  return (
    <div className="flex min-h-dvh flex-col">
      <GridBackground />
      <TopBar>
        <Link to="/" className={buttonClass({ variant: "outline", size: "sm" }, "hidden sm:inline-flex")}>
          Home
        </Link>
      </TopBar>
      <main className="mx-auto flex w-full max-w-[720px] flex-1 flex-col gap-8 px-4 py-10 sm:py-14">
        <header className="flex flex-col gap-2">
          <p className="font-mono text-sm text-graphite">Last updated {UPDATED}</p>
          <h1 className="text-[32px] leading-[1.1] font-semibold tracking-[-0.64px] text-ink sm:text-4xl">Terms &amp; disclaimer</h1>
          <p className="text-base leading-[1.5] text-graphite">The short version: this is a portfolio project. It works, but treat it like one.</p>
        </header>

        <Section title="This is a portfolio project">
          <p>
            LinkForge is a public portfolio project by Dea Prima, built to show backend engineering: microservices, event streaming and
            observability. It is a real, running service, but it is not a commercial product and nobody is paid to keep it up.
          </p>
        </Section>

        <Section title="No guarantees">
          <p>
            The service is provided <strong className="font-semibold text-ink">as is</strong>. There is no uptime promise or support commitment.
            It can change, lose data, be reset or be shut down at any time, without notice. Short links may stop working.
          </p>
          <p>Do not use LinkForge for anything important, such as printed material, paid campaigns or anything people depend on.</p>
        </Section>

        <Section title="Acceptable use">
          <p>
            Do not shorten links to malware, phishing, illegal content or spam, and do not use the service to hide where a link really goes
            to harm someone. Links to blocked domains are rejected, requests are rate limited, and links or accounts that abuse the service
            can be disabled or removed without warning.
          </p>
        </Section>

        <Section title="What is stored">
          <ul className="flex list-disc flex-col gap-1.5 pl-5">
            <li>
              <strong className="font-semibold text-ink">Your account:</strong> name, email address, and a hash of your password (the password
              itself is never stored). API keys are stored as hashes too.
            </li>
            <li>
              <strong className="font-semibold text-ink">Your links:</strong> the destination URL, alias, optional title and expiry date. A link
              password, if you set one, is stored as a hash.
            </li>
            <li>
              <strong className="font-semibold text-ink">Clicks:</strong> every time a short link is opened, LinkForge records the time, the
              visitor&apos;s IP address, browser, operating system and device type (from the User-Agent), the referring site, and a country
              derived from the IP address. The link&apos;s owner sees this as analytics.
            </li>
          </ul>
          <p>
            Cookies are used only to keep you signed in (HttpOnly session cookies). There are no advertising or tracking cookies, and no data
            is sold or shared with third parties. Click records are not deleted automatically.
          </p>
        </Section>

        <Section title="Contact and removal requests">
          <p>
            Questions, abuse reports, or a request to remove your account or a link:{" "}
            <a href={`${REPO_URL}/issues`} className="font-semibold text-ink underline">
              open an issue on GitHub
            </a>
            . The source code is public at{" "}
            <a href={REPO_URL} className="font-semibold text-ink underline">
              github.com/deaprima/linkforge
            </a>
            .
          </p>
        </Section>
      </main>
    </div>
  );
}
