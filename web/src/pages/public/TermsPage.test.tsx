import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RegisterPage } from "@/pages/auth/AuthPages";
import { TermsPage } from "@/pages/public/TermsPage";
import { fail, renderRoute, stubFetch } from "@/test/helpers";

describe("TermsPage", () => {
  it("menyatakan dengan jelas bahwa ini proyek portofolio tanpa jaminan", () => {
    renderRoute(<TermsPage />);
    expect(screen.getByRole("heading", { level: 1, name: /Terms & disclaimer/ })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "This is a portfolio project" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "No guarantees" })).toBeInTheDocument();
    expect(document.title).toBe("Terms & disclaimer · LinkForge");
  });

  it("menjelaskan data klik apa adanya (IP, browser, negara) dan cookie sesi saja", () => {
    renderRoute(<TermsPage />);
    const stored = screen.getByRole("heading", { name: "What is stored" }).closest("section") as HTMLElement;
    expect(within(stored).getByText(/IP address/)).toBeInTheDocument();
    expect(within(stored).getByText(/HttpOnly session cookies/)).toBeInTheDocument();
    expect(within(stored).getByText(/not deleted automatically/)).toBeInTheDocument();
  });

  it("menautkan kontak dan kode sumber ke GitHub", () => {
    renderRoute(<TermsPage />);
    expect(screen.getByRole("link", { name: "open an issue on GitHub" })).toHaveAttribute("href", "https://github.com/deaprima/linkforge/issues");
  });
});

describe("RegisterPage", () => {
  it("menautkan ke halaman Terms", async () => {
    stubFetch(() => fail(401, "UNAUTHORIZED", "x"));
    renderRoute(<RegisterPage />, { path: "/app/register", route: "/app/register" });
    expect(await screen.findByRole("link", { name: "terms" })).toHaveAttribute("href", "/app/terms");
  });
});
