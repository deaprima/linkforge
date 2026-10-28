import { describe, expect, it } from "vitest";
import { safeNext } from "@/lib/auth";
import { displayUrl, formatDate, formatDevice, formatNumber, isExpired, timeAgo } from "@/lib/format";

describe("format", () => {
  it("formatNumber memakai pemisah ribuan", () => expect(formatNumber(1280)).toBe("1,280"));
  it("formatDate: 31 Dec 2026", () => expect(formatDate("2026-12-31T10:00:00Z")).toBe("31 Dec 2026"));

  it("timeAgo", () => {
    const now = Date.UTC(2026, 0, 10, 12);
    const ago = (s: number) => timeAgo(new Date(now - s * 1000), now);
    expect(ago(2)).toBe("just now");
    expect(ago(12)).toBe("12 s ago");
    expect(ago(5 * 60)).toBe("5 min ago");
    expect(ago(3 * 3600)).toBe("3 h ago");
    expect(ago(3 * 86400)).toBe("7 Jan 2026");
  });

  it("displayUrl membuang skema dan trailing slash", () => {
    expect(displayUrl("https://github.com/deaprima/linkforge/")).toBe("github.com/deaprima/linkforge");
    expect(displayUrl("http://a.dev/x")).toBe("a.dev/x");
  });

  it("isExpired", () => {
    const now = Date.UTC(2026, 0, 1);
    expect(isExpired(null, now)).toBe(false);
    expect(isExpired("2025-12-31T00:00:00Z", now)).toBe(true);
    expect(isExpired("2026-02-01T00:00:00Z", now)).toBe(false);
  });

  it("formatDevice: label dikenal dan fallback", () => {
    expect(formatDevice("mobile")).toBe("Mobile");
    expect(formatDevice("smart-tv")).toBe("smart-tv");
  });
});

describe("safeNext (cegah open redirect)", () => {
  it.each([
    ["/app/links", "/app/links"],
    ["/app/links/1?x=2", "/app/links/1?x=2"],
    ["https://evil.com", "/app/dashboard"],
    ["//evil.com", "/app/dashboard"],
    ["/other", "/app/dashboard"],
    [null, "/app/dashboard"],
  ])("%s → %s", (input, expected) => expect(safeNext(input)).toBe(expected));
});
