import { useEffect, useRef, useState } from "react";
import { API_PREFIX } from "@/lib/api";

/*
 * Feed klik realtime dari WS /api/v1/analytics/realtime (lihat API-CONTRACT.md).
 * Auth lewat cookie access_token (same-origin). Reconnect otomatis dengan backoff
 * — Gateway menutup dengan 1013 saat stream Analytics putus, dan ini memang sinyal untuk reconnect.
 */

export type ClickEvent = {
  link_id: string;
  alias: string;
  country: string;
  browser: string;
  device: string;
  timestamp: string;
  /** Kapan event diterima client — dipakai untuk highlight event terbaru. */
  receivedAt: number;
};

export type RealtimeStatus = "connecting" | "connected" | "reconnecting";

const MAX_EVENTS = 20;
const MAX_BACKOFF_MS = 30_000;

function socketUrl(): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}${API_PREFIX}/analytics/realtime`;
}

/** @param linkId opsional: hanya simpan event untuk link ini (dipakai halaman Link detail). */
export function useRealtimeClicks(linkId?: string) {
  const [events, setEvents] = useState<ClickEvent[]>([]);
  const [status, setStatus] = useState<RealtimeStatus>("connecting");
  const attempt = useRef(0);

  useEffect(() => {
    let socket: WebSocket | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let disposed = false;

    const connect = () => {
      socket = new WebSocket(socketUrl());
      socket.onopen = () => {
        attempt.current = 0;
        setStatus("connected");
      };
      socket.onmessage = (msg) => {
        try {
          const parsed = JSON.parse(msg.data) as { event: string; data: Omit<ClickEvent, "receivedAt"> };
          if (parsed.event !== "link.clicked") return;
          if (linkId && parsed.data.link_id !== linkId) return;
          setEvents((prev) => [{ ...parsed.data, receivedAt: Date.now() }, ...prev].slice(0, MAX_EVENTS));
        } catch {
          /* abaikan pesan yang tidak dikenal */
        }
      };
      socket.onclose = () => {
        if (disposed) return;
        setStatus("reconnecting");
        const delay = Math.min(1000 * 2 ** attempt.current, MAX_BACKOFF_MS);
        attempt.current += 1;
        retryTimer = setTimeout(connect, delay);
      };
    };

    connect();
    return () => {
      disposed = true;
      clearTimeout(retryTimer);
      socket?.close();
    };
  }, [linkId]);

  return { events, status };
}

/** Re-render periodik supaya label "12 s ago" tetap akurat. */
export function useNow(intervalMs = 10_000): number {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}
