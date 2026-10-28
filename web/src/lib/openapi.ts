import rawSpec from "virtual:openapi";

/*
 * Membaca api/openapi.yml (di-parse saat build oleh plugin Vite) menjadi bentuk yang mudah dirender halaman docs:
 * operasi dikelompokkan per tag, $ref di-resolve, dan contoh request/response/curl dibuat dari schema.
 * Tidak ada dependensi runtime selain data spec itu sendiri.
 */

export type Schema = {
  $ref?: string;
  type?: string;
  format?: string;
  description?: string;
  example?: unknown;
  enum?: unknown[];
  default?: unknown;
  nullable?: boolean;
  required?: string[];
  properties?: Record<string, Schema>;
  items?: Schema;
  allOf?: Schema[];
  minLength?: number;
  maxLength?: number;
  minimum?: number;
  maximum?: number;
};

type MediaType = { schema?: Schema; example?: unknown; examples?: Record<string, { summary?: string; value?: unknown }> };
type Header = { description?: string; schema?: Schema };
type ResponseObject = { $ref?: string; description?: string; headers?: Record<string, Header>; content?: Record<string, MediaType> };
type Parameter = { name: string; in: "path" | "query" | "header"; required?: boolean; description?: string; schema?: Schema };

type RawOperation = {
  tags?: string[];
  summary?: string;
  description?: string;
  operationId?: string;
  security?: Record<string, string[]>[];
  servers?: { url: string; description?: string }[];
  parameters?: Parameter[];
  requestBody?: { required?: boolean; content?: Record<string, MediaType> };
  responses?: Record<string, ResponseObject>;
};

type Spec = {
  info: { title: string; description: string; version: string; license?: { name: string; url?: string }; contact?: { name: string; url?: string } };
  servers: { url: string; description?: string }[];
  tags?: { name: string; description?: string }[];
  paths: Record<string, Record<string, RawOperation>>;
  components: { schemas: Record<string, Schema>; responses: Record<string, ResponseObject> };
};

const spec = rawSpec as Spec;

export const info = spec.info;
export const servers = spec.servers;

const METHODS = ["get", "post", "put", "patch", "delete"] as const;
export type Method = Uppercase<(typeof METHODS)[number]>;

export type Auth = "public" | "api-key" | "bearer" | "cookie";

export type ResponseDoc = {
  status: string;
  description: string;
  headers: { name: string; description?: string }[];
  /** Satu atau lebih contoh body; kosong untuk 204 / non-JSON. */
  examples: { title?: string; body: unknown }[];
};

export type Operation = {
  id: string;
  method: Method;
  path: string;
  summary: string;
  description: string;
  tag: string;
  auth: Auth[];
  /** Server khusus operasi ini (mis. redirect di root), kalau berbeda dari server global. */
  servers?: { url: string; description?: string }[];
  parameters: Parameter[];
  body?: { required: boolean; schema: Schema; example: unknown };
  responses: ResponseDoc[];
};

function lookup<T>(ref: string): T {
  // Hanya "#/components/<bagian>/<nama>" yang dipakai spec ini
  const [, , section, name] = ref.split("/");
  const bucket = (spec.components as unknown as Record<string, Record<string, T>>)[section];
  const found = bucket?.[name];
  if (!found) throw new Error(`Unresolved $ref ${ref}`);
  return found;
}

/** Resolve $ref dan gabungkan allOf menjadi satu schema datar. */
export function resolveSchema(schema: Schema | undefined): Schema {
  if (!schema) return {};
  if (schema.$ref) return resolveSchema(lookup<Schema>(schema.$ref));
  if (schema.allOf) {
    return schema.allOf.map(resolveSchema).reduce<Schema>(
      (acc, part) => ({
        ...acc,
        ...part,
        properties: { ...acc.properties, ...part.properties },
        required: [...(acc.required ?? []), ...(part.required ?? [])],
      }),
      {},
    );
  }
  return schema;
}

