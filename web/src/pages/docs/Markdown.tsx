import { Fragment, type ReactNode } from "react";
import { CodeBlock } from "@/pages/docs/CodeBlock";
import { parseBlocks } from "@/pages/docs/parse";

/*
 * Renderer Markdown mini untuk teks di api/openapi.yml: paragraf, daftar, tabel, blok kode, **tebal**, `kode`,
 * dan [tautan](url). Sengaja kecil dan tanpa dangerouslySetInnerHTML — hanya React element, jadi teks spec
 * tidak bisa menyisipkan HTML. Selain subset ini ditampilkan apa adanya sebagai teks.
 */

const INLINE = /(`[^`]+`|\*\*[^*]+\*\*|\[[^\]]+\]\([^)]+\))/g;

function safeHref(href: string): string | null {
  return /^(https?:\/\/|\/|#|mailto:)/.test(href) ? href : null;
}

export function Inline({ text }: { text: string }): ReactNode {
  return text.split(INLINE).map((part, i) => {
    if (part.startsWith("`") && part.endsWith("`") && part.length > 2) {
      return (
        <code key={i} className="bg-grid px-1.5 py-0.5 font-mono text-[0.85em] text-ink">
          {part.slice(1, -1)}
        </code>
      );
    }
    if (part.startsWith("**") && part.endsWith("**") && part.length > 4) {
      return (
        <strong key={i} className="font-semibold text-ink">
          {part.slice(2, -2)}
        </strong>
      );
    }
    const link = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(part);
    if (link) {
      const href = safeHref(link[2]);
      return href ? (
        <a key={i} href={href} className="font-semibold text-ink underline">
          {link[1]}
        </a>
      ) : (
        <Fragment key={i}>{link[1]}</Fragment>
      );
    }
    return <Fragment key={i}>{part}</Fragment>;
  });
}

export function Markdown({ text, className }: { text: string; className?: string }) {
  return (
    <div className={className ?? "flex flex-col gap-4 text-[17px] leading-[1.65] text-graphite"}>
      {parseBlocks(text).map((b, i) => {
        switch (b.kind) {
          case "p":
            return (
              <p key={i}>
                <Inline text={b.text} />
              </p>
            );
          case "ul":
          case "ol": {
            const List = b.kind === "ul" ? "ul" : "ol";
            return (
              <List key={i} className={`space-y-2 pl-6 ${b.kind === "ul" ? "list-disc" : "list-decimal"} marker:text-ink`}>
                {b.items.map((item, j) => (
                  <li key={j}>
                    <Inline text={item} />
                  </li>
                ))}
              </List>
            );
          }
          case "code":
            return <CodeBlock key={i} code={b.code} lang={b.lang} />;
          case "table":
            return (
              <div key={i} className="overflow-x-auto border border-ink bg-paper">
                <table className="w-full min-w-[460px] border-collapse text-left text-[15px]">
                  <thead>
                    <tr className="border-b border-ink">
                      {b.head.map((h, j) => (
                        <th key={j} className="px-4 py-2.5 text-[13px] font-semibold text-graphite">
                          <Inline text={h} />
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {b.rows.map((row, j) => (
                      <tr key={j} className="border-t border-ink/30 first:border-t-0 align-top">
                        {row.map((c, k) => (
                          <td key={k} className="px-4 py-3 text-ink">
                            <Inline text={c} />
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            );
        }
      })}
    </div>
  );
}
