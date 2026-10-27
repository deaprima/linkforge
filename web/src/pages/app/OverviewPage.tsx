import { useQuery } from "@tanstack/react-query";
import { cn } from "cn";
import type { ReactNode } from "react";
import { Link, useNavigate } from "react-router";
import { LiveClicks } from "@/components/LiveClicks";
import { PageHeader, Panel, PanelMessage } from "@/components/layout/AppLayout";
import { Button } from "@/components/ui/primitives";
import { request, type Summary } from "@/lib/api";
import { formatNumber } from "@/lib/format";

/** Readout (Figma "Readouts"): label graphite 13px + angka 34px. */
function Readout({ label, children, last }: { label: string; children: ReactNode; last?: boolean }) {
  return (
    <div className={cn("flex min-w-0 flex-1 flex-col gap-1.5 px-5 py-4.5", !last && "border-b border-ink sm:border-r sm:border-b-0")}>
      <p className="text-[13px] text-graphite">{label}</p>
      <div className="flex min-w-0 items-baseline gap-2.5">{children}</div>
    </div>
  );
}

function TopLinks({ summary }: { summary: Summary }) {
  const max = summary.top_links[0]?.clicks ?? 0;
  if (summary.top_links.length === 0) {
    return <PanelMessage>No clicks yet. Share a short link and it will show up here.</PanelMessage>;
  }
  return (
    <ol>
      {summary.top_links.slice(0, 5).map((l, i) => (
        <li key={l.link_id} className={cn(i > 0 && "border-t border-ink")}>
          <Link
            to={`/app/links/${l.link_id}`}
            className="flex items-center gap-4 px-5 py-3.5 hover:bg-grid focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-signal"
          >
            <span className="text-[13px] text-graphite">{i + 1}</span>
            <span className="w-[130px] shrink-0 truncate font-mono text-[13px] text-ink">/{l.alias || "—"}</span>
            <span className="flex min-w-0 flex-1" aria-hidden>
              {/* Bar teratas signal, sisanya ink (Figma) */}
              <span className={cn("h-2", i === 0 ? "bg-signal" : "bg-ink")} style={{ width: `${max ? (l.clicks / max) * 100 : 0}%` }} />
            </span>
            <span className="w-12 shrink-0 text-right text-sm font-semibold text-ink">{formatNumber(l.clicks)}</span>
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
        actions={<Button onClick={() => navigate("/app/links/new")}>New link</Button>}
      />

      <section aria-label="Totals" className="flex flex-col border border-ink bg-paper sm:flex-row">
        <Readout label="Links">
          <p className="text-[34px] font-semibold tracking-[-0.68px] text-ink">{summary.data ? formatNumber(summary.data.total_links) : "—"}</p>
        </Readout>
        <Readout label="Total clicks">
          <p className="text-[34px] font-semibold tracking-[-0.68px] text-ink">{summary.data ? formatNumber(summary.data.total_clicks) : "—"}</p>
        </Readout>
        <Readout label="Top link" last>
          {top ? (
            <>
              <Link to={`/app/links/${top.link_id}`} className="truncate font-mono text-[28px] font-semibold text-ink hover:underline">
                /{top.alias}
              </Link>
              <p className="shrink-0 text-sm text-graphite">{formatNumber(top.clicks)} clicks</p>
            </>
          ) : (
            <p className="text-[34px] font-semibold text-ink">—</p>
          )}
        </Readout>
      </section>

      <div className="flex flex-1 flex-col gap-6 xl:flex-row">
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
