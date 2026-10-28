# LinkForge web app

React 19 + TypeScript + Vite. Landing page, dashboard, password gate and error pages, auth pages (including password
reset), and the API reference at `/app/docs`.

```bash
npm install
npm run dev        # http://localhost:5173, proxies /api to the Gateway on :8080
```

| Script | What it does |
|---|---|
| `npm run dev` | Dev server with hot reload |
| `npm run build` | Typecheck, then production build into `dist/` |
| `npm run lint` | ESLint |
| `npm test` | Vitest (`npm run test:watch` to watch) |

## Notes

- **Routes:** the app lives under `/app/*` because short links own the root of the domain (`/:alias`). `/` is the landing page.
- **Sessions:** the app never touches tokens in JavaScript. The Gateway sets HttpOnly cookies and the client sends
  `X-Session-Mode: cookie` so tokens stay out of response bodies.
- **API reference:** `src/pages/docs` renders `../api/openapi.yml` (parsed at build time by a small Vite plugin in
  `vite.config.ts`). Edit the spec and the page follows.
- **Config:** `VITE_SHORT_URL_BASE` is the origin short links are served from in dev; `VITE_SWAGGER_URL` adds a
  "Try it in Swagger UI" button to the docs page. See `.env.development`.
- **Theme and brand:** colour tokens, light and dark, live in `src/index.css`. Illustrations are exported from Figma;
  Kliko (the mascot) is `src/components/brand/Kliko.tsx`.
