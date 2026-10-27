/*
 * Client API Gateway (lihat docs/API-CONTRACT.md).
 * - Sesi = cookie HttpOnly yang dipasang Gateway; JavaScript tidak pernah memegang token.
 * - Envelope {success,data,meta} / {success:false,error} di-unwrap jadi data atau ApiError.
 * - 401 → coba POST /auth/refresh sekali (single-flight), lalu ulangi request.
 */

export const API_PREFIX = "/api/v1";

export type Meta = { page: number; limit: number; total: number };

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly retryAfter?: number;

  constructor(status: number, code: string, message: string, retryAfter?: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.retryAfter = retryAfter;
  }
}

type Envelope<T> =
  | { success: true; data: T; meta?: Meta }
  | { success: false; error: { code: string; message: string } };

type RequestOptions = {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  /** Jangan mencoba refresh saat 401 (dipakai oleh endpoint auth itu sendiri). */
  noRefresh?: boolean;
  signal?: AbortSignal;
};

// Minta Gateway tidak pernah menaruh token di body JSON (lihat API-CONTRACT.md "Mode cookie-only")
const SESSION_HEADERS = { "X-Session-Mode": "cookie" } as const;

let refreshing: Promise<boolean> | null = null;

/** Single-flight: banyak request 401 bersamaan hanya memicu satu refresh. */
function refreshSession(): Promise<boolean> {
  refreshing ??= fetch(`${API_PREFIX}/auth/refresh`, { method: "POST", credentials: "same-origin", headers: SESSION_HEADERS })
    .then((res) => res.ok)
    .catch(() => false)
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

async function send(path: string, opts: RequestOptions): Promise<Response> {
  const hasBody = opts.body !== undefined;
  return fetch(`${API_PREFIX}${path}`, {
    method: opts.method ?? "GET",
    credentials: "same-origin",
    headers: hasBody ? { ...SESSION_HEADERS, "Content-Type": "application/json" } : SESSION_HEADERS,
    body: hasBody ? JSON.stringify(opts.body) : undefined,
    signal: opts.signal,
  });
}

async function toError(res: Response): Promise<ApiError> {
  const retryAfter = Number(res.headers.get("Retry-After")) || undefined;
  try {
    const body = (await res.json()) as Envelope<unknown>;
    if (!body.success) return new ApiError(res.status, body.error.code, body.error.message, retryAfter);
  } catch {
    /* body bukan JSON (mis. proxy error) */
  }
  return new ApiError(res.status, res.status >= 500 ? "SERVICE_UNAVAILABLE" : "UNKNOWN", res.statusText || "Request failed", retryAfter);
}

export async function requestWithMeta<T>(path: string, opts: RequestOptions = {}): Promise<{ data: T; meta?: Meta }> {
  let res: Response;
  try {
    res = await send(path, opts);
    if (res.status === 401 && !opts.noRefresh && (await refreshSession())) {
      res = await send(path, opts);
    }
  } catch (err) {
    if ((err as Error).name === "AbortError") throw err;
    throw new ApiError(0, "NETWORK_ERROR", "Cannot reach LinkForge. Check your connection.");
  }

  if (!res.ok) throw await toError(res);
  if (res.status === 204) return { data: undefined as T };

  const body = (await res.json()) as Envelope<T>;
  if (!body.success) throw new ApiError(res.status, body.error.code, body.error.message);
  return { data: body.data, meta: body.meta };
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  return (await requestWithMeta<T>(path, opts)).data;
}

// Tipe data — sinkron dengan respons Gateway

export type User = { id: string; name: string; email: string; created_at?: string };

export type Link = {
  id: string;
  original_url: string;
  alias: string;
  short_url: string;
  has_password: boolean;
  click_count: number;
  expired_at: string | null;
  created_at: string;
  updated_at: string;
};

export type ApiKey = { id: string; name: string; key_prefix: string; last_used_at: string | null; created_at: string };
export type CreatedApiKey = { id: string; name: string; key: string; created_at: string };

export type LinkAnalytics = {
  link_id: string;
  total_clicks: number;
  unique_clicks: number;
  top_countries: { country: string; count: number }[];
  top_browsers: { browser: string; count: number }[];
  top_devices: { device: string; count: number }[];
  time_series: { date: string; clicks: number }[];
};

export type Summary = {
  total_links: number;
  total_clicks: number;
  top_links: { link_id: string; alias: string; clicks: number }[];
};
