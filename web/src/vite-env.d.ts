/// <reference types="vite/client" />

/** Hasil parse api/openapi.yml (lihat plugin "linkforge-openapi" di vite.config.ts). */
declare module "virtual:openapi" {
  const spec: unknown;
  export default spec;
}

interface ImportMetaEnv {
  /** URL Swagger UI (mis. http://localhost:8090). Kosong = tombol "Try it" disembunyikan. */
  readonly VITE_SWAGGER_URL?: string;
}
