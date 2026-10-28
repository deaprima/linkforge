import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { AuthLayout, FormError } from "@/components/layout/AuthLayout";
import { Button, Divider, Field } from "@/components/ui/primitives";
import { API_PREFIX, ApiError } from "@/lib/api";
import { safeNext, useLogin, useRegister } from "@/lib/auth";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

// Alasan dari Gateway saat OAuth gagal (redirect ke /app/login?error=<reason>)
const OAUTH_ERRORS: Record<string, string> = {
  access_denied: "Google sign-in was cancelled.",
  invalid_state: "Google sign-in expired or was tampered with. Please try again.",
  exchange_failed: "Google sign-in failed. Please try again.",
  auth_failed: "We couldn't sign you in with that Google account. Make sure its email is verified.",
};

function GoogleButton() {
  // Navigasi penuh: Gateway me-redirect ke Google, lalu kembali dengan cookie sesi
  return (
    <Button type="button" variant="outline" className="w-full" onClick={() => window.location.assign(`${API_PREFIX}/auth/oauth/google`)}>
      Continue with Google
    </Button>
  );
}

function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.code === "SERVICE_UNAVAILABLE" || err.code === "NETWORK_ERROR") return "LinkForge is temporarily unavailable. Please try again in a moment.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

export function SignInPage() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const login = useLogin();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<{ email?: string; password?: string }>({});

  const oauthError = params.get("error");
  const next = safeNext(params.get("next"));

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const errors: typeof fieldErrors = {};
    if (!EMAIL_RE.test(email)) errors.email = "Enter a valid email address.";
    if (!password) errors.password = "Enter your password.";
    setFieldErrors(errors);
    if (Object.keys(errors).length) return;
    login.mutate({ email, password }, { onSuccess: () => navigate(next, { replace: true }) });
  }

  const serverError =
    login.error instanceof ApiError && login.error.code === "UNAUTHORIZED" ? "Email or password is incorrect." : login.error ? errorMessage(login.error) : null;

  return (
    <AuthLayout
      pose="hero"
      title="Sign in"
      subtitle="Welcome back. Your links are where you left them."
      footer={
        <>
          <span className="text-graphite">No account yet?</span>
          <Link to={`/app/register${params.get("next") ? `?next=${encodeURIComponent(next)}` : ""}`} className="font-semibold text-ink underline">
            Create one
          </Link>
        </>
      }
    >
      <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
        {oauthError && <FormError>{OAUTH_ERRORS[oauthError] ?? "Google sign-in failed. Please try again."}</FormError>}
        {serverError && <FormError>{serverError}</FormError>}
        <Field label="Email" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} error={fieldErrors.email} />
        <Field
          label="Password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          error={fieldErrors.password}
        />
        <Link to="/app/forgot-password" className="-mt-2 self-end text-[13px] font-semibold text-ink underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-signal">
          Forgot password?
        </Link>
        <Button type="submit" className="w-full" loading={login.isPending}>
          Sign in
        </Button>
      </form>
      <Divider label="or" />
      <GoogleButton />
    </AuthLayout>
  );
}

export function RegisterPage() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const register = useRegister();
  const [form, setForm] = useState({ name: "", email: "", password: "" });
  const [fieldErrors, setFieldErrors] = useState<{ name?: string; email?: string; password?: string }>({});
  const next = safeNext(params.get("next"));

  const set = (key: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => setForm((f) => ({ ...f, [key]: e.target.value }));

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const errors: typeof fieldErrors = {};
    if (!form.name.trim()) errors.name = "Enter your name.";
    if (!EMAIL_RE.test(form.email)) errors.email = "Enter a valid email address.";
    if (form.password.length < 8) errors.password = "At least 8 characters.";
    else if (form.password.length > 72) errors.password = "At most 72 characters.";
    setFieldErrors(errors);
    if (Object.keys(errors).length) return;
    register.mutate({ ...form, name: form.name.trim() }, { onSuccess: () => navigate(next, { replace: true }) });
  }

  const emailTaken = register.error instanceof ApiError && register.error.code === "EMAIL_ALREADY_EXISTS";
  const serverError = register.error && !emailTaken ? errorMessage(register.error) : null;

  return (
    <AuthLayout
      pose="success"
      title="Create your account"
      subtitle="Free, no card needed. You can shorten links right away."
      footer={
        <>
          <span className="text-graphite">Already have an account?</span>
          <Link to="/app/login" className="font-semibold text-ink underline">
            Sign in
          </Link>
        </>
      }
    >
      <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
        {serverError && <FormError>{serverError}</FormError>}
        <Field label="Name" autoComplete="name" value={form.name} onChange={set("name")} error={fieldErrors.name} />
        <Field
          label="Email"
          type="email"
          autoComplete="email"
          value={form.email}
          onChange={set("email")}
          error={fieldErrors.email ?? (emailTaken ? "An account with this email already exists." : undefined)}
        />
        <Field
          label="Password"
          type="password"
          autoComplete="new-password"
          value={form.password}
          onChange={set("password")}
          helper="At least 8 characters."
          error={fieldErrors.password}
        />
        <Button type="submit" className="w-full" loading={register.isPending}>
          Create account
        </Button>
      </form>
      <Divider label="or" />
      <GoogleButton />
    </AuthLayout>
  );
}
