/** Memecah teks Markdown (subset yang dipakai api/openapi.yml) menjadi blok. Lihat Markdown.tsx untuk render-nya. */

export type Block =
  | { kind: "p"; text: string }
  | { kind: "ul" | "ol"; items: string[] }
  | { kind: "code"; lang: string; code: string }
  | { kind: "table"; head: string[]; rows: string[][] };

const cells = (line: string) =>
  line
    .trim()
    .replace(/^\||\|$/g, "")
    .split("|")
    .map((c) => c.trim());

export function parseBlocks(markdown: string): Block[] {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  const blocks: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (!line.trim()) {
      i++;
      continue;
    }

    const fence = /^```(\w*)\s*$/.exec(line.trim());
    if (fence) {
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith("```")) code.push(lines[i++]);
      i++; // penutup ```
      blocks.push({ kind: "code", lang: fence[1], code: code.join("\n") });
      continue;
    }

    if (line.trim().startsWith("|") && /^\s*\|?\s*:?-{3,}/.test(lines[i + 1] ?? "")) {
      const head = cells(line);
      i += 2;
      const rows: string[][] = [];
      while (i < lines.length && lines[i].trim().startsWith("|")) rows.push(cells(lines[i++]));
      blocks.push({ kind: "table", head, rows });
      continue;
    }

    const list = /^(\s*)(-|\d+\.)\s+/.exec(line);
    if (list) {
      const ordered = list[2] !== "-";
      const items: string[] = [];
      while (i < lines.length && /^(\s*)(-|\d+\.)\s+/.test(lines[i])) {
        items.push(lines[i++].replace(/^(\s*)(-|\d+\.)\s+/, ""));
        // baris lanjutan yang menjorok milik item yang sama
        while (i < lines.length && /^\s{2,}\S/.test(lines[i]) && !/^(\s*)(-|\d+\.)\s+/.test(lines[i])) items[items.length - 1] += " " + lines[i++].trim();
      }
      blocks.push({ kind: ordered ? "ol" : "ul", items });
      continue;
    }

    const para: string[] = [];
    while (i < lines.length && lines[i].trim() && !lines[i].trim().startsWith("```") && !/^(\s*)(-|\d+\.)\s+/.test(lines[i]) && !lines[i].trim().startsWith("|")) {
      para.push(lines[i++].trim());
    }
    blocks.push({ kind: "p", text: para.join(" ") });
  }
  return blocks;
}
