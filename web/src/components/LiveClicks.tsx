import { cn } from "cn";
import { StatusDot } from "@/components/brand/Icons";
import { Panel, PanelMessage } from "@/components/layout/AppLayout";
import { formatDevice, timeAgo } from "@/lib/format";
import { useNow, useRealtimeClicks, type RealtimeStatus } from "@/lib/realtime";

/** Event teratas di-highlight signal-soft (seperti baris pertama di Figma) selama 10 detik. */
const FRESH_MS = 10_000;

function Status({ status }: { status: RealtimeStatus }) {
  if (status === "connected") {
    return (
      <span className="flex items-center gap-1.5 text-signal-ink">
        <StatusDot />
        Connected
      </span>
    );
  }
  return (
    <span className="flex items-center gap-1.5 text-graphite" role="status">
      <span aria-hidden className="size-2 border border-graphite" />
      {status === "connecting" ? "Connecting…" : "Reconnecting…"}
    </span>
  );
}

/** Panel "Live clicks" (Figma Overview 16:366). linkId opsional untuk memfilter satu link. */
export function LiveClicks({ linkId, className }: { linkId?: string; className?: string }) {
  const { events, status } = useRealtimeClicks(linkId);
  const now = useNow(5_000);

  return (
    <Panel title="Live clicks" aside={<Status status={status} />} className={className}>
      {events.length === 0 ? (
        <PanelMessage>Waiting for clicks. Open one of your short links to see it here instantly.</PanelMessage>
      ) : (
        <ul aria-live="polite">
          {events.map((ev, i) => (
            <li
              key={`${ev.receivedAt}-${i}`}
              className={cn(
                "flex items-center gap-3 px-5 py-3",
                i > 0 && "border-t border-ink",
                // Hanya event teratas yang di-highlight (Figma), selama masih baru
                i === 0 && now - ev.receivedAt < FRESH_MS && "bg-signal-soft",
              )}
            >
              <span className="w-[110px] shrink-0 truncate font-mono text-xs text-ink">/{ev.alias}</span>
              <span className="w-7 shrink-0 font-mono text-xs text-graphite" title="Country">
                {ev.country}
              </span>
              <span className="min-w-0 flex-1 truncate text-[13px] text-ink">
                {ev.browser}, {formatDevice(ev.device)}
              </span>
              <span className="shrink-0 text-xs whitespace-nowrap text-graphite">{timeAgo(new Date(ev.receivedAt), now)}</span>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}
