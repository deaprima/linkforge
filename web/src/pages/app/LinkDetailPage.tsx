import { useQuery } from "@tanstack/react-query";
import { cn } from "cn";
import { useState, type ReactNode } from "react";
import { Link as RouterLink, useNavigate, useParams } from "react-router";
import { BreakdownList, DailyChart, Segmented } from "@/components/charts";
import { CopyIcon } from "@/components/brand/Icons";
import { Panel, PanelMessage } from "@/components/layout/AppLayout";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Button, ButtonLink, Chip } from "@/components/ui/primitives";
import { API_PREFIX, ApiError, request, type Link, type LinkAnalytics } from "@/lib/api";
import { formatDate, formatDevice, formatNumber, isExpired } from "@/lib/format";
import { copyShortUrl, DeleteLinkDialog } from "@/pages/app/LinksPage";

const RANGES = [7, 30, 90] as const;
type Range = (typeof RANGES)[number];

const isoDay = (d: Date) => d.toISOString().slice(0, 10);
const rangeFmt = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", timeZone: "UTC" });
const rangeFmtYear = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });

function rangeDates(days: Range) {
  const to = new Date();
  const from = new Date(to.getTime() - (days - 1) * 86_400_000);
  const sameMonth = from.getUTCMonth() === to.getUTCMonth() && from.getUTCFullYear() === to.getUTCFullYear();
  const compact = sameMonth ? `${from.getUTCDate()}–${rangeFmt.format(to)}` : `${rangeFmt.format(from)} – ${rangeFmt.format(to)}`;
  return { from: isoDay(from), to: isoDay(to), label: `${rangeFmt.format(from)} – ${rangeFmtYear.format(to)}`, compact };
}

/** Sel detail (Figma "Details"): label graphite 12px + nilai 14px. */
function Detail({ label, children, className, wrap }: { label: string; children: ReactNode; className?: string; wrap?: boolean }) {
  return (
    <div className={cn("flex min-w-0 flex-col gap-1 px-4 py-3 md:px-4.5 md:py-3.5", className)}>
      <p className="text-xs text-graphite">{label}</p>
      <div className={cn("text-sm text-ink", wrap ? "break-words md:truncate" : "truncate")}>{children}</div>
    </div>
  );
}

function QrPanel({ link }: { link: Link }) {
  const [size, setSize] = useState<128 | 256 | 512>(256);
  const [format, setFormat] = useState<"png" | "svg">("png");
  const [options, setOptions] = useState(false);
  const base = `${API_PREFIX}/links/${link.id}/qr`;
  const download = (
    <ButtonLink href={`${base}?size=${size}&format=${format}`} download={`${link.alias}-${size}.${format}`} className="w-full">
      Download {format.toUpperCase()}
    </ButtonLink>
  );
  return (
    <Panel title="QR code" className="w-full shrink-0 lg:w-80">
      <div className="flex flex-col gap-4 p-4 sm:p-5">
        {/* Mobile (Figma 20:466): QR kecil di kiri, unduh + "Change size" di kanan; pilihan ukuran/format dibuka sesuai kebutuhan */}
        <div className="flex gap-4 lg:flex-col lg:gap-4">
          <div className="flex shrink-0 justify-center self-start border border-ink bg-white p-2 lg:self-stretch lg:p-3">
            {/* Selalu SVG untuk preview supaya tajam; QR berlatar putih di dark mode agar tetap bisa dipindai */}
            <img src={`${base}?format=svg&size=200`} alt={`QR code for ${link.short_url}`} className="size-[104px] lg:size-[200px]" />
          </div>
          <div className="flex min-w-0 flex-1 flex-col gap-2.5 lg:hidden">
            <p className="text-xs text-graphite">
              {format.toUpperCase()}, {size} px
            </p>
            {download}
            <Button variant="outline" className="w-full" aria-expanded={options} onClick={() => setOptions((o) => !o)}>
              Change size
            </Button>
          </div>
        </div>
        <div className={cn("flex-col gap-4 lg:flex", options ? "flex" : "hidden")}>
          <p className="text-[13px] font-semibold text-ink">Size</p>
          <Segmented label="QR size" mono value={size} onChange={setSize} options={[128, 256, 512].map((v) => ({ value: v as 128 | 256 | 512, label: String(v) }))} />
          <p className="text-[13px] font-semibold text-ink">Format</p>
          <Segmented
            label="QR format"
            value={format}
            onChange={setFormat}
            options={[
              { value: "png", label: "PNG" },
              { value: "svg", label: "SVG" },
            ]}
          />
        </div>
        <div className="hidden lg:block">{download}</div>
      </div>
    </Panel>
  );
}

