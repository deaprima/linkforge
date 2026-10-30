# Deploying LinkForge

One VPS, Docker Compose, HTTPS by Caddy. Only ports 80 and 443 are open to the internet. PostgreSQL, Redis, Kafka, the
gRPC links between services and `/metrics` live on an internal network; Prometheus and Grafana listen on loopback only.

```
internet ──443──▶ edge (Caddy)  ──/api/*, /<alias>──▶ gateway ──gRPC──▶ auth · link · analytics
                     │                                   │                    │
                     └── /, /app/*, /assets/* (static web)└── redis             └── postgres · kafka
```

Everything is served from **one origin** (`https://linkforge.deaprima.my.id`). The session cookie is HttpOnly and
same-origin, and the Gateway redirects browsers to web pages on that same origin, so do not split the API and the web app
across different domains.

## 1. Prepare the server

- A VPS with Docker Engine and the Compose plugin. 2 GB RAM is the practical minimum (Kafka takes about 400 MB).
- DNS: an `A` (and `AAAA`, if you have IPv6) record for the domain pointing at the VPS. Caddy requests the certificate
  on first start, so DNS must already resolve or issuance fails.
- Firewall: allow only `22`, `80`, `443` (and `443/udp` for HTTP/3).

  ```bash
  sudo ufw default deny incoming && sudo ufw allow 22/tcp && sudo ufw allow 80/tcp && sudo ufw allow 443 && sudo ufw enable
  ```

  Docker publishes ports by editing iptables directly and **bypasses ufw**. That is fine here because the compose file
  publishes only the edge ports (and loopback for monitoring), but keep it that way: never add `ports:` to postgres,
  redis, kafka or the services.

## 2. Configure

```bash
git clone https://github.com/deaprima/linkforge.git && cd linkforge
cp .env.prod.example .env.prod
```

Fill in `.env.prod`. The stack refuses to start while a required secret is empty:

| Variable | Notes |
|---|---|
| `SITE_ADDRESS`, `ACME_EMAIL` | Public domain (no scheme) and the address Let's Encrypt contacts about the certificate |
| `DB_PASSWORD` | Used when the Postgres volume is created. Changing it later does not change an existing volume |
| `JWT_SECRET` | `openssl rand -base64 48`. Changing it signs everyone out |
| `SMTP_*` | **Required for password reset.** Without `SMTP_HOST` the reset email, including its link, is only written to the log |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | Optional. Leave both empty to turn Google sign-in off. Register `https://<domain>/api/v1/auth/oauth/google/callback` as the redirect URI |
| `GRAFANA_ADMIN_PASSWORD` | Grafana admin login |

`.env.prod` is git-ignored. Keep it that way, and restrict it: `chmod 600 .env.prod`.

## 3. First deploy

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
docker compose --env-file .env.prod -f docker-compose.prod.yml ps      # everything should become "healthy"
```

Check it from outside the server:

```bash
curl -sI https://linkforge.deaprima.my.id/ | head -1                       # 200
curl -s  https://linkforge.deaprima.my.id/healthz                          # {"status":"ok"}
curl -sI https://linkforge.deaprima.my.id/metrics | head -1                # 404, never public
```

Then register an account in the browser, create a link, open it, and confirm the click shows up in the dashboard
(that exercises Gateway, Auth, Link, Kafka and Analytics end to end).

Tip: write a shorter alias for the compose command, because every command needs both flags:

```bash
alias lf='docker compose --env-file .env.prod -f docker-compose.prod.yml'
```

## 4. Update and roll back

```bash
git pull
TAG=$(git rev-parse --short HEAD) lf up -d --build     # rebuilds only what changed, replaces containers
```

Images are tagged with `TAG`, so the previous version is still on the disk:

```bash
TAG=<previous-sha> lf up -d                            # roll back (no --build)
```

Schema changes run automatically on service start (GORM `AutoMigrate`). That only adds tables and columns, so a rollback
to the previous image is safe, but take a backup before any release that changes entities.

## 5. Backups

PostgreSQL is the only state worth keeping (Redis holds rate-limit counters, Kafka only buffers click events).

```bash
lf exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' | gzip > linkforge-$(date +%F).sql.gz
```

Run it from cron and copy the file off the server (a backup on the same disk does not survive the disk). Restore into an
empty database:

```bash
gunzip -c linkforge-2026-01-01.sql.gz | lf exec -T postgres sh -c 'psql -U "$POSTGRES_USER" "$POSTGRES_DB"'
```

Also keep `.env.prod` somewhere safe: without `JWT_SECRET` and `DB_PASSWORD` a restored database is hard to use.
The Caddy volume (`caddy_data`) holds the TLS certificate; losing it only means a new certificate is requested, but
Let's Encrypt rate-limits repeated issuance.

## 6. Monitoring

Prometheus and Grafana are bound to `127.0.0.1` on the server. Reach them through an SSH tunnel:

```bash
ssh -L 3001:127.0.0.1:3001 -L 9090:127.0.0.1:9090 user@your-vps
# Grafana:    http://localhost:3001   (admin / GRAFANA_ADMIN_PASSWORD) → LinkForge Overview
# Prometheus: http://localhost:9090   (Status → Targets: all four services should be UP)
```

## 7. Troubleshooting

| Symptom | Look at |
|---|---|
| Certificate is not issued | `lf logs edge`: DNS not pointing at the VPS yet, or ports 80/443 blocked |
| Everyone shares one rate limit / `429` for normal use | The Gateway sees the proxy's IP. `TRUSTED_PROXIES` must contain the Compose subnet (`172.30.0.0/24`, set in the compose file) |
| `503` on the site, `edge` is up | `lf ps` and `lf logs gateway`: a dependency is unhealthy. `/readyz` is not public; run `lf exec gateway wget -qO- http://127.0.0.1:8080/readyz` |
| Clicks are not counted | `lf logs link analytics kafka`. In Grafana, check *Kafka consumer lag* and *Click events lost before Kafka* |
| Password reset email never arrives | `SMTP_*` unset or wrong: `lf logs auth \| grep -i smtp` |
| Sessions reset after every deploy | `JWT_SECRET` changed, or cookies are being dropped because the site is opened over plain HTTP |

Logs are JSON on stdout (10 MB x 3 files per container). `lf logs -f gateway` follows the Gateway; every request line
has a `request_id`, which is also returned to the client in the `X-Request-ID` header.