const FORMAT_SAMPLES: Record<string, unknown> = {
  uuid: "3f2b8c1a-9d4e-4b7a-8c11-5e6f7a8b9c0d",
  email: "dea@example.com",
  uri: "https://example.com",
  "date-time": "2026-07-08T07:00:00Z",
  date: "2026-07-08",
};

/** Contoh nilai dari schema: pakai `example` kalau ada, kalau tidak bangun dari tipe. */
export function sampleFor(schema: Schema | undefined, depth = 0): unknown {
  const s = resolveSchema(schema);
  if (s.example !== undefined) return s.example;
  if (s.enum?.length) return s.enum[0];
  if (s.default !== undefined) return s.default;
  if (depth > 6) return null;
  switch (s.type) {
    case "object": {
      const out: Record<string, unknown> = {};
      for (const [key, value] of Object.entries(s.properties ?? {})) out[key] = sampleFor(value, depth + 1);
      return out;
    }
    case "array":
      return [sampleFor(s.items, depth + 1)];
    case "integer":
    case "number":
      return 1;
    case "boolean":
      return true;
    default:
      return (s.format && FORMAT_SAMPLES[s.format]) ?? "string";
  }
}

/** Ringkasan tipe untuk tabel field, mis. "string · email", "integer", "string[]". */
export function typeLabel(schema: Schema | undefined): string {
  const s = resolveSchema(schema);
  if (s.type === "array") return `${typeLabel(s.items)}[]`;
  const base = s.type ?? "object";
  const extra = s.enum ? s.enum.map(String).join(" | ") : s.format;
  return extra ? `${base} · ${extra}` : base;
}

/** Batasan ("8–72 characters", "1–100", default) dalam satu string, atau null. */
export function constraintLabel(schema: Schema | undefined): string | null {
  const s = resolveSchema(schema);
  const parts: string[] = [];
  if (s.minLength !== undefined && s.maxLength !== undefined) parts.push(`${s.minLength}–${s.maxLength} characters`);
  else if (s.maxLength !== undefined) parts.push(`up to ${s.maxLength} characters`);
  else if (s.minLength !== undefined) parts.push(`at least ${s.minLength} characters`);
  if (s.minimum !== undefined || s.maximum !== undefined) parts.push(`${s.minimum ?? "−∞"}–${s.maximum ?? "∞"}`);
  if (s.default !== undefined) parts.push(`default ${String(s.default)}`);
  if (s.nullable) parts.push("nullable");
  return parts.length ? parts.join(", ") : null;
}

export type Field = { name: string; type: string; required: boolean; description?: string; constraint: string | null };

export function fieldsOf(schema: Schema | undefined): Field[] {
  const s = resolveSchema(schema);
  const required = new Set(s.required);
  return Object.entries(s.properties ?? {}).map(([name, prop]) => ({
    name,
    type: typeLabel(prop),
    required: required.has(name),
    description: resolveSchema(prop).description ?? prop.description,
    constraint: constraintLabel(prop),
  }));
}

function authOf(op: RawOperation): Auth[] {
  if (!op.security || op.security.length === 0) return ["public"];
  const kinds: Record<string, Auth> = { ApiKeyAuth: "api-key", BearerAuth: "bearer", CookieAuth: "cookie" };
  return op.security.flatMap((req) => Object.keys(req).map((k) => kinds[k])).filter(Boolean);
}

function responseDocs(op: RawOperation): ResponseDoc[] {
  return Object.entries(op.responses ?? {}).map(([status, raw]) => {
    const res = raw.$ref ? lookup<ResponseObject>(raw.$ref) : raw;
    const json = res.content?.["application/json"];
    const examples: ResponseDoc["examples"] = [];
    if (json?.examples) {
      for (const ex of Object.values(json.examples)) examples.push({ title: ex.summary, body: ex.value });
    } else if (json?.example !== undefined) {
      examples.push({ body: json.example });
    } else if (json?.schema) {
      examples.push({ body: sampleFor(json.schema) });
    }
    return {
      status,
      description: res.description ?? "",
      headers: Object.entries(res.headers ?? {}).map(([name, h]) => ({ name, description: h.description })),
      examples,
    };
  });
}

