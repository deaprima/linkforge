import type { ReactNode } from "react";
import { Link } from "react-router";

/**
 * Origin web app (mis. https://linkforge.deaprima.my.id) saat landing di-host terpisah, misalnya di Vercel.
 * Kosong = landing satu origin dengan app (Caddy / dev), jadi link /app/* tetap navigasi SPA biasa.
 */
export const APP_ORIGIN = (import.meta.env.VITE_APP_ORIGIN ?? "").replace(/\/+$/, "");

/** Link ke halaman /app/*: navigasi SPA kalau satu origin, link absolut ke APP_ORIGIN kalau terpisah. */
export function AppLink({ to, className, children }: { to: `/app${string}`; className?: string; children: ReactNode }) {
  if (APP_ORIGIN) {
    return (
      <a href={APP_ORIGIN + to} className={className}>
        {children}
      </a>
    );
  }
  return (
    <Link to={to} className={className}>
      {children}
    </Link>
  );
}
