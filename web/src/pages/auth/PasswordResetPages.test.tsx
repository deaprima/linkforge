import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { ForgotPasswordPage, ResetPasswordPage } from "@/pages/auth/PasswordResetPages";
import { fail, ok, renderRoute, stubFetch } from "@/test/helpers";

describe("ForgotPasswordPage", () => {
  function setup() {
    const user = userEvent.setup();
    renderRoute(<ForgotPasswordPage />, { path: "/app/forgot-password", route: "/app/forgot-password" });
    return user;
  }

  it("menolak email yang bukan email tanpa memanggil API", async () => {
    const fetchMock = stubFetch(() => ok({}));
    const user = setup();
    await user.type(screen.getByLabelText("Email"), "not-an-email");
    await user.click(screen.getByRole("button", { name: "Send reset link" }));
    expect(await screen.findByText("Enter a valid email address.")).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("mengirim email yang sudah di-trim lalu menampilkan konfirmasi", async () => {
    const fetchMock = stubFetch(() => ok({ message: "sent" }));
    const user = setup();
    await user.type(screen.getByLabelText("Email"), "  dea@example.com ");
    await user.click(screen.getByRole("button", { name: "Send reset link" }));

    expect(await screen.findByRole("heading", { name: "Check your email" })).toBeInTheDocument();
    expect(screen.getByText("dea@example.com")).toBeInTheDocument();
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/auth/forgot-password");
    expect(init?.body).toBe('{"email":"dea@example.com"}');
  });

  it("'Use a different email' kembali ke form", async () => {
    stubFetch(() => ok({ message: "sent" }));
    const user = setup();
    await user.type(screen.getByLabelText("Email"), "dea@example.com");
    await user.click(screen.getByRole("button", { name: "Send reset link" }));
    await user.click(await screen.findByRole("button", { name: "Use a different email" }));
    expect(await screen.findByRole("button", { name: "Send reset link" })).toBeInTheDocument();
  });

  it("429 menyebut berapa lama harus menunggu", async () => {
    stubFetch(() => fail(429, "RATE_LIMIT_EXCEEDED", "slow down", { "Retry-After": "1800" }));
    const user = setup();
    await user.type(screen.getByLabelText("Email"), "dea@example.com");
    await user.click(screen.getByRole("button", { name: "Send reset link" }));
    expect(await screen.findByText("Too many attempts. Try again in 30 minutes.")).toBeInTheDocument();
  });

  it("layanan mati: pesan ramah, bukan pesan teknis", async () => {
    stubFetch(() => fail(503, "SERVICE_UNAVAILABLE", "dial tcp: refused"));
    const user = setup();
    await user.type(screen.getByLabelText("Email"), "dea@example.com");
    await user.click(screen.getByRole("button", { name: "Send reset link" }));
    expect(await screen.findByText(/temporarily unavailable/)).toBeInTheDocument();
    expect(screen.queryByText(/dial tcp/)).not.toBeInTheDocument();
  });
});

describe("ResetPasswordPage", () => {
  function setup(route = "/app/reset-password?token=abc123") {
    const user = userEvent.setup();
    renderRoute(<ResetPasswordPage />, { path: "/app/reset-password", route });
    return user;
  }

  it("tanpa token: menawarkan link baru, tanpa form", () => {
    setup("/app/reset-password");
    expect(screen.getByRole("heading", { name: "This link is incomplete" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Request a new link" })).toHaveAttribute("href", "/app/forgot-password");
    expect(screen.queryByLabelText("New password")).not.toBeInTheDocument();
  });

  it("validasi: panjang password dan konfirmasi harus sama", async () => {
    const fetchMock = stubFetch(() => ok({}));
    const user = setup();

    await user.type(screen.getByLabelText("New password"), "short");
    await user.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("At least 8 characters.")).toBeInTheDocument();

    await user.clear(screen.getByLabelText("New password"));
    await user.type(screen.getByLabelText("New password"), "longenough1");
    await user.type(screen.getByLabelText("Confirm new password"), "different123");
    await user.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("The two passwords don't match.")).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("sukses: mengirim token + password, lalu mengarahkan ke sign in", async () => {
    const fetchMock = stubFetch(() => ok({ message: "changed" }));
    const user = setup();
    await user.type(screen.getByLabelText("New password"), "brandnewpass");
    await user.type(screen.getByLabelText("Confirm new password"), "brandnewpass");
    await user.click(screen.getByRole("button", { name: "Change password" }));

    expect(await screen.findByRole("heading", { name: "Password changed" })).toBeInTheDocument();
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/auth/reset-password");
    expect(init?.body).toBe('{"token":"abc123","password":"brandnewpass"}');

    await user.click(screen.getByRole("button", { name: "Sign in" }));
    await waitFor(() => expect(screen.getByTestId("location")).toHaveTextContent("/app/login"));
  });

  it("token kedaluwarsa/terpakai: tampil halaman 'link expired' dengan tombol minta link baru", async () => {
    stubFetch(() => fail(400, "INVALID_RESET_TOKEN", "This reset link is invalid or has expired."));
    const user = setup();
    await user.type(screen.getByLabelText("New password"), "brandnewpass");
    await user.type(screen.getByLabelText("Confirm new password"), "brandnewpass");
    await user.click(screen.getByRole("button", { name: "Change password" }));

    expect(await screen.findByRole("heading", { name: "This link has expired" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Request a new link" })).toBeInTheDocument();
  });

  it("error lain tetap di form dan menampilkan pesan", async () => {
    stubFetch(() => fail(429, "RATE_LIMIT_EXCEEDED", "slow", { "Retry-After": "60" }));
    const user = setup();
    await user.type(screen.getByLabelText("New password"), "brandnewpass");
    await user.type(screen.getByLabelText("Confirm new password"), "brandnewpass");
    await user.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("Too many attempts. Try again in a minute.")).toBeInTheDocument();
    expect(screen.getByLabelText("New password")).toBeInTheDocument();
  });
});
