import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { groups, splitIntro } from "@/lib/openapi";
import { DocsPage } from "@/pages/docs/DocsPage";
import { Markdown } from "@/pages/docs/Markdown";
import { parseBlocks } from "@/pages/docs/parse";
import { fail, ok, renderRoute, stubFetch } from "@/test/helpers";

function renderDocs() {
  // Tamu: /auth/me 401 dan refresh juga 401
  stubFetch(() => fail(401, "UNAUTHORIZED", "Authentication required."));
  return renderRoute(<DocsPage />);
}

describe("Markdown", () => {
  it("mengenali paragraf, daftar, tabel, dan blok kode", () => {
    const blocks = parseBlocks("Intro **bold** text.\n\n- one\n- two\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n```bash\ncurl x\n```");
    expect(blocks.map((b) => b.kind)).toEqual(["p", "ul", "table", "code"]);
    expect(blocks[2]).toMatchObject({ head: ["A", "B"], rows: [["1", "2"]] });
    expect(blocks[3]).toMatchObject({ lang: "bash", code: "curl x" });
  });

  it("tidak pernah merender HTML mentah", () => {
    const { container } = render(<Markdown text={'<img src=x onerror="alert(1)"> and `<b>code</b>`'} />);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("b")).toBeNull();
    expect(container).toHaveTextContent('<img src=x onerror="alert(1)">');
  });

  it("menolak tautan dengan skema berbahaya", () => {
    render(<Markdown text="[bad](javascript:alert(1)) and [good](https://example.com)" />);
    expect(screen.queryByRole("link", { name: "bad" })).toBeNull();
    expect(screen.getByRole("link", { name: "good" })).toHaveAttribute("href", "https://example.com");
  });
});

describe("splitIntro", () => {
  it("memecah deskripsi spec per judul ##", () => {
    const { lead, sections } = splitIntro("Lead text.\n\n## Quick start\n\nBody 1\n\n## Rate limits\n\nBody 2");
    expect(lead).toBe("Lead text.");
    expect(sections.map((s) => [s.id, s.title, s.body])).toEqual([
      ["quick-start", "Quick start", "Body 1"],
      ["rate-limits", "Rate limits", "Body 2"],
    ]);
  });
});

describe("DocsPage", () => {
  it("menampilkan setiap operasi di spec dengan anchor-nya", () => {
    renderDocs();
    for (const op of groups().flatMap((g) => g.operations)) {
      expect(document.getElementById(op.id), op.id).not.toBeNull();
    }
    expect(screen.getByRole("heading", { level: 1, name: "LinkForge API" })).toBeInTheDocument();
  });

  it("menampilkan endpoint lupa password lengkap dengan contoh curl dan error INVALID_RESET_TOKEN", async () => {
    renderDocs();
    const card = document.getElementById("resetPassword") as HTMLElement;
    expect(within(card).getByText("/auth/reset-password")).toBeInTheDocument();
    expect(within(card).getByText(/curl -X POST/)).toBeInTheDocument();
    await userEvent.setup().click(within(card).getByRole("button", { name: /^400/ }));
    expect(within(card).getAllByText(/INVALID_RESET_TOKEN/).length).toBeGreaterThan(0);
  });

  it("tombol Local mengganti base URL pada contoh", async () => {
    const user = userEvent.setup();
    renderDocs();
    const card = document.getElementById("createLink") as HTMLElement;
    expect(within(card).getByText(/https:\/\/linkforge\.deaprima\.my\.id\/api\/v1\/links/)).toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: "Local" }));
    expect(within(card).getByText(/http:\/\/localhost:8080\/api\/v1\/links/)).toBeInTheDocument();
  });
});

describe("DocsPage — tombol akun", () => {
  // Ada beberapa <header> di halaman ini; TopBar selalu yang pertama
  const topBar = () => within(screen.getAllByRole("banner")[0]);

  it("tamu melihat Sign in", async () => {
    renderDocs();
    const signIn = await topBar().findByRole("link", { name: "Sign in" });
    expect(signIn).toHaveAttribute("href", "/app/login");
    expect(topBar().queryByRole("link", { name: "Dashboard" })).toBeNull();
  });

  it("user yang sudah login melihat Dashboard, bukan Sign in", async () => {
    stubFetch(() => ok({ id: "u1", name: "Dea", email: "dea@example.com" }));
    renderRoute(<DocsPage />);
    const dashboard = await topBar().findByRole("link", { name: "Dashboard" });
    expect(dashboard).toHaveAttribute("href", "/app/dashboard");
    expect(topBar().queryByRole("link", { name: "Sign in" })).toBeNull();
  });
});
