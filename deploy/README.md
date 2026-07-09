# Deploy

Docker Compose definitions for the shop-server production deployment.

**Full step-by-step setup guide (VM or bare metal, from a clean machine): [DEPLOY.md](DEPLOY.md).**

## When to use this

| Environment | Postgres source | Use this compose? |
| --- | --- | --- |
| **Dev** (this PC, Postgres already on host)     | Host Postgres at `localhost:5432` | No — point `DATABASE_URL` at the host PG in `.env` and run the Go API natively (`go run ./cmd/api`). |
| **Prod** (shop server, clean install)           | Dockerised Postgres in this stack | Yes — follow [DEPLOY.md](DEPLOY.md). |

The compose file is committed so the prod deploy is reproducible. It's safe to *not* use it locally as long as `DATABASE_URL` in `.env` points at whatever Postgres you do use.

## Prerequisites

- Docker Engine + Compose v2
- A populated `.env` at the **repo root** (see `../.env.example`).

## Quick reference

All commands assume CWD = `deploy/`. The compose file references `../.env`.

```bash
docker compose up -d                 # bring up the full stack
docker compose ps                    # status / health
docker compose logs -f api          # tail a service (api, caddy, cloudflared, postgres, web)
docker compose exec postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"   # psql shell
docker compose down                  # stop everything (volumes preserved)
docker compose down -v               # stop and WIPE volumes (destructive)

# migrations / users (binaries ship inside the api image):
docker compose exec api sh -c '/app/goose -dir /app/migrations postgres "$DATABASE_URL" up'
docker compose exec api /app/jobs seed-owner --username <u> --password <p>
docker compose exec api /app/jobs set-password --username <u> --password <p>
```

## Services

| Service     | Port (host)        | Notes                                                                 |
| ----------- | ------------------ | --------------------------------------------------------------------- |
| postgres    | 127.0.0.1:5432     | Loopback-only. Data in named volume `rp_pgdata`.                       |
| api         | — (internal)       | Built from `../backend`. Attachments in named volume `rp_attachments`. |
| web         | — (internal)       | Built from `../frontend`; nginx serving the SPA build.                 |
| caddy       | 80                 | Single entrypoint: `/api/*` + `/healthz` → api, everything else → web. |
| cloudflared | — (outbound only)  | Public HTTPS via Cloudflare Tunnel; needs `TUNNEL_TOKEN` in `.env`.    |

Nightly ops (recurring expenses, session cleanup, `backup.sh`) run from host cron — see [DEPLOY.md](DEPLOY.md) §3.8.
