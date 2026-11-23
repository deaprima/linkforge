import { cn } from "cn";
import { forwardRef, useId, type AnchorHTMLAttributes, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode } from "react";

/*
 * Primitive UI LinkForge — diterjemahkan dari Figma Components:
 *   Button (12:10)  Solid = CTA utama (satu per view), Outline = sekunder; MD 44px / SM 36px.
 *   Input  (15:290) label + field 44px + helper/error, opsional prefix monospace (untuk alias).
 *   Chip   (12:17)  nilai monospace (alias, key, tanggal, status): default / signal / stop.
 * Semua bersudut tajam dengan border ink, tanpa shadow.
 */

type ButtonStyle = { variant?: "solid" | "outline"; size?: "md" | "sm" };

export function buttonClass({ variant = "solid", size = "md" }: ButtonStyle, className?: string) {
  return cn(
    "inline-flex cursor-pointer items-center justify-center gap-2 border-[1.5px] border-ink font-semibold whitespace-nowrap",
    "transition-opacity disabled:cursor-not-allowed disabled:opacity-50",
    "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-signal",
    size === "md" ? "h-11 px-5 text-[15px]" : "h-9 px-3.5 text-sm",
    variant === "solid" ? "bg-ink text-paper hover:opacity-90" : "bg-paper text-ink hover:bg-grid",
    className,
  );
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & ButtonStyle & { loading?: boolean };

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant, size, loading, disabled, className, children, ...props },
  ref,
) {
  return (
    <button ref={ref} disabled={disabled || loading} aria-busy={loading || undefined} className={buttonClass({ variant, size }, className)} {...props}>
      {loading ? "…" : children}
    </button>
  );
});

/** Tautan bergaya Button (mis. unduhan file). */
export function ButtonLink({ variant, size, className, ...props }: AnchorHTMLAttributes<HTMLAnchorElement> & ButtonStyle) {
  return <a className={buttonClass({ variant, size }, className)} {...props} />;
}

type FieldProps = InputHTMLAttributes<HTMLInputElement> & {
  label: string;
  helper?: ReactNode;
  error?: string | null;
  prefix?: string;
};

export const Field = forwardRef<HTMLInputElement, FieldProps>(function Field(
  { label, helper, error, prefix, id, className, ...props },
  ref,
) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const describedBy = error || helper ? `${inputId}-desc` : undefined;
  return (
    <div className={cn("flex w-full flex-col gap-1.5", className)}>
      <label htmlFor={inputId} className="text-sm font-semibold text-ink">
        {label}
      </label>
      <div
        className={cn(
          "flex h-11 w-full items-center overflow-clip bg-paper px-3",
          error ? "border-[1.5px] border-stop" : "border border-ink",
          "focus-within:outline-2 focus-within:outline-offset-1 focus-within:outline-signal",
        )}
      >
        {prefix && <span className="shrink-0 font-mono text-xs text-graphite">{prefix}</span>}
        <input
          ref={ref}
          id={inputId}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          className="min-w-0 flex-1 bg-transparent text-[15px] text-ink outline-none placeholder:text-graphite"
          {...props}
        />
      </div>
      {(error || helper) && (
        <p id={describedBy} className={cn("text-[13px] leading-[1.4]", error ? "text-stop" : "text-graphite")}>
          {error || helper}
        </p>
      )}
    </div>
  );
});

export function Chip({ tone = "default", className, children }: { tone?: "default" | "signal" | "stop"; className?: string; children: ReactNode }) {
  return (
    <span
      className={cn(
        "inline-flex items-center border px-2.5 py-1.5 font-mono text-xs whitespace-nowrap",
        tone === "signal" && "border-signal bg-signal-soft text-signal-ink",
        tone === "stop" && "border-stop bg-paper text-stop",
        tone === "default" && "border-ink bg-paper text-ink",
        className,
      )}
    >
      {children}
    </span>
  );
}

/** Pemisah "or" (Figma Sign in → Divider): garis rule + label graphite 13px. */
export function Divider({ label }: { label: string }) {
  return (
    <div className="flex w-full items-center gap-3">
      <span className="h-px flex-1 bg-rule" />
      <span className="text-[13px] text-graphite">{label}</span>
      <span className="h-px flex-1 bg-rule" />
    </div>
  );
}
