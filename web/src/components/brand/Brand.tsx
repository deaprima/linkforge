import { cn } from "cn";
import gridDark from "@/assets/brand/grid-dark.svg";
import gridLight from "@/assets/brand/grid-light.svg";
import markDark from "@/assets/brand/mark-dark.svg";
import markLight from "@/assets/brand/mark-light.svg";
import themeIconDark from "@/assets/brand/theme-icon-dark.svg";
import themeIconLight from "@/assets/brand/theme-icon-light.svg";
import { useTheme } from "@/lib/theme";

/** Logo (Figma 12:26): mark 26px + wordmark Martian Mono Bold 17px. */
export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn("inline-flex items-center gap-2", className)}>
      <span className="relative size-[26px] shrink-0">
        <img alt="" src={markLight} className="absolute inset-0 size-full dark:hidden" />
        <img alt="" src={markDark} className="absolute inset-0 hidden size-full dark:block" />
      </span>
      <span className="font-mono text-[17px] font-bold tracking-[-0.34px] whitespace-nowrap text-ink">linkforge</span>
    </span>
  );
}

/** Theme toggle (Figma 26:1077): kotak 36px, border 1.5px ink, ikon 18px. */
export function ThemeToggle({ className }: { className?: string }) {
  const { theme, toggle } = useTheme();
  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
      className={cn(
        "flex size-9 shrink-0 cursor-pointer items-center justify-center border-[1.5px] border-ink bg-paper",
        "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-signal",
        className,
      )}
    >
      <img alt="" src={themeIconLight} className="size-[18px] dark:hidden" />
      <img alt="" src={themeIconDark} className="hidden size-[18px] dark:block" />
    </button>
  );
}

/**
 * Latar grid 24px (Figma "Grid"). Aset 1440×900 dipakai apa adanya, di-tile & di-fix ke viewport
 * supaya tetap menutup layar di resolusi berapa pun.
 */
export function GridBackground() {
  return (
    <div aria-hidden className="pointer-events-none fixed inset-0 -z-10">
      <div className="absolute inset-0 bg-size-[1440px_900px] bg-top-left dark:hidden" style={{ backgroundImage: `url("${gridLight}")` }} />
      <div className="absolute inset-0 hidden bg-size-[1440px_900px] bg-top-left dark:block" style={{ backgroundImage: `url("${gridDark}")` }} />
    </div>
  );
}
