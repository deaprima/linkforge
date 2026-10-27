import { cn } from "cn";
import closeDark from "@/assets/brand/icon-close-dark.svg";
import closeLight from "@/assets/brand/icon-close-light.svg";
import copyDark from "@/assets/brand/icon-copy-dark.svg";
import copyLight from "@/assets/brand/icon-copy-light.svg";
import lockDark from "@/assets/brand/icon-lock-dark.svg";
import lockLight from "@/assets/brand/icon-lock-light.svg";
import lockLgDark from "@/assets/brand/icon-lock-lg-dark.svg";
import lockLgLight from "@/assets/brand/icon-lock-lg-light.svg";
import moreDark from "@/assets/brand/icon-more-dark.svg";
import moreLight from "@/assets/brand/icon-more-light.svg";
import dotDark from "@/assets/brand/status-dot-dark.svg";
import dotLight from "@/assets/brand/status-dot-light.svg";

/*
 * Ikon dari Figma. Warna di-bake ke SVG, jadi setiap ikon punya varian light & dark
 * yang ditukar via CSS (tanpa JS, tanpa kedipan saat ganti tema).
 */

function Themed({ light, dark, className }: { light: string; dark: string; className: string }) {
  return (
    <span aria-hidden className={cn("relative inline-block shrink-0", className)}>
      <img alt="" src={light} className="absolute inset-0 size-full dark:hidden" />
      <img alt="" src={dark} className="absolute inset-0 hidden size-full dark:block" />
    </span>
  );
}

export const CopyIcon = ({ className }: { className?: string }) => <Themed light={copyLight} dark={copyDark} className={cn("size-4", className)} />;
export const MoreIcon = ({ className }: { className?: string }) => <Themed light={moreLight} dark={moreDark} className={cn("size-5", className)} />;
/** Gembok 40px untuk password gate (Figma 19:651), beda gambar dari LockIcon kecil. */
export const LockIconLarge = ({ className }: { className?: string }) => <Themed light={lockLgLight} dark={lockLgDark} className={cn("size-10", className)} />;
export const LockIcon =({ className }: { className?: string }) => <Themed light={lockLight} dark={lockDark} className={cn("size-3.5", className)} />;
export const StatusDot = ({ className }: { className?: string }) => <Themed light={dotLight} dark={dotDark} className={cn("size-2", className)} />;
export const CloseIcon = ({ className }: { className?: string }) => <Themed light={closeLight} dark={closeDark} className={cn("size-5", className)} />;
