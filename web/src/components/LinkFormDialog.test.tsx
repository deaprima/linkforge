import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LinkFormDialog } from "@/components/LinkFormDialog";
import { fail, ok, renderRoute, stubFetch } from "@/test/helpers";

const saved = { id: "1", original_url: "https://a.dev", alias: "abc", short_url: "http://x/abc", has_password: false, click_count: 0, expired_at: null, created_at: "", updated_at: "" };

function setup() {
  const user = userEvent.setup();
  const onSaved = vi.fn();
  renderRoute(<LinkFormDialog open onClose={() => {}} onSaved={onSaved} />);
  return { user, onSaved, submit: () => user.click(screen.getByRole("button", { name: "Create link" })) };
}

describe("LinkFormDialog", () => {
  it("URL harus http(s)", async () => {
    const fetchMock = stubFetch(() => ok(saved));
    const { user, submit } = setup();
    await user.type(screen.getByLabelText("Destination URL"), "ftp://a.dev");
    await submit();
    expect(await screen.findByText("Must start with http:// or https://")).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("alias hanya huruf, angka, - dan _ (3–64)", async () => {
    const fetchMock = stubFetch(() => ok(saved));
    const { user, submit } = setup();
    await user.type(screen.getByLabelText("Destination URL"), "https://a.dev");
    await user.type(screen.getByLabelText(/Custom alias/), "a!");
    await submit();
    expect(await screen.findByText("3–64 characters: letters, numbers, - and _ only.")).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("password 1–3 karakter ditolak", async () => {
    stubFetch(() => ok(saved));
    const { user, submit } = setup();
    await user.type(screen.getByLabelText("Destination URL"), "https://a.dev");
    await user.type(screen.getByLabelText(/Password/), "abc");
    await submit();
    expect(await screen.findByText("Between 4 and 72 characters.")).toBeInTheDocument();
  });

  it("form valid mengirim POST /links tanpa field kosong", async () => {
    const fetchMock = stubFetch(() => ok(saved));
    const { user, onSaved, submit } = setup();
    await user.type(screen.getByLabelText("Destination URL"), " https://a.dev ");
    await submit();
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/links");
    expect(init?.body).toBe('{"original_url":"https://a.dev"}');
  });

  it("409 menampilkan 'alias already taken' di field alias", async () => {
    stubFetch(() => fail(409, "ALIAS_ALREADY_TAKEN", "taken"));
    const { user, submit } = setup();
    await user.type(screen.getByLabelText("Destination URL"), "https://a.dev");
    await user.type(screen.getByLabelText(/Custom alias/), "pitch");
    await submit();
    expect(await screen.findByText("This alias is already taken. Try another one.")).toBeInTheDocument();
  });
});
