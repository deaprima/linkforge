import { useState } from "react";

/** Blok kode monospace dengan tombol Copy. Tidak ada syntax highlighting: sengaja polos seperti gaya lembar gambar teknik. */
export function CodeBlock({ code, lang, label }: { code: string; lang?: string; label?: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard diblokir: pengguna masih bisa menyeleksi teksnya */
    }
  }

  return (
    <figure className="min-w-0 border border-ink bg-paper">
      <figcaption className="flex items-center justify-between gap-3 border-b border-ink px-4 py-2 text-[13px] text-graphite">
        <span className="font-mono">{label ?? lang ?? "text"}</span>
        <button
          type="button"
          onClick={copy}
          aria-label={`Copy ${label ?? "code"}`}
          className="cursor-pointer font-semibold text-ink hover:underline focus-visible:outline-2 focus-visible:outline-signal"
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </figcaption>
      <pre className="overflow-x-auto p-4 font-mono text-[14px] leading-[1.7] text-ink">
        <code>{code}</code>
      </pre>
    </figure>
  );
}
