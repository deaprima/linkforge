import { describe, expect, it } from "vitest";
import { baseUrlFor, curlFor, fieldsOf, groups, resolveSchema, sampleFor, type Operation } from "@/lib/openapi";

const all = () => groups().flatMap((g) => g.operations);
const byId = (id: string): Operation => {
  const op = all().find((o) => o.id === id);
  if (!op) throw new Error(`operation ${id} not in spec`);
  return op;
};

describe("api/openapi.yml", () => {
  it("setiap operasi punya id unik, ringkasan, dan minimal satu respons", () => {
    const ids = all().map((o) => o.id);
    expect(new Set(ids).size).toBe(ids.length);
    for (const op of all()) {
      expect(op.summary, op.id).not.toBe("");
      expect(op.responses.length, op.id).toBeGreaterThan(0);
    }
  });

  it("semua $ref bisa di-resolve (groups() akan melempar kalau ada yang putus)", () => {
    expect(() => groups()).not.toThrow();
    for (const op of all()) {
      if (op.body) expect(Object.keys(resolveSchema(op.body.schema).properties ?? {}).length, op.id).toBeGreaterThan(0);
    }
  });

  it("endpoint lupa password terdokumentasi dan publik", () => {
    const forgot = byId("forgotPassword");
    const reset = byId("resetPassword");
    expect([forgot.method, forgot.path, forgot.auth]).toEqual(["POST", "/auth/forgot-password", ["public"]]);
    expect([reset.method, reset.path, reset.auth]).toEqual(["POST", "/auth/reset-password", ["public"]]);
    expect(reset.responses.map((r) => r.status)).toContain("400");
    expect(JSON.stringify(reset.responses)).toContain("INVALID_RESET_TOKEN");
  });

  it("tag mengikuti urutan spec dan tidak ada operasi tanpa tag terdaftar", () => {
    const names = groups().map((g) => g.name);
    expect(names.slice(0, 3)).toEqual(["Auth", "Auth - OAuth", "Auth - API Keys"]);
    expect(all().every((o) => names.includes(o.tag))).toBe(true);
  });
});

describe("sampleFor", () => {
  it("memakai example bila ada, kalau tidak membangun dari tipe/format", () => {
    expect(sampleFor({ type: "string", example: "pitch" })).toBe("pitch");
    expect(sampleFor({ type: "string", format: "email" })).toBe("dea@example.com");
    expect(sampleFor({ type: "integer" })).toBe(1);
    expect(sampleFor({ type: "array", items: { type: "boolean" } })).toEqual([true]);
    expect(sampleFor({ type: "string", enum: ["png", "svg"] })).toBe("png");
  });

  it("menggabungkan allOf", () => {
    const s = resolveSchema({ allOf: [{ type: "object", properties: { a: { type: "string" } } }, { type: "object", properties: { b: { type: "integer" } } }] });
    expect(Object.keys(s.properties ?? {})).toEqual(["a", "b"]);
  });

  it("contoh request createLink hanya berisi field dari schema", () => {
    expect(Object.keys(byId("createLink").body?.example as object)).toEqual(["original_url", "alias", "expired_at", "password"]);
  });
});

describe("fieldsOf", () => {
  it("menandai field wajib dan batasannya", () => {
    const register = fieldsOf(byId("registerUser").body?.schema);
    const password = register.find((f) => f.name === "password");
    expect(password).toMatchObject({ type: "string", required: true });
    expect(password?.constraint).toBe("8–72 characters");
  });
});

describe("curlFor", () => {
  it("API key + body untuk operasi tulis", () => {
    const curl = curlFor(byId("createLink"), "https://linkforge.deaprima.my.id/api/v1");
    expect(curl.startsWith('curl -X POST "https://linkforge.deaprima.my.id/api/v1/links"')).toBe(true);
    expect(curl).toContain('-H "X-API-Key: YOUR_API_KEY"');
    expect(curl).toContain('-H "Content-Type: application/json"');
    expect(curl).toContain('"original_url"');
  });

  it("GET tanpa -X dan path param diganti nilai contoh", () => {
    const curl = curlFor(byId("getLink"), "http://localhost:8080/api/v1");
    expect(curl.startsWith('curl "http://localhost:8080/api/v1/links/')).toBe(true);
    expect(curl).not.toContain("{id}");
    expect(curl).not.toContain("-X");
  });

  it("endpoint publik tidak memakai header autentikasi", () => {
    const curl = curlFor(byId("loginUser"), "http://localhost:8080/api/v1");
    expect(curl).not.toContain("X-API-Key");
    expect(curl).not.toContain("Authorization");
  });

  it("endpoint akun (bearer/cookie) memakai Authorization", () => {
    expect(curlFor(byId("getMe"), "http://localhost:8080/api/v1")).toContain('Authorization: Bearer YOUR_ACCESS_TOKEN');
  });
});

describe("baseUrlFor", () => {
  it("API biasa memakai prefix /api/v1", () => {
    expect(baseUrlFor(byId("listLinks"), "production")).toBe("https://linkforge.deaprima.my.id/api/v1");
    expect(baseUrlFor(byId("listLinks"), "local")).toBe("http://localhost:8080/api/v1");
  });

  it("redirect di root domain, tanpa /api/v1", () => {
    expect(baseUrlFor(byId("resolveRedirect"), "production")).toBe("https://linkforge.deaprima.my.id");
    expect(baseUrlFor(byId("verifyLinkPassword"), "local")).toBe("http://localhost:8080");
  });
});
