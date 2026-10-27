import { useMutation } from "@tanstack/react-query";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { GridBackground } from "@/components/brand/Brand";
import { ErrorMascot, type ErrorMascotKind } from "@/components/brand/ErrorMascot";
import { LockIconLarge } from "@/components/brand/Icons";
import { FormError, TopBar } from "@/components/layout/AuthLayout";
import { Button, ButtonLink, Field } from "@/components/ui/primitives";
import { ApiError, request } from "@/lib/api";
import { shortLinkBase } from "@/lib/format";

/*
 * Halaman publik yang dituju pengunjung short link (Gateway me-redirect browser ke sini,
 * lihat API-CONTRACT.md "Redirect (Publik)"): password gate, 404, 410, 503.
 * Pengunjung biasanya tidak punya akun, jadi top bar menawarkan "Shorten your own links".
 */

function PublicLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-dvh flex-col">
      <GridBackground />
      <TopBar>
        <ButtonLink href="/app/register" variant="outline" size="sm" className="hidden sm:inline-flex">
          Shorten your own links
        </ButtonLink>
      </TopBar>
      <main className="flex flex-1 flex-col items-center justify-center px-4 py-10">{children}</main>
    </div>
  );
}

/** Pengunjung tidak bisa salah ketik alias sepanjang ini; batasi supaya teks tidak meledakkan layout. */
function useAliasParam(): string | null {
  const [params] = useSearchParams();
  const alias = params.get("alias")?.trim();
  return alias ? alias.slice(0, 64) : null;
}

/** Layar error (Figma 404 19:680, 410 19:752, 503 19:826): maskot 220px + kode + judul + teks + satu CTA. */
function ErrorScreen({ mascot, code, title, children, action }: { mascot: ErrorMascotKind; code: string; title: string; children: ReactNode; action: ReactNode }) {
  useEffect(() => {
    document.title = `${title} · LinkForge`;
  }, [title]);
  return (
    <PublicLayout>
      <div className="flex w-full max-w-[520px] flex-col items-center gap-4 text-center">
        <ErrorMascot kind={mascot} className="size-[180px] sm:size-[220px]" />
        <p className="font-mono text-[13px] text-graphite">{code}</p>
        <h1 className="text-[30px] leading-[1.1] font-semibold tracking-[-0.72px] text-ink sm:text-4xl">{title}</h1>
        <p className="text-base leading-[1.5] text-graphite">{children}</p>
        <div className="h-2" aria-hidden />
        {action}
      </div>
    </PublicLayout>
  );
}

const Alias = ({ alias }: { alias: string }) => <span className="break-words">/{alias}</span>;

export function NotFoundPage() {
  const alias = useAliasParam();
  return (
    <ErrorScreen
      mascot="lost"
      code="404"
      title="This link doesn’t exist"
      action={<ButtonLink href="/">Go to LinkForge</ButtonLink>}
    >
      {alias ? (
        <>
          There’s no LinkForge link at <Alias alias={alias} />. Check the address for typos, or ask the sender for a new link.
        </>
      ) : (
        <>There’s nothing at this address. Check it for typos, or ask the sender for a new link.</>
      )}
    </ErrorScreen>
  );
}

// Catatan: Figma menyebut tanggal expiry ("…on 27 Oct 2026"), tapi redirect Gateway hanya membawa
// alias — endpoint publik tidak membuka expired_at. Tanggal bisa ditambah kalau kontraknya diperluas.
export function ExpiredPage() {
  const alias = useAliasParam();
  return (
    <ErrorScreen
      mascot="expired"
      code="410"
      title="This link has expired"
      action={<ButtonLink href="/">Go to LinkForge</ButtonLink>}
    >
      {alias ? (
        <>
          The owner set <Alias alias={alias} /> to stop working. Ask them for an updated link.
        </>
      ) : (
        <>The owner set this link to stop working. Ask them for an updated link.</>
      )}
    </ErrorScreen>
  );
}

