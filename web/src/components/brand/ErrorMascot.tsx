import { cn } from "cn";
import expiredDark from "@/assets/mascot/errors/expired-dark.svg";
import expiredLight from "@/assets/mascot/errors/expired.svg";
import fuseDark from "@/assets/mascot/errors/fuse-dark.svg";
import fuseLight from "@/assets/mascot/errors/fuse.svg";
import lostDark from "@/assets/mascot/errors/lost-dark.svg";
import lostLight from "@/assets/mascot/errors/lost.svg";

/*
 * Maskot halaman error (Figma: Components → Mascot/404 Lost, 410 Expired, 503 Fuse).
 * Diekspor utuh dari Figma sebagai satu SVG per varian (teks "?", "EXPIRED" sudah jadi outline),
 * jadi cukup <img>. Warna di-bake, light & dark ditukar via CSS seperti ikon lain.
 */

const art = {
  lost: { light: lostLight, dark: lostDark },
  expired: { light: expiredLight, dark: expiredDark },
  fuse: { light: fuseLight, dark: fuseDark },
} as const;

export type ErrorMascotKind = keyof typeof art;

export function ErrorMascot({ kind, className }: { kind: ErrorMascotKind; className?: string }) {
  return (
    <span aria-hidden className={cn("relative inline-block shrink-0", className)}>
      <img alt="" src={art[kind].light} className="absolute inset-0 size-full dark:hidden" />
      <img alt="" src={art[kind].dark} className="absolute inset-0 hidden size-full dark:block" />
    </span>
  );
}
