import { Link } from "react-router";
import dimensionDark from "@/assets/landing/dimension-dark.svg";
import dimensionLight from "@/assets/landing/dimension-light.svg";
import { Themed } from "@/components/brand/Icons";
import { MascotHero } from "@/components/brand/MascotHero";
import { buttonClass } from "@/components/ui/primitives";
import { REPO_URL } from "@/pages/landing/links";

/** Keterangan gambar "Fig. N" (Figma: "Fig." semibold ink + teks graphite 13px). */
export function Caption({ fig, children }: { fig: number; children: string }) {
  return (
    <p className="text-[13px] leading-[1.4] whitespace-pre-wrap text-graphite">
      <span className="font-semibold text-ink">Fig. {fig}</span>
      {"  "}
      {children}
    </p>
  );
}

const mono = "absolute font-mono whitespace-nowrap";

/**
 * Fig. 1 (Figma 13:36): URL panjang vs pendek dengan garis dimensi. Garis (gambar) dan label diposisikan
 * absolut di kotak 640×210 persis seperti di Figma, supaya garis & teksnya tetap sejajar.
 */
function Measure() {
  return (
    <div role="img" aria-label="A 78 character Google Slides URL shortened to 30 characters, 48 characters removed" className="relative h-[210px] w-[640px] shrink-0">
      <Themed light={dimensionLight} dark={dimensionDark} className="absolute top-0 left-0 h-[210px] w-[640px]" />
      <p className={`${mono} top-[56px] left-5 text-[10.98px] text-graphite`}>https://docs.google.com/presentation/d/1a9XkLmQ2vRt/edit?usp=sharing&amp;ouid=1042</p>
      <p className={`${mono} top-[144px] left-5 text-[10.98px] text-ink`}>linkforge.deaprima.my.id/pitch</p>
      <p className={`${mono} top-[29px] left-[269.5px] text-[11px] text-ink`}>78 characters</p>
      <p className={`${mono} top-[96px] left-[458px] text-[11px] text-graphite`}>48 characters removed</p>
      <p className={`${mono} top-[177px] left-[85px] text-[11px] text-signal-ink`}>30 characters</p>
    </div>
  );
}

export function Hero() {
  return (
    <section aria-labelledby="hero-title" className="flex flex-col gap-12 px-4 py-24 lg:px-gutter min-[1440px]:flex-row">
      <div className="flex shrink-0 flex-col items-start pt-6 min-[1440px]:w-[560px]">
        <h1 id="hero-title" className="text-[66px] leading-[0.98] font-semibold tracking-[-2.31px] text-ink">
          Shorten a link.
          <br />
          See who clicks it.
        </h1>
        <p className="mt-7 max-w-[460px] text-[19px] leading-[29px] text-graphite">
          LinkForge turns long URLs into short ones you control, with custom names, expiry dates, passwords, QR codes and live click analytics. Free and open source.
        </p>
        <div className="mt-9 flex gap-3">
          <Link to="/app/register" className={buttonClass({})}>
            Create free account
          </Link>
          <a href={REPO_URL} className={buttonClass({ variant: "outline" })}>
            Read the source
          </a>
        </div>
      </div>

      <figure className="relative w-full max-w-[702px] min-w-0 min-[1440px]:flex-1">
        <div className="flex flex-col gap-2 overflow-x-auto border border-ink bg-paper px-6 pt-7 pb-5">
          <Measure />
          <figcaption className="pl-5">
            <Caption fig={1}>Same destination, 48 characters shorter. Lines drawn to scale.</Caption>
          </figcaption>
        </div>
        <MascotHero className="absolute top-[238px] right-[-4px] hidden size-[200px] min-[1440px]:block" />
      </figure>
    </section>
  );
}
