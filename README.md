# LinkForge

[![CI](https://github.com/deaprima/linkforge/actions/workflows/ci.yml/badge.svg)](https://github.com/deaprima/linkforge/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-black.svg)](LICENSE)

**Shorten a link. See who clicks it.**

LinkForge is an open source URL shortener built as four Go microservices behind one HTTP gateway, with a React web app.
It does the usual things (custom aliases, expiry, passwords, QR codes, click analytics) and is written to show how the
parts around them are built: event streaming, circuit breakers, rate limiting and session security.

- **Short links** with custom aliases, expiry dates and optional passwords (brute-force protected).
- **QR codes** for any link, as PNG or SVG.
- **Analytics** by day, country, browser and device, plus a live click stream over WebSocket.
- **API keys** for scripts and CI, and **Google sign-in** next to email and password.
- **Password reset by email**, with single-use, hashed, expiring tokens.
- **Resilient by design**: every service call goes through a circuit breaker, and the redirect keeps working when analytics is down.
- **Documented**: [`api/openapi.yml`](api/openapi.yml) is the single source for the API reference page in the web app and for Swagger UI.

## Architecture

```
                    ┌──────────────┐
  Browser / CLI ───▶│   Gateway    │──── Redis (rate limits)
   HTTP + WebSocket │  (Gin, :8080)│
                    └──────┬───────┘
                 gRPC      │      gRPC           gRPC
              ┌────────────┼──────────────┐
              ▼            ▼              ▼
        ┌──────────┐ ┌──────────┐  ┌────────────┐
        │   Auth   │ │   Link   │  │ Analytics  │
        │  :50051  │ │  :50052  │  │   :50053   │
        └────┬─────┘ └────┬─────┘  └──────▲─────┘
             │            │  link.clicked │
             │            └──── Kafka ────┘
             ▼            ▼                ▼
        ┌────────────────────────────────────────┐
        │ PostgreSQL: schemas auth, link, analytics │
        └────────────────────────────────────────┘
```

| Service | Responsibility |
|---|---|
| **Gateway** | The only public entry point. Routing, the response envelope, rate limiting, circuit breakers, session cookies, QR code rendering, WebSocket fan-out. |
| **Auth** | Accounts, JWT access tokens, rotating refresh tokens with reuse detection, API keys, Google OAuth, password reset email. |
| **Link** | Links, aliases, expiry, passwords, redirect resolution. Publishes a `link.clicked` event to Kafka for every redirect. |
| **Analytics** | Consumes `link.clicked`, aggregates clicks by day, country, browser and device, and serves the stats. |

Services talk gRPC internally. Only the Gateway is reachable from outside, and each service owns its own database schema.

**Stack:** Go, Gin, gRPC, GORM, Kafka, PostgreSQL 16, Redis 7 · React 19, TypeScript, Vite, Tailwind CSS v4, TanStack Query, React Router 7.

## Quick start

You need [Go](https://go.dev/dl/) (the version in [`services/go.mod`](services/go.mod)), [Node.js](https://nodejs.org/) 22, and [Docker](https://docs.docker.com/get-docker/).

```bash
git clone https://github.com/deaprima/linkforge.git
cd linkforge
cp .env.example .env
```

Edit `.env`. For local development you need at least:

```ini
DB_PASSWORD=choose-something
JWT_SECRET=a-long-random-string
GOOGLE_CLIENT_ID=anything          # Auth refuses to start without it; real value only needed for Google sign-in
FRONTEND_URL=http://localhost:5173
COOKIE_SECURE=false                # the dev server runs over http
```

Start the infrastructure, then each service in its own terminal so the logs stay readable:

```bash
docker compose up -d postgres redis kafka

cd services
go run ./auth/cmd/server        # gRPC :50051
go run ./link/cmd/server        # gRPC :50052
go run ./analytics/cmd/server   # gRPC :50053
go run ./gateway/cmd/server     # HTTP :8080  (start this last)
```

```bash
cd web
npm install
npm run dev                     # http://localhost:5173
```

Open <http://localhost:5173/app/register> and create an account. The API reference is at <http://localhost:5173/app/docs>.

**Password reset emails.** Without `SMTP_HOST` the Auth service prints the reset link to its log instead of sending mail, which is enough to try the flow locally. See [Configuration](#configuration) for real delivery.

**Swagger UI.** `docker compose up -d swagger-ui` serves the same spec at <http://localhost:8090>.

## API

Everything the dashboard does is available over HTTP. The full, always up to date reference is the
[docs page in the web app](http://localhost:5173/app/docs) (built from [`api/openapi.yml`](api/openapi.yml)). A taste:

```bash
curl -X POST http://localhost:8080/api/v1/links \
  -H "X-API-Key: lf_sk_..." \
  -H "Content-Type: application/json" \
  -d '{"original_url": "https://example.com/a/very/long/url", "alias": "pitch"}'
```

Create an API key under **API keys** in the dashboard. Responses share one envelope (`{ "success": true, "data": ... }` or
`{ "success": false, "error": { "code": ..., "message": ... } }`); branch on `error.code`.

## Configuration

All configuration is environment variables, read from `.env` at the repository root (variables already set in the
environment win). [`.env.example`](.env.example) lists every option with comments. The ones you are most likely to touch:

| Variable | Used by | Purpose |
|---|---|---|
| `DB_*`, `REDIS_ADDR`, `KAFKA_BROKERS` | all | Infrastructure addresses |
| `JWT_SECRET`, `JWT_ACCESS_DURATION`, `JWT_REFRESH_DURATION` | Auth | Token signing and lifetime (15 minutes / 7 days by default) |
| `FRONTEND_URL`, `BASE_URL`, `CORS_ALLOWED_ORIGINS` | Gateway, Auth | Where the web app lives, and the public origin of short links |
| `COOKIE_SECURE` | Gateway | Keep `true` in production; `false` only for local http |
| `TRUSTED_PROXIES` | Gateway | **Required behind a reverse proxy**, otherwise every visitor shares one rate limit |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | Auth, Gateway | Google sign-in (leave the secret empty to turn it off) |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM` | Auth | Password reset email. Port 465 uses TLS, anything else uses STARTTLS |
| `VERIFY_LIMIT_*`, `FORGOT_LIMIT_*`, `RESET_LIMIT_*` | Gateway | Rate limits for password checks and password reset |
| `AUTH_LIMIT_*`, `PUBLIC_LIMIT_*`, `USER_LIMIT_*` | Gateway | General rate limits: sign-in per IP, redirects per IP, authenticated API per user |
| `LOG_FORMAT`, `LOG_LEVEL` | all | `json` (default) or `text`; `debug`, `info`, `warn`, `error` |
| `DB_DEBUG` | Auth, Link, Analytics | `true` prints every SQL statement with its values to the log. Development only; never in production |
| `*_METRICS_ADDR` | all | Internal `/metrics` and `/healthz` listener (`:9100` to `:9103`). Never expose it to the internet |
| `GEOIP_DB_PATH` | Analytics | Optional [GeoLite2-Country](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) database; without it countries show as `ZZ` |

## Monitoring

`docker compose up -d prometheus grafana` starts Prometheus on `127.0.0.1:9090` and Grafana on `127.0.0.1:3001`
(`admin` / `GRAFANA_ADMIN_PASSWORD`). Prometheus scrapes the four services on the host through
`host.docker.internal`, and Grafana comes with a provisioned **LinkForge Overview** dashboard: request rate and latency,
5xx ratio, gRPC latency and errors per dependency, circuit breaker state, Kafka consumer lag, and click events lost
before they reach Kafka.

## Project layout

```
api/openapi.yml        the public API contract (OpenAPI 3)
services/
  gateway/             HTTP gateway
  auth/  link/  analytics/
  shared/              protobuf definitions, circuit breaker, database helpers
web/                   React app: landing, dashboard, docs, auth pages
docker-compose.yml     PostgreSQL, Redis, Kafka, Swagger UI, Prometheus, Grafana
monitoring/            Prometheus config and provisioned Grafana dashboard
```

## Development

```bash
# Go (from services/)
go vet ./...
go test ./...

# Web (from web/)
npm run lint
npm test
npm run build
```

After changing a `.proto` file, regenerate the Go code with `make proto` (needs `protoc`, `protoc-gen-go` and `protoc-gen-go-grpc`).
Integration tests for Kafka and Postgres are skipped unless their environment is set; see the `Makefile`. Repository tests
run against a real Postgres in a throwaway schema, so any empty database works:

```bash
docker run -d --rm -p 55432:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=linkforge postgres:16-alpine
make cover TEST_DSN="postgres://postgres:test@localhost:55432/linkforge?sslmode=disable"
```

`make cover` measures the core layers of each service (handlers, services, repositories; not `main`, generated code or
entities) and fails below 70%. CI runs the same check against a Postgres service container.

The API is **contract first**: change [`api/openapi.yml`](api/openapi.yml) before the code, and the docs page updates itself.

## Status

LinkForge is a portfolio project in active development. The core product and web app are done. Still to come: broader
rate limiting, metrics and dashboards (Prometheus and Grafana), structured logging and a public deployment.

## Contributing

Issues and pull requests are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) first; it covers commit messages and the
branch flow. To report a security problem, follow [SECURITY.md](SECURITY.md) rather than opening a public issue.
Everyone taking part is expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE) © 2026 Dea Prima
