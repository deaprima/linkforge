import { describe, expect, it } from "vitest";
import { ApiError, request, requestWithMeta } from "@/lib/api";
import { fail, ok, stubFetch } from "@/test/helpers";

describe("request", () => {
  it("membuka envelope sukses menjadi data", async () => {
    stubFetch(() => ok({ id: "1" }));
    await expect(request<{ id: string }>("/links/1")).resolves.toEqual({ id: "1" });
  });

  it("mengembalikan meta paginasi", async () => {
    stubFetch(() => new Response(JSON.stringify({ success: true, data: [], meta: { page: 2, limit: 10, total: 31 } }), { status: 200 }));
    await expect(requestWithMeta("/links")).resolves.toEqual({ data: [], meta: { page: 2, limit: 10, total: 31 } });
  });

  it("selalu meminta mode cookie dan mengirim body sebagai JSON", async () => {
    const fetchMock = stubFetch(() => ok({}));
    await request("/links", { method: "POST", body: { original_url: "https://a.dev" } });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/links");
    expect(init?.headers).toMatchObject({ "X-Session-Mode": "cookie", "Content-Type": "application/json" });
    expect(init?.body).toBe('{"original_url":"https://a.dev"}');
  });

  it("memakai prefix kosong untuk endpoint root", async () => {
    const fetchMock = stubFetch(() => ok({ redirect_url: "https://a.dev" }));
    await request("/pitch/verify", { method: "POST", body: { password: "x" }, prefix: "" });
    expect(fetchMock.mock.calls[0][0]).toBe("/pitch/verify");
  });

  it("204 menghasilkan undefined", async () => {
    stubFetch(() => new Response(null, { status: 204 }));
    await expect(request("/links/1", { method: "DELETE" })).resolves.toBeUndefined();
  });
});

describe("error", () => {
  it("memetakan envelope error ke ApiError", async () => {
    stubFetch(() => fail(409, "ALIAS_TAKEN", "alias already taken"));
    const err = await request("/links", { method: "POST", body: {} }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 409, code: "ALIAS_TAKEN", message: "alias already taken" });
  });

  it("membaca Retry-After dan header rate limit", async () => {
    stubFetch(() =>
      fail(401, "UNAUTHORIZED", "wrong password", { "Retry-After": "120", "X-RateLimit-Limit": "5", "X-RateLimit-Remaining": "3", "X-RateLimit-Reset": "1800000000" }),
    );
    const err = (await request("/x/verify", { prefix: "", noRefresh: true }).catch((e) => e)) as ApiError;
    expect(err.retryAfter).toBe(120);
    expect(err.rateLimit).toEqual({ limit: 5, remaining: 3, reset: new Date(1800000000 * 1000) });
  });

  it("tanpa header rate limit, rateLimit kosong", async () => {
    stubFetch(() => fail(404, "NOT_FOUND", "nope"));
    const err = (await request("/links/9").catch((e) => e)) as ApiError;
    expect(err.rateLimit).toBeUndefined();
  });

  it("respons non-JSON dari proxy menjadi SERVICE_UNAVAILABLE untuk 5xx", async () => {
    stubFetch(() => new Response("<html>bad gateway</html>", { status: 502, statusText: "Bad Gateway" }));
    const err = (await request("/links").catch((e) => e)) as ApiError;
    expect(err).toMatchObject({ status: 502, code: "SERVICE_UNAVAILABLE" });
  });

  it("gagal jaringan menjadi NETWORK_ERROR", async () => {
    stubFetch(() => {
      throw new TypeError("Failed to fetch");
    });
    const err = (await request("/links").catch((e) => e)) as ApiError;
    expect(err).toMatchObject({ status: 0, code: "NETWORK_ERROR" });
  });
});

describe("refresh sesi", () => {
  it("401 memicu refresh sekali lalu mengulang request", async () => {
    let calls = 0;
    const fetchMock = stubFetch((url) => {
      if (url.endsWith("/auth/refresh")) return ok({});
      return ++calls === 1 ? fail(401, "UNAUTHORIZED", "expired") : ok({ id: "1" });
    });
    await expect(request("/auth/me")).resolves.toEqual({ id: "1" });
    expect(fetchMock.mock.calls.map(([u]) => u)).toEqual(["/api/v1/auth/me", "/api/v1/auth/refresh", "/api/v1/auth/me"]);
  });

  it("banyak 401 bersamaan hanya satu refresh (single-flight)", async () => {
    const seen = new Set<string>();
    const fetchMock = stubFetch((url) => {
      if (url.endsWith("/auth/refresh")) return ok({});
      if (seen.has(url)) return ok({});
      seen.add(url);
      return fail(401, "UNAUTHORIZED", "expired");
    });
    await Promise.all([request("/a"), request("/b"), request("/c")]);
    expect(fetchMock.mock.calls.filter(([u]) => String(u).endsWith("/auth/refresh"))).toHaveLength(1);
  });

  it("refresh gagal: 401 asli dilempar", async () => {
    stubFetch((url) => (url.endsWith("/auth/refresh") ? fail(401, "UNAUTHORIZED", "no session") : fail(401, "UNAUTHORIZED", "expired")));
    await expect(request("/auth/me")).rejects.toMatchObject({ status: 401 });
  });

  it("noRefresh: tidak mencoba refresh", async () => {
    const fetchMock = stubFetch(() => fail(401, "UNAUTHORIZED", "bad credentials"));
    await expect(request("/auth/login", { method: "POST", body: {}, noRefresh: true })).rejects.toMatchObject({ status: 401 });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
