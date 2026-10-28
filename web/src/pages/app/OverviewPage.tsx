import { useQuery } from "@tanstack/react-query";
import { cn } from "cn";
import type { ReactNode } from "react";
import { Link, useNavigate } from "react-router";
import { LiveClicks } from "@/components/LiveClicks";
import { Kliko } from "@/components/brand/Kliko";
import { PageHeader, Panel, PanelMessage } from "@/components/layout/AppLayout";
import { Button } from "@/components/ui/primitives";
import { request, type Summary } from "@/lib/api";
import { formatNumber } from "@/lib/format";

/** Readout (Figma "Readouts"): label graphite + angka. Mobile: dua sel sebaris, "Top link" satu baris penuh di bawahnya. */
function Readout({ label, children, className }: { label: string; children: ReactNode; className?: string }) {
  return (
    <div className={cn("flex min-w-0 flex-col gap-1 px-4 py-3.5 sm:gap-1.5 sm:px-5 sm:py-4.5", className)}>
      <p className="text-xs text-graphite sm:text-[13px]">{label}</p>
      {children}
    </div>
  );
}

const readoutValue = "text-[28px] font-semibold tracking-[-0.56px] text-ink sm:text-[34px] sm:tracking-[-0.68px]";

function TopLinks({ summary }: { summary: Summary }) {
  const max = summary.top_links[0]?.clicks ?? 0;
  if (summary.top_links.length === 0) {
    return (
      <div className="flex flex-col items-center px-5 pt-6">
        <Kliko pose="chart" className="w-[160px]" />
        <PanelMessage>No clicks yet. Share a short link and it will show up here.</PanelMessage>
      </div>
    );
  }
  return (
    <ol>
      {summary.top_links.slice(0, 5).map((l, i) => (
        <li key={l.link_id} className={cn(i > 0 && "border-t border-ink")}>
          <Link
            to={`/app/links/${l.link_id}`}
            className="flex flex-col gap-2 px-4 py-3 hover:bg-grid focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-signal sm:flex-row sm:items-center sm:gap-4 sm:px-5 sm:py-3.5"
          >
            {/* Mobile: alias + jumlah di satu baris, bar di bawahnya (Figma 20:324) */}
            <span className="hidden text-[13px] text-graphite sm:inline">{i + 1}</span>
            <span className="flex items-start justify-between sm:contents">
              <span className="truncate font-mono text-xs text-ink sm:w-[130px] sm:shrink-0 sm:text-[13px]">/{l.alias || "—"}</span>
              <span className="text-sm font-semibold text-ink sm:order-last sm:w-12 sm:shrink-0 sm:text-right">{formatNumber(l.clicks)}</span>
            </span>
            <span className="flex min-w-0 sm:flex-1" aria-hidden>
              {/* Bar teratas signal, sisanya ink (Figma) */}
              <span className={cn("h-1.5 sm:h-2", i === 0 ? "bg-signal" : "bg-ink")} style={{ width: `${max ? (l.clicks / max) * 100 : 0}%` }} />
            </span>
          </Link>
        </li>
      ))}
    </ol>
  );
}

export function OverviewPage() {
  const navigate = useNavigate();
  const summary = useQuery({ queryKey: ["analytics", "summary"], queryFn: () => request<Summary>("/analytics/summary") });
  const top = summary.data?.top_links[0];

  return (
    <>
      <PageHeader
        title="Overview"
        subtitle="Your links and clicks across all time."
        mobileSubtitle="hidden"
        actions={<Button onClick={() => navigate("/app/links/new")}>New link</Button>}
      />

      <section aria-label="Totals" className="grid grid-cols-2 border border-ink bg-paper sm:grid-cols-3">
        <Readout label="Links" className="border-r border-ink">
          <p className={readoutValue}>{summary.data ? formatNumber(summary.data.total_links) : "—"}</p>
        </Readout>
        <Readout label="Total clicks" className="sm:border-r sm:border-ink">
          <p className={readoutValue}>{summary.data ? formatNumber(summary.data.total_clicks) : "—"}</p>
        </Readout>
        <Readout label="Top link" className="col-span-2 border-t border-ink sm:col-span-1 sm:border-t-0">
          {top ? (
            <div className="flex min-w-0 items-baseline justify-between gap-2.5 sm:justify-start">
              <Link to={`/app/links/${top.link_id}`} className="truncate font-mono text-xl font-medium text-ink hover:underline sm:text-[28px] sm:font-semibold">
                /{top.alias}
              </Link>
              <p className="shrink-0 text-sm text-graphite">{formatNumber(top.clicks)} clicks</p>
            </div>
          ) : (
            <p className={readoutValue}>—</p>
          )}
        </Readout>
      </section>

      <div className="flex flex-1 flex-col gap-4 sm:gap-6 xl:flex-row">
        <Panel title="Top links" aside="All time" className="min-w-0 flex-1">
          {summary.isPending ? (
            <PanelMessage>Loading…</PanelMessage>
          ) : summary.isError ? (
            <PanelMessage tone="error">Couldn't load your stats. {summary.error.message}</PanelMessage>
          ) : (
            <TopLinks summary={summary.data} />
          )}
        </Panel>
        <LiveClicks className="w-full shrink-0 xl:w-[420px]" />
      </div>
    </>
  );
}