function toOperation(path: string, method: (typeof METHODS)[number], op: RawOperation): Operation {
  const jsonBody = op.requestBody?.content?.["application/json"];
  return {
    id: op.operationId ?? `${method}-${path}`,
    method: method.toUpperCase() as Method,
    path,
    summary: op.summary ?? "",
    description: op.description ?? "",
    tag: op.tags?.[0] ?? "Other",
    auth: authOf(op),
    servers: op.servers,
    parameters: op.parameters ?? [],
    body: jsonBody?.schema ? { required: op.requestBody?.required ?? false, schema: jsonBody.schema, example: sampleFor(jsonBody.schema) } : undefined,
    responses: responseDocs(op),
  };
}

export type TagGroup = { name: string; description: string; operations: Operation[] };

/** Operasi dikelompokkan per tag, urutan tag mengikuti spec; urutan operasi mengikuti urutan di file. */
export function groups(): TagGroup[] {
  const ops: Operation[] = [];
  for (const [path, item] of Object.entries(spec.paths)) {
    for (const method of METHODS) {
      if (item[method]) ops.push(toOperation(path, method, item[method]));
    }
  }
  return (spec.tags ?? [])
    .map((t) => ({ name: t.name, description: t.description ?? "", operations: ops.filter((o) => o.tag === t.name) }))
    .filter((g) => g.operations.length > 0);
}

const AUTH_HEADERS: Record<Auth, string | null> = {
  "api-key": '-H "X-API-Key: YOUR_API_KEY"',
  bearer: '-H "Authorization: Bearer YOUR_ACCESS_TOKEN"',
  cookie: null,
  public: null,
};

/** Perintah curl untuk satu operasi, memakai metode autentikasi pertama yang bisa ditulis sebagai header. */
export function curlFor(op: Operation, baseUrl: string): string {
  const url = `${baseUrl}${op.path.replace(/\{(\w+)\}/g, (_, name: string) => {
    const param = op.parameters.find((p) => p.name === name);
    return String(sampleFor(param?.schema) ?? name);
  })}`;
  const query = op.parameters
    .filter((p) => p.in === "query" && p.required)
    .map((p) => `${p.name}=${String(sampleFor(p.schema))}`)
    .join("&");

  const verb = op.method === "GET" ? "" : `-X ${op.method} `;
  const lines = [`curl ${verb}"${url}${query ? `?${query}` : ""}"`];
  const authHeader = op.auth.map((a) => AUTH_HEADERS[a]).find(Boolean);
  if (authHeader) lines.push(`  ${authHeader}`);
  if (op.body) {
    lines.push('  -H "Content-Type: application/json"');
    lines.push(`  -d '${JSON.stringify(op.body.example)}'`);
  }
  return lines.join(" \\\n");
}

/** Server dasar yang dipakai untuk operasi ini (server operasi menang atas server global). */
export function baseUrlFor(op: Operation, env: "production" | "local"): string {
  const list = op.servers ?? servers;
  const wantLocal = env === "local";
  const match = list.find((s) => /localhost|127\.0\.0\.1/.test(s.url) === wantLocal) ?? list[0];
  return match.url.replace(/\/$/, "");
}

export const slug = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");

/** Pecah deskripsi spec per "## Judul": bagian sebelum judul pertama jadi pengantar. */
export function splitIntro(markdown: string): { lead: string; sections: { id: string; title: string; body: string }[] } {
  const [lead, ...rest] = markdown.split(/^## /m);
  return {
    lead: lead.trim(),
    sections: rest.map((chunk) => {
      const [title, ...body] = chunk.split("\n");
      return { id: slug(title), title: title.trim(), body: body.join("\n").trim() };
    }),
  };
}
