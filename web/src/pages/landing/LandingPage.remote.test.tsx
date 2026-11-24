import { screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

// Landing di-host terpisah dari app (mis. Vercel): VITE_APP_ORIGIN di-set saat build.
const ORIGIN = "https://app.example.com";

async function renderRemote() {
  vi.stubEnv("VITE_APP_ORIGIN", ORIGIN + "/");
  vi.resetModules();
  const { LandingPage } = await import("@/pages/landing/LandingPage");
  const { renderRoute, stubFetch } = await import("@/test/helpers");
  const fetch = stubFetch(() => new Response(null, { status: 500 }));
  renderRoute(<LandingPage />);
  return fetch;
}

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("LandingPage — di origin terpisah dari app", () => {
  it("mengarahkan semua link /app/* ke APP_ORIGIN", async () => {
    await renderRemote();
    const links = screen.getAllByRole("link").map((a) => a.getAttribute("href") ?? "");
    const appLinks = links.filter((h) => h.includes("/app"));
    expect(appLinks.length).toBeGreaterThan(0);
    for (const href of appLinks) expect(href.startsWith(`${ORIGIN}/app/`)).toBe(true);
    // Logo tetap ke landing itu sendiri
    expect(screen.getByRole("link", { name: "LinkForge home" })).toHaveAttribute("href", "/");
  });

  it("tidak memanggil /auth/me dan langsung menampilkan tombol tamu", async () => {
    const fetch = await renderRemote();
    const nav = within(screen.getByRole("banner"));
    expect(nav.getByRole("link", { name: "Sign in" })).not.toHaveClass("invisible");
    expect(nav.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", `${ORIGIN}/app/login`);
    expect(nav.getByRole("link", { name: "Create free account" })).toHaveAttribute("href", `${ORIGIN}/app/register`);
    expect(fetch).not.toHaveBeenCalled();
  });
});
