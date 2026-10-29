import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { LandingPage } from "@/pages/landing/LandingPage";
import { fail, ok, renderRoute, stubFetch } from "@/test/helpers";

const me = { id: "u1", name: "Dea", email: "dea@example.com" };

/** Server yang menganggap pengunjung login (/auth/me 200) atau tamu (401, refresh juga 401). */
function stubSession(loggedIn: boolean) {
  return stubFetch(() => (loggedIn ? ok(me) : fail(401, "UNAUTHORIZED", "Authentication required.")));
}

// Tombol akun ada di header (nav); bagian lain landing punya tombol serupa sendiri
const nav = () => within(screen.getByRole("banner"));

describe("LandingPage — tombol akun di nav", () => {
  it("tamu melihat Sign in dan Create free account", async () => {
    stubSession(false);
    renderRoute(<LandingPage />);

    await waitFor(() => expect(nav().getByRole("link", { name: "Sign in" })).not.toHaveClass("invisible"));
    expect(nav().getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/app/login");
    expect(nav().getByRole("link", { name: "Create free account" })).toHaveAttribute("href", "/app/register");
    expect(nav().queryByRole("link", { name: "Dashboard" })).toBeNull();
  });

  it("user yang sudah login melihat satu tombol Dashboard", async () => {
    stubSession(true);
    renderRoute(<LandingPage />);

    const dashboard = await nav().findByRole("link", { name: "Dashboard" });
    expect(dashboard).toHaveAttribute("href", "/app/dashboard");
    expect(nav().queryByRole("link", { name: "Sign in" })).toBeNull();
    expect(nav().queryByRole("link", { name: "Create free account" })).toBeNull();
  });

  it("menyembunyikan tombol tamu selagi status login belum diketahui, tanpa menghilangkannya dari layout", async () => {
    stubFetch(() => new Promise<Response>(() => {})); // /auth/me tak kunjung selesai
    renderRoute(<LandingPage />);

    expect(nav().getByRole("link", { name: "Sign in", hidden: true })).toHaveClass("invisible");
    expect(nav().getByRole("link", { name: "Create free account", hidden: true })).toHaveClass("invisible");
    expect(nav().queryByRole("link", { name: "Dashboard", hidden: true })).toBeNull();
  });

  it("kalau API tidak terjangkau, tamu tetap mendapat tombol Sign in yang terlihat", async () => {
    stubFetch(() => fail(503, "SERVICE_UNAVAILABLE", "Down"));
    renderRoute(<LandingPage />);

    await waitFor(() => expect(nav().getByRole("link", { name: "Sign in" })).not.toHaveClass("invisible"));
    expect(nav().queryByRole("link", { name: "Dashboard" })).toBeNull();
  });

  it("menu mobile juga menampilkan Dashboard untuk user yang sudah login", async () => {
    stubSession(true);
    renderRoute(<LandingPage />);
    await nav().findByRole("link", { name: "Dashboard" });

    await userEvent.setup().click(nav().getByRole("button", { name: "Open menu" }));
    const menu = within(document.getElementById("landing-menu") as HTMLElement);
    expect(menu.getByRole("link", { name: "Dashboard" })).toHaveAttribute("href", "/app/dashboard");
    expect(menu.queryByRole("link", { name: "Sign in" })).toBeNull();
    expect(menu.queryByRole("link", { name: "Create free account" })).toBeNull();
  });

  it("menu mobile untuk tamu tetap Create free account dan Sign in", async () => {
    stubSession(false);
    renderRoute(<LandingPage />);

    await userEvent.setup().click(nav().getByRole("button", { name: "Open menu" }));
    const menu = within(document.getElementById("landing-menu") as HTMLElement);
    expect(menu.getByRole("link", { name: "Create free account" })).toBeInTheDocument();
    expect(menu.getByRole("link", { name: "Sign in" })).toBeInTheDocument();
    expect(menu.queryByRole("link", { name: "Dashboard" })).toBeNull();
  });
});