export function UnavailablePage() {
  const alias = useAliasParam();
  // Coba lagi = buka ulang short link-nya; Gateway mengembalikan ke sini kalau masih down.
  const retry = () => (alias ? window.location.assign(`${shortLinkBase()}/${encodeURIComponent(alias)}`) : window.location.reload());
  return (
    <ErrorScreen mascot="fuse" code="503" title="We can’t open this link right now" action={<Button onClick={retry}>Try again</Button>}>
      Part of LinkForge isn’t responding. Your link is safe; try again in a minute.
    </ErrorScreen>
  );
}

const toMinutes = (seconds: number) => Math.max(1, Math.ceil(seconds / 60));
const minutesText = (m: number) => (m === 1 ? "a minute" : `${m} minutes`);

/**
 * Figma: "That password is wrong. 4 attempts left this minute." — window limiter Gateway 15 menit,
 * jadi sisa waktunya diambil dari X-RateLimit-Reset.
 */
function wrongPasswordMessage(err: ApiError): string {
  const rl = err.rateLimit;
  if (!rl) return "That password is wrong.";
  const m = toMinutes((rl.reset.getTime() - Date.now()) / 1000);
  if (rl.remaining <= 0) return `That password is wrong. No attempts left; try again in ${minutesText(m)}.`;
  const attempts = `${rl.remaining} ${rl.remaining === 1 ? "attempt" : "attempts"}`;
  return `That password is wrong. ${attempts} left ${m === 1 ? "this minute" : `in the next ${m} minutes`}.`;
}

const tooManyMessage = (err: ApiError) => `Too many attempts. Try again in ${minutesText(toMinutes(err.retryAfter ?? 60))}.`;

/** Figma "Password gate — Desktop" (19:651). */
export function PasswordGatePage() {
  const { alias = "" } = useParams();
  const navigate = useNavigate();
  const [password, setPassword] = useState("");
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const host = shortLinkBase().replace(/^https?:\/\//, "");

  useEffect(() => {
    document.title = "Password required · LinkForge";
  }, []);

  const verify = useMutation({
    mutationFn: (pw: string) =>
      request<{ redirect_url: string }>(`/${encodeURIComponent(alias)}/verify`, { method: "POST", body: { password: pw }, prefix: "", noRefresh: true }),
    // Biarkan tombol tetap loading sampai browser pindah halaman
    onSuccess: ({ redirect_url }) => window.location.assign(redirect_url),
    onError: (err) => {
      if (!(err instanceof ApiError)) return setFormError("Something went wrong. Please try again.");
      const q = `?alias=${encodeURIComponent(alias)}`;
      switch (err.status) {
        case 401:
          return setFieldError(wrongPasswordMessage(err));
        case 429:
          return setFieldError(tooManyMessage(err));
        case 404:
          return navigate(`/app/not-found${q}`, { replace: true });
        case 410:
          return navigate(`/app/expired${q}`, { replace: true });
        case 503:
          return navigate(`/app/unavailable${q}`, { replace: true });
        default:
          return setFormError(err.code === "NETWORK_ERROR" ? err.message : "Something went wrong. Please try again.");
      }
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!password) return setFieldError("Enter the password.");
    setFieldError(null);
    setFormError(null);
    verify.mutate(password);
  }

  return (
    <PublicLayout>
      <section className="flex w-full max-w-[440px] flex-col gap-4 border border-ink bg-paper p-6 sm:p-8">
        <LockIconLarge />
        <h1 className="text-2xl font-semibold tracking-[-0.36px] text-ink">This link is password protected</h1>
        <p className="text-sm leading-[1.5] text-graphite">
          Enter the password you were given to open{" "}
          <span className="font-mono text-xs break-words text-ink">
            {host}/{alias}
          </span>
          .
        </p>
        <form onSubmit={onSubmit} noValidate className="flex flex-col gap-4">
          {formError && <FormError>{formError}</FormError>}
          <Field
            label="Password"
            type="password"
            autoComplete="off"
            autoFocus
            value={password}
            onChange={(e) => {
              setPassword(e.target.value);
              // Pesan 429 tetap tampil: mengetik ulang tidak membuka blokirnya
              if (!(verify.error instanceof ApiError && verify.error.status === 429)) setFieldError(null);
            }}
            error={fieldError}
          />
          <Button type="submit" className="w-full" loading={verify.isPending || verify.isSuccess}>
            Open link
          </Button>
        </form>
      </section>
    </PublicLayout>
  );
}
