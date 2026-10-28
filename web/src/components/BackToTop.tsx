import { cn } from "cn";
import { useEffect, useState } from "react";
import { buttonClass } from "@/components/ui/primitives";

/** Tombol kembali ke atas untuk halaman panjang (landing, docs): muncul setelah halaman digulir cukup jauh. */
export function BackToTop() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const onScroll = () => setVisible(window.scrollY > 600);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  if (!visible) return null;

  function toTop() {
    const reduce = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    window.scrollTo({ top: 0, behavior: reduce ? "auto" : "smooth" });
  }

  return (
    <button type="button" onClick={toTop} className={cn(buttonClass({ variant: "outline" }, "fixed right-4 bottom-4 z-20 gap-2 sm:right-8 sm:bottom-8"))}>
      <span aria-hidden>↑</span>
      Back to top
    </button>
  );
}
