import { cn } from "cn";
import { useState } from "react";
import { baseUrlFor, constraintLabel, curlFor, fieldsOf, typeLabel, type Auth, type Field, type Method, type Operation, type ResponseDoc } from "@/lib/openapi";
import { CodeBlock } from "@/pages/docs/CodeBlock";
import { Inline, Markdown } from "@/pages/docs/Markdown";

const METHOD_STYLE: Record<Method, string> = {
  GET: "border-signal-ink bg-signal-soft text-signal-ink",
  POST: "border-ink bg-ink text-paper",
  PUT: "border-ink bg-paper text-ink",
  PATCH: "border-ink bg-paper text-ink",
  DELETE: "border-stop bg-paper text-stop",
};

export function MethodBadge({ method, className }: { method: Method; className?: string }) {
  return <span className={cn("inline-flex w-16 shrink-0 justify-center border px-1.5 py-1 font-mono text-xs font-semibold", METHOD_STYLE[method], className)}>{method}</span>;
}

const AUTH_LABEL: Record<Auth, string> = {
  public: "Public",
  "api-key": "API key",
  bearer: "Bearer token",
  cookie: "Session cookie",
};

function AuthChips({ auth }: { auth: Auth[] }) {
  return (
    <ul className="flex flex-wrap gap-1.5" aria-label="Accepted authentication">
      {auth.map((a) => (
        <li key={a} className={cn("border px-2 py-0.5 text-xs", a === "public" ? "border-signal-ink text-signal-ink" : "border-ink text-ink")}>
          {AUTH_LABEL[a]}
        </li>
      ))}
    </ul>
  );
}

const statusTone = (status: string) => (status.startsWith("2") || status === "101" ? "border-signal-ink text-signal-ink" : status.startsWith("3") ? "border-ink text-ink" : "border-stop text-stop");

function Heading({ children }: { children: string }) {
  return <h4 className="text-[13px] font-semibold tracking-wide text-graphite uppercase">{children}</h4>;
}

/** Daftar field: nama + tipe di kiri, penjelasan di kanan; di layar sempit ditumpuk supaya penjelasan tidak terjepit. */
function FieldTable({ rows, label }: { rows: Field[]; label: string }) {
  return (
    <ul aria-label={label} className="border border-ink">
      {rows.map((f) => (
        <li key={f.name} className="flex flex-col gap-1 border-t border-ink/30 px-4 py-3 first:border-t-0 sm:grid sm:grid-cols-[210px_1fr] sm:gap-5">
          <div className="min-w-0">
            <code className="font-mono text-[15px] font-medium break-all text-ink">{f.name}</code>
            <p className="mt-0.5 text-[13px] text-graphite">
              {f.type}
              {f.required ? <span className="text-stop"> · required</span> : null}
            </p>
          </div>
          <div className="min-w-0 text-[15px] leading-[1.55] text-graphite">
            {f.description ? <Inline text={f.description} /> : null}
            {f.constraint ? <p className="mt-0.5 text-[13px]">{f.constraint}</p> : null}
          </div>
        </li>
      ))}
    </ul>
  );
}

function Response({ res, open }: { res: ResponseDoc; open: boolean }) {
  const [show, setShow] = useState(open);
  const hasBody = res.examples.length > 0 || res.headers.length > 0;
  return (
    <li className="border border-ink">
      <button
        type="button"
        aria-expanded={show}
        disabled={!hasBody}
        onClick={() => setShow((s) => !s)}
        className="flex w-full cursor-pointer items-center gap-3 px-4 py-3 text-left disabled:cursor-default focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-signal"
      >
        <span className={cn("border px-2 py-0.5 font-mono text-sm font-semibold", statusTone(res.status))}>{res.status}</span>
        <span className="min-w-0 flex-1 text-[15px] leading-[1.4] text-ink">
          <Inline text={res.description} />
        </span>
        {hasBody && <span aria-hidden className="shrink-0 font-mono text-xs text-graphite">{show ? "−" : "+"}</span>}
      </button>
      {show && hasBody && (
        <div className="flex flex-col gap-3 border-t border-ink p-4">
          {res.headers.length > 0 && (
            <p className="text-sm leading-[1.6] text-graphite">
              Headers: {res.headers.map((h) => <code key={h.name} className="mr-2 font-mono text-ink">{h.name}</code>)}
            </p>
          )}
          {res.examples.map((ex, i) => (
            <CodeBlock key={i} code={JSON.stringify(ex.body, null, 2)} label={ex.title ?? "Response"} lang="json" />
          ))}
        </div>
      )}
    </li>
  );
}

export function OperationCard({ op, env }: { op: Operation; env: "production" | "local" }) {
  const baseUrl = baseUrlFor(op, env);
  const path = op.parameters.filter((p) => p.in === "path");
  const query = op.parameters.filter((p) => p.in === "query");
  const toField = (p: (typeof path)[number]): Field => ({
    name: p.name,
    type: typeLabel(p.schema),
    required: Boolean(p.required),
    description: p.description,
    constraint: constraintLabel(p.schema),
  });
  const defaultOpen = op.responses.find((r) => r.status.startsWith("2"))?.status;

  return (
    <section id={op.id} aria-labelledby={`${op.id}-title`} className="scroll-mt-24 border border-ink bg-paper">
      <header className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-ink px-4 py-3.5 sm:px-5">
        <MethodBadge method={op.method} />
        <code className="min-w-0 flex-1 font-mono text-base font-medium break-all text-ink">{op.path}</code>
        <AuthChips auth={op.auth} />
      </header>

      <div className="flex flex-col gap-6 p-4 sm:p-6">
        <div className="flex flex-col gap-2.5">
          <h3 id={`${op.id}-title`} className="text-2xl font-semibold tracking-[-0.3px] text-ink">
            {op.summary}
          </h3>
          {op.description && <Markdown text={op.description} />}
        </div>

        {path.length > 0 && (
          <div className="flex flex-col gap-2">
            <Heading>Path parameters</Heading>
            <FieldTable rows={path.map(toField)} label="Path parameters" />
          </div>
        )}
        {query.length > 0 && (
          <div className="flex flex-col gap-2">
            <Heading>Query parameters</Heading>
            <FieldTable rows={query.map(toField)} label="Query parameters" />
          </div>
        )}
        {op.body && (
          <div className="flex flex-col gap-2">
            <Heading>Request body</Heading>
            <FieldTable rows={fieldsOf(op.body.schema)} label="Request body" />
          </div>
        )}

        <CodeBlock code={curlFor(op, baseUrl)} label="curl" lang="bash" />

        <div className="flex flex-col gap-2">
          <Heading>Responses</Heading>
          <ul className="flex flex-col gap-2">
            {op.responses.map((r) => (
              <Response key={r.status} res={r} open={r.status === defaultOpen} />
            ))}
          </ul>
        </div>
      </div>
    </section>
  );
}
