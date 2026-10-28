import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import type { ReactElement } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { vi } from "vitest";
import { ThemeProvider } from "@/lib/theme";

/** Envelope sukses Gateway. */
export const ok = (data: unknown, init: ResponseInit = {}) =>
  new Response(JSON.stringify({ success: true, data }), { status: 200, headers: { "Content-Type": "application/json" }, ...init });

/** Envelope error Gateway. */
export const fail = (status: number, code: string, message: string, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify({ success: false, error: { code, message } }), { status, headers: { "Content-Type": "application/json", ...headers } });

/** Ganti fetch global; panggilannya bisa diperiksa lewat mock yang dikembalikan. */
export function stubFetch(handler: (url: string, init?: RequestInit) => Response | Promise<Response>) {
  const fn = vi.fn((input: RequestInfo | URL, init?: RequestInit) => Promise.resolve(handler(String(input), init)));
  vi.stubGlobal("fetch", fn);
  return fn;
}

/** Menampilkan path saat ini, supaya tes bisa memeriksa hasil navigasi. */
function LocationProbe() {
  const { pathname, search } = useLocation();
  return <p data-testid="location">{pathname + search}</p>;
}

/** Render di router + react-query (tanpa retry). Path lain dari `path` jatuh ke LocationProbe. */
export function renderRoute(ui: ReactElement, { path = "/", route = "/" }: { path?: string; route?: string } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <ThemeProvider>
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[route]}>
          <Routes>
            <Route path={path} element={ui} />
            <Route path="*" element={<LocationProbe />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </ThemeProvider>,
  );
}
