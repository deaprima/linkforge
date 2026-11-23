import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { AuthLayout, FormError } from "@/components/layout/AuthLayout";
import { Button, buttonClass, Field } from "@/components/ui/primitives";
import { ApiError } from "@/lib/api";
import { useForgotPassword, useResetPassword } from "@/lib/auth";

/*
 * Lupa password: dua halaman di layout auth yang sama (panel 440px + Kliko). Tidak ada desain Figma khusus,
 * jadi disusun dari komponen yang sudah ada: Field, Button, FormError.
 *   /app/forgot-password  minta link lewat email
 *   /app/reset-password?token=...  setel password baru (link dari email)
 */

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const minutes = (seconds: number) => Math.max(1, Math.ceil(seconds / 60));
const minutesText = (m: number) => (m === 1 ? "a minute" : `${m} minutes`);

/** Pesan untuk error server; 429 menyebut berapa lama harus menunggu (Retry-After). */
function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 429) return `Too many attempts. Try again in ${minutesText(minutes(err.retryAfter ?? 60))}.`;
    if (err.code === "SERVICE_UNAVAILABLE" || err.code === "NETWORK_ERROR") return "LinkForge is temporarily unavailable. Please try again in a moment.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

function useTitle(title: string) {
  useEffect(() => {
    document.title = `${title} · LinkForge`;
  }, [title]);
}

const signInFooter = (
  <>
    <span className="text-graphite">Remembered it?</span>
    <Link to="/app/login" className="font-semibold text-ink underline">
      Sign in
    </Link>
  </>
);

function Note({ children }: { children: ReactNode }) {
  return <p className="text-sm leading-[1.5] text-graphite">{children}</p>;
}

export function ForgotPasswordPage() {
  useTitle("Forgot password");
  const forgot = useForgotPassword();
  const [email, setEmail] = useState("");
  const [fieldError, setFieldError] = useState<string | null>(null);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const value = email.trim();
    if (!EMAIL_RE.test(value)) return setFieldError("Enter a valid email address.");
    setFieldError(null);
    forgot.mutate({ email: value });
  }

  if (forgot.isSuccess) {
    return (
      <AuthLayout pose="success" title="Check your email" subtitle="If that address has an account, a reset link is on its way." footer={signInFooter}>
        <Note>
          We sent it to <span className="font-semibold break-all text-ink">{email.trim()}</span>. The link works once and expires in 1 hour.
        </Note>
        <Note>Nothing there? Check your spam folder, or try again in a minute.</Note>
        <Button type="button" variant="outline" className="w-full" onClick={() => forgot.reset()}>
          Use a different email
        </Button>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout pose="lock" title="Forgot your password?" subtitle="Enter your email and we'll send you a link to choose a new one." footer={signInFooter}>
      <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
        {forgot.error && <FormError>{errorMessage(forgot.error)}</FormError>}
        <Field label="Email" type="email" autoComplete="email" autoFocus value={email} onChange={(e) => setEmail(e.target.value)} error={fieldError} />
        <Button type="submit" className="w-full" loading={forgot.isPending}>
          Send reset link
        </Button>
      </form>
    </AuthLayout>
  );
}

export function ResetPasswordPage() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const token = params.get("token") ?? "";
  const reset = useResetPassword();
  const [form, setForm] = useState({ password: "", confirm: "" });
  const [errors, setErrors] = useState<{ password?: string; confirm?: string }>({});
  useTitle("Reset password");

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const next: typeof errors = {};
    if (form.password.length < 8) next.password = "At least 8 characters.";
    else if (form.password.length > 72) next.password = "At most 72 characters.";
    if (form.confirm !== form.password) next.confirm = "The two passwords don't match.";
    setErrors(next);
    if (Object.keys(next).length) return;
    reset.mutate({ token, password: form.password });
  }

  if (reset.isSuccess) {
    return (
      <AuthLayout pose="success" title="Password changed" subtitle="Sign in with your new password.">
        <Note>For your security you've been signed out everywhere else.</Note>
        <Button type="button" className="w-full" onClick={() => navigate("/app/login", { replace: true })}>
          Sign in
        </Button>
      </AuthLayout>
    );
  }

  const badToken = !token || (reset.error instanceof ApiError && reset.error.code === "INVALID_RESET_TOKEN");
  if (badToken) {
    return (
      <AuthLayout
        pose="lost"
        title={token ? "This link has expired" : "This link is incomplete"}
        subtitle={token ? "Reset links work once and expire after an hour." : "Open the link from your email again, or ask for a new one."}
        footer={signInFooter}
      >
        <Link to="/app/forgot-password" className={buttonClass({}, "w-full")}>
          Request a new link
        </Link>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout pose="lock" title="Choose a new password" subtitle="Use at least 8 characters." footer={signInFooter}>
      <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
        {reset.error && <FormError>{errorMessage(reset.error)}</FormError>}
        <Field
          label="New password"
          type="password"
          autoComplete="new-password"
          autoFocus
          value={form.password}
          onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
          error={errors.password}
        />
        <Field
          label="Confirm new password"
          type="password"
          autoComplete="new-password"
          value={form.confirm}
          onChange={(e) => setForm((f) => ({ ...f, confirm: e.target.value }))}
          error={errors.confirm}
        />
        <Button type="submit" className="w-full" loading={reset.isPending}>
          Change password
        </Button>
      </form>
    </AuthLayout>
  );
}
