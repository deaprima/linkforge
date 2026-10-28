import { cn } from "cn";

/*
 * Kliko, maskot LinkForge (Figma "Mascot — Brand", 40:378). Satu karakter, sembilan pose; pose dipilih dari situasinya
 * (lihat "Where Kliko appears"). Tiap pose diekspor dari Figma sebagai SVG 250×225 untuk light dan dark, warnanya
 * di-bake, jadi variannya ditukar via CSS seperti ikon lain. Atur ukuran lewat lebar saja (rasio dijaga; jangan
 * atur lebar dan tinggi sekaligus, jangan diputar atau diberi balon bicara — aturan "Do this, not that").
 */

const art = import.meta.glob<string>("../../assets/mascot/kliko/*.svg", { eager: true, query: "?url", import: "default" });
const url = (name: string) => art[`../../assets/mascot/kliko/${name}.svg`];

export const KLIKO_POSES = ["hero", "success", "empty", "lock", "key", "chart", "lost", "expired", "fuse"] as const;
export type KlikoPose = (typeof KLIKO_POSES)[number];

const label: Record<KlikoPose, string> = {
  hero: "Kliko waving hello",
  success: "Kliko celebrating",
  empty: "Kliko with an empty box",
  lock: "Kliko holding a padlock",
  key: "Kliko holding a key",
  chart: "Kliko next to a bar chart",
  lost: "Kliko looking lost",
  expired: "Kliko asleep",
  fuse: "Kliko with a blown fuse",
};

export function Kliko({ pose, className, decorative = true }: { pose: KlikoPose; className?: string; decorative?: boolean }) {
  return (
    <span
      role={decorative ? undefined : "img"}
      aria-label={decorative ? undefined : label[pose]}
      aria-hidden={decorative || undefined}
      className={cn("relative block aspect-[250/225] shrink-0", className)}
    >
      <img alt="" src={url(pose)} className="absolute inset-0 size-full dark:hidden" />
      <img alt="" src={url(`${pose}-dark`)} className="absolute inset-0 hidden size-full dark:block" />
    </span>
  );
}