function ClicksPanel({ link }: { link: Link }) {
  const [range, setRange] = useState<Range>(30);
  const { from, to, label, compact } = rangeDates(range);
  const stats = useQuery({
    queryKey: ["analytics", "link", link.id, range],
    queryFn: () => request<LinkAnalytics>(`/analytics/links/${link.id}?from=${from}&to=${to}`),
  });

  return (
    <Panel
      title="Clicks"
      className="min-w-0 flex-1"
      aside={
        <DropdownMenu>
          <DropdownMenuTrigger className="cursor-pointer focus-visible:outline-2 focus-visible:outline-signal" aria-label="Change date range">
            <Chip>
              <span className="sm:hidden">{compact}</span>
              <span className="hidden sm:inline">{label}</span>
            </Chip>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="border-ink shadow-none">
            {RANGES.map((r) => (
              <DropdownMenuItem key={r} onSelect={() => setRange(r)}>
                Last {r} days
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      }
    >
      {stats.isPending ? (
        <PanelMessage>Loading…</PanelMessage>
      ) : stats.isError ? (
        <PanelMessage tone="error">Couldn't load analytics. {stats.error.message}</PanelMessage>
      ) : (
        <div className="flex flex-col gap-4 p-4 sm:gap-5 sm:p-5">
          <div className="flex gap-10">
            <div className="flex flex-col gap-0.5">
              <p className="text-xs text-graphite">Total clicks</p>
              <p className="text-[28px] font-semibold tracking-[-0.56px] text-ink">{formatNumber(stats.data.total_clicks)}</p>
            </div>
            <div className="flex flex-col gap-0.5">
              <p className="text-xs text-graphite">Unique visitors</p>
              <p className="text-[28px] font-semibold tracking-[-0.56px] text-ink">{formatNumber(stats.data.unique_clicks)}</p>
            </div>
          </div>
          <DailyChart days={stats.data.time_series} />
          <div className="flex flex-col gap-8 border-t border-ink pt-4 md:flex-row">
            <BreakdownList title="Countries" mono items={stats.data.top_countries.map((c) => ({ label: c.country, count: c.count }))} />
            <BreakdownList title="Browsers" items={stats.data.top_browsers.map((b) => ({ label: b.browser, count: b.count }))} />
            <BreakdownList title="Devices" items={stats.data.top_devices.map((d) => ({ label: formatDevice(d.device), count: d.count }))} />
          </div>
        </div>
      )}
    </Panel>
  );
}

export function LinkDetailPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [deleting, setDeleting] = useState(false);
  const link = useQuery({ queryKey: ["links", "detail", id], queryFn: () => request<Link>(`/links/${id}`) });

  if (link.isPending) return <PanelMessage>Loading…</PanelMessage>;
  if (link.isError) {
    const notFound = link.error instanceof ApiError && (link.error.code === "LINK_NOT_FOUND" || link.error.code === "VALIDATION_ERROR");
    return (
      <div className="flex flex-col items-start gap-4">
        <p className="text-sm text-graphite">{notFound ? "This link doesn't exist or isn't yours." : `Couldn't load the link. ${link.error.message}`}</p>
        <Button variant="outline" onClick={() => navigate("/app/links")}>
          Back to links
        </Button>
      </div>
    );
  }

  const l = link.data;
  const expired = isExpired(l.expired_at);

  return (
    <>
      <div className="flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-end sm:justify-between">
        <div className="flex min-w-0 flex-col gap-2">
          <nav aria-label="Breadcrumb" className="text-[13px] whitespace-pre text-graphite">
            <RouterLink to="/app/links" className="hover:underline">
              Links
            </RouterLink>
            {"  /  "}
            {l.alias}
          </nav>
          <div className="flex items-center gap-3">
            <h1 className="truncate font-mono text-2xl font-medium text-ink sm:text-[28px]">/{l.alias}</h1>
            {expired ? <Chip tone="stop">Expired</Chip> : <Chip tone="signal">Active</Chip>}
          </div>
          <div className="flex items-center gap-2">
            <span className="truncate font-mono text-[13px] text-graphite">{l.short_url.replace(/^https?:\/\//, "")}</span>
            <button type="button" aria-label="Copy short link" className="cursor-pointer p-0.5 focus-visible:outline-2 focus-visible:outline-signal" onClick={() => copyShortUrl(l.short_url)}>
              <CopyIcon />
            </button>
          </div>
        </div>
        <div className="flex gap-2.5 [&>button]:flex-1 sm:[&>button]:flex-none">
          <Button variant="outline" onClick={() => navigate(`/app/links/${l.id}/edit`)}>
            Edit
          </Button>
          <Button variant="outline" onClick={() => setDeleting(true)}>
            Delete
          </Button>
        </div>
      </div>

      <section aria-label="Link details" className="flex flex-col border border-ink bg-paper md:flex-row">
        <Detail label="Destination" className="border-b border-ink md:flex-1 md:border-r md:border-b-0" wrap>
          <a href={l.original_url} target="_blank" rel="noreferrer noopener" className="hover:underline" title={l.original_url}>
            {l.original_url.replace(/^https?:\/\//, "")}
          </a>
        </Detail>
        <Detail label="Created" className="border-b border-ink md:w-[180px] md:border-r md:border-b-0">
          {formatDate(l.created_at)}
        </Detail>
        <Detail label="Expires" className="border-b border-ink md:w-[180px] md:border-r md:border-b-0">
          {l.expired_at ? formatDate(l.expired_at) : "Never"}
        </Detail>
        <Detail label="Password" className="md:w-[140px]">
          {l.has_password ? "On" : "Off"}
        </Detail>
      </section>

      <div className="flex flex-col gap-4 sm:gap-6 lg:flex-row lg:items-start">
        <ClicksPanel link={l} />
        <QrPanel link={l} />
      </div>

      <DeleteLinkDialog link={deleting ? l : null} onClose={() => setDeleting(false)} onDeleted={() => navigate("/app/links", { replace: true })} />
    </>
  );
}
