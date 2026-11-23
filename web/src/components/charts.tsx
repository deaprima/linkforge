import { cn } from "cn";
import { formatNumber } from "@/lib/format";

/*
 * Grafik dari Figma "Link detail": dirender dari data (bukan gambar statis).
 *   DailyChart  — tinggi 150, garis bantu grid di atas & tengah, baseline ink,
 *                 batang ink dengan celah 20%, batang tertinggi signal.
 *   BreakdownList — label 64px + bar 6px ink + angka graphite.
 */

const shortDate = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", timeZone: "UTC" });

export function DailyChart({ days }: { days: { date: string; clicks: number }[] }) {
  const max = Math.max(0, ...days.map((d) => d.clicks));
  const maxIndex = max > 0 ? days.findIndex((d) => d.clicks === max) : -1;
  // 5 label sumbu tersebar rata (awal, 3 tengah, akhir) seperti di Figma; di mobile hanya awal, tengah, akhir
  const fractions = days.length <= 1 ? [0] : [0, 0.25, 0.5, 0.75, 1];
  const labels = fractions
    .map((f) => ({ i: Math.round(f * (days.length - 1)), quarter: f === 0.25 || f === 0.75 }))
    .filter((l, k, all) => all.findIndex((o) => o.i === l.i) === k);

  return (
    <figure className="flex w-full flex-col gap-5">
      <div className="relative h-[150px] w-full border-b border-ink" role="img" aria-label={`Daily clicks, peak ${formatNumber(max)}`}>
        <span aria-hidden className="absolute inset-x-0 top-0 h-px bg-grid" />
        <span aria-hidden className="absolute inset-x-0 top-1/2 h-px bg-grid" />
        <div className="absolute inset-0 flex items-end">
          {days.map((d, i) => (
            <div key={d.date} className="group flex h-full flex-1 items-end justify-center" title={`${shortDate.format(new Date(d.date))}: ${formatNumber(d.clicks)} clicks`}>
              <div
                className={cn("w-4/5", i === maxIndex ? "bg-signal" : "bg-ink", "group-hover:opacity-80")}
                style={{ height: max ? `${(d.clicks / max) * 100}%` : 0 }}
              />
            </div>
          ))}
        </div>
      </div>
      <figcaption className="flex justify-between text-xs text-graphite">
        {labels.map(({ i, quarter }) => (
          <span key={i} className={cn(quarter && "hidden sm:inline")}>
            {days[i] ? shortDate.format(new Date(days[i].date)) : ""}
          </span>
        ))}
      </figcaption>
    </figure>
  );
}

export function BreakdownList({ title, items, mono }: { title: string; items: { label: string; count: number }[]; mono?: boolean }) {
  const max = Math.max(0, ...items.map((i) => i.count));
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-2.5">
      <h3 className="text-[13px] font-semibold text-ink">{title}</h3>
      {items.length === 0 ? (
        <p className="text-[13px] text-graphite">No data yet</p>
      ) : (
        <ul className="flex flex-col gap-2.5">
          {items.map((it) => (
            <li key={it.label} className="flex items-center gap-2.5">
              <span className={cn("w-16 shrink-0 truncate text-[13px] text-ink", mono && "font-mono")}>{it.label}</span>
              <span className="flex min-w-0 flex-1" aria-hidden>
                <span className="h-1.5 min-w-1.5 bg-ink" style={{ width: `${max ? (it.count / max) * 100 : 0}%` }} />
              </span>
              <span className="w-8 shrink-0 text-right text-[13px] text-graphite">{formatNumber(it.count)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** Segmented control (Figma QR panel "Segmented"): pilihan aktif = blok ink. */
export function Segmented<T extends string | number>({
  label,
  options,
  value,
  onChange,
  mono,
}: {
  label: string;
  options: { value: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
  mono?: boolean;
}) {
  return (
    <div role="radiogroup" aria-label={label} className="flex w-full border border-ink">
      {options.map((o, i) => {
        const active = o.value === value;
        return (
          <button
            key={String(o.value)}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(o.value)}
            className={cn(
              "flex flex-1 cursor-pointer items-center justify-center px-3.5 py-2 text-[13px]",
              i < options.length - 1 && "border-r border-ink",
              mono && "font-mono",
              active ? "bg-ink font-semibold text-paper" : "bg-paper text-ink hover:bg-grid",
              "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-signal",
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
