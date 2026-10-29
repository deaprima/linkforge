import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ApiKeysPage } from "@/pages/app/ApiKeysPage";
import { ok, renderRoute, stubFetch } from "@/test/helpers";

describe("ApiKeysPage", () => {
  it("punya pintasan ke dokumentasi API di header, di samping New key", async () => {
    stubFetch(() => ok([]));
    renderRoute(<ApiKeysPage />, { path: "/app/api-keys", route: "/app/api-keys" });

    const docs = await screen.findByRole("link", { name: "API docs" });
    expect(docs).toHaveAttribute("href", "/app/docs");
    expect(screen.getByRole("button", { name: "New key" })).toBeInTheDocument();
  });

  it("pintasan docs tetap ada saat daftar key gagal dimuat", async () => {
    stubFetch(() => new Response("boom", { status: 500 }));
    renderRoute(<ApiKeysPage />, { path: "/app/api-keys", route: "/app/api-keys" });

    expect(await screen.findByRole("link", { name: "API docs" })).toHaveAttribute("href", "/app/docs");
  });
});
