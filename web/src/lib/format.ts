// Format tampilan mengikuti contoh di Figma: "1,280", "31 Dec 2026", "12 s ago".

const numberFmt = new Intl.NumberFormat("en-US");
const dateFmt = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", year: "numeric" });

export const formatNumber = (n: number) => numberFmt.format(n);
export const formatDate = (iso: string) => dateFmt.format(new Date(iso));

export function timeAgo(date: Date, now = Date.now()): string {
  const s = Math.max(0, Math.round((now - date.getTime()) / 1000));
  if (s < 5) return "just now";
  if (s < 60) return `${s} s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} min ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} h ago`;
  return formatDate(date.toISOString());
}

/** Destination tanpa skema & trailing slash, seperti di tabel Figma ("github.com/deaprima/linkforge"). */
export function displayUrl(url: string): string {
  return url.replace(/^https?:\/\//, "").replace(/\/$/, "");
}

export function isExpired(expiredAt: string | null, now = Date.now()): boolean {
  return expiredAt !== null && new Date(expiredAt).getTime() <= now;
}

const deviceLabel: Record<string, string> = { mobile: "Mobile", desktop: "Desktop", tablet: "Tablet", bot: "Bot" };
export const formatDevice = (d: string) => deviceLabel[d] ?? d;

/**
 * Origin short link, mis. "https://linkforge.deaprima.my.id". Di produksi short link & web app satu
 * domain; saat dev short link hidup di Gateway (VITE_SHORT_URL_BASE, lihat .env.development).
 */
export function shortLinkBase(): string {
  const base = import.meta.env.VITE_SHORT_URL_BASE as string | undefined;
  return (base ?? window.location.origin).replace(/\/$/, "");
}
