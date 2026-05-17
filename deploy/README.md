# Deploy

Docker Compose definitions for shop-PC production deployment.

## When to use this

| Environment | Postgres source | Use this compose? |
| --- | --- | --- |
| **Dev** (this PC, Postgres already on host)     | Host Postgres at `localhost:5432` | No — point `DATABASE_URL` at the host PG in `.env` and run the Go API natively (`go run ./cmd/api`). |
| **Prod** (shop PC, clean install)               | Dockerised Postgres in this stack | Yes — `docker compose up -d` brings up everything. |

The compose file is committed so the prod deploy is reproducible. It's safe to *not* use it locally as long as `DATABASE_URL` in `.env` points at whatever Postgres you do use.

## Prerequisites

- Docker Engine + Compose v2
- A populated `.env` at the **repo root** (see `../.env.example`).

## Usage (prod or any dev that wants a containerised PG)

All commands assume CWD = `deploy/`. The compose file references `../.env`.

```bash
# Bring up Postgres (currently the only service)
docker compose up -d

# Status / health
docker compose ps

# Tail logs
docker compose logs -f postgres

# Open a psql shell
docker compose exec postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"

# Stop everything (volumes preserved)
docker compose down

# Stop and WIPE the database volume (destructive)
docker compose down -v
```

## Services (current)

| Service  | Port      | Notes                                                                                            |
| -------- | --------- | ------------------------------------------------------------------------------------------------ |
| postgres | 5432:5432 | Data persisted in named volume `rp_pgdata`.                                                      |
| api      | 8080:8080 | Built from `../backend`. **Prod-only by default.** In dev, run `go run ./cmd/api` natively — the dev `.env` `DATABASE_URL` points at the host PG and won't resolve from inside a container. |

More services (`web`, `caddy`, `cloudflared`) are added in later milestone-1 steps.
