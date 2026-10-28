import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PasswordGatePage } from "@/pages/public/PublicPages";
import { fail, ok, renderRoute, stubFetch } from "@/test/helpers";

const NOW = new Date("2026-01-01T00:00:00Z");
const resetIn = (minutes: number) => String(Math.round(NOW.getTime() / 1000) + minutes * 60);
const rl = (remaining: number, minutes: number) => ({ "X-RateLimit-Limit": "5", "X-RateLimit-Remaining": String(remaining), "X-RateLimit-Reset": resetIn(minutes) });

async function submit(password = "secret") {
  const user = userEvent.setup();
  renderRoute(<PasswordGatePage />, { path: "/app/p/:alias", route: "/app/p/pitch" });
  if (password) await user.type(screen.getByLabelText("Password"), password);
  await user.click(screen.getByRole("button", { name: "Open link" }));
}

// Hanya Date yang dibekukan: sisa menit dihitung dari X-RateLimit-Reset terhadap "sekarang"
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: NOW });
});
afterEach(() => vi.useRealTimers());

describe("PasswordGatePage", () => {
  it("menampilkan alias yang dibuka", () => {
    renderRoute(<PasswordGatePage />, { path: "/app/p/:alias", route: "/app/p/pitch" });
    expect(screen.getByText(/\/pitch$/)).toBeInTheDocument();
  });

  it("password kosong tidak memanggil API", async () => {
    const fetchMock = stubFetch(() => ok({}));
    await submit("");
    expect(await screen.findByText("Enter the password.")).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("password salah: sisa percobaan dan waktu dari header", async () => {
    stubFetch(() => fail(401, "UNAUTHORIZED", "wrong password", rl(4, 12)));
    await submit();
    expect(await screen.findByText("That password is wrong. 4 attempts left in the next 12 minutes.")).toBeInTheDocument();
  });

  it("sisa 1 percobaan dan jendela 1 menit memakai bentuk tunggal", async () => {
    stubFetch(() => fail(401, "UNAUTHORIZED", "wrong password", rl(1, 1)));
    await submit();
    expect(await screen.findByText("That password is wrong. 1 attempt left this minute.")).toBeInTheDocument();
  });

  it("percobaan habis", async () => {
    stubFetch(() => fail(401, "UNAUTHORIZED", "wrong password", rl(0, 15)));
    await submit();
    expect(await screen.findByText("That password is wrong. No attempts left; try again in 15 minutes.")).toBeInTheDocument();
  });

  it("tanpa header rate limit: pesan polos", async () => {
    stubFetch(() => fail(401, "UNAUTHORIZED", "wrong password"));
    await submit();
    expect(await screen.findByText("That password is wrong.")).toBeInTheDocument();
  });

  it("429 memakai Retry-After", async () => {
    stubFetch(() => fail(429, "RATE_LIMITED", "slow down", { "Retry-After": "600" }));
    await submit();
    expect(await screen.findByText("Too many attempts. Try again in 10 minutes.")).toBeInTheDocument();
  });

  it.each([
    [404, "NOT_FOUND", "/app/not-found?alias=pitch"],
    [410, "GONE", "/app/expired?alias=pitch"],
    [503, "SERVICE_UNAVAILABLE", "/app/unavailable?alias=pitch"],
  ])("status %i pindah ke halaman error", async (status, code, target) => {
    stubFetch(() => fail(status, code, "x"));
    await submit();
    await waitFor(() => expect(screen.getByTestId("location")).toHaveTextContent(target));
  });

  it("mengirim password ke /:alias/verify tanpa prefix /api", async () => {
    const fetchMock = stubFetch(() => fail(401, "UNAUTHORIZED", "wrong password"));
    await submit("hunter2");
    await screen.findByText("That password is wrong.");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/pitch/verify");
    expect(init?.body).toBe('{"password":"hunter2"}');
  });

  it("sukses: browser pindah ke redirect_url", async () => {
    const assign = vi.fn();
    vi.stubGlobal("location", { ...window.location, assign, origin: "http://localhost:3000" });
    stubFetch(() => ok({ redirect_url: "https://example.com/dest" }));
    await submit();
    await waitFor(() => expect(assign).toHaveBeenCalledWith("https://example.com/dest"));
  });
});
