# RP-Management

Management application for an electronic repair shop in Argentina. Replaces a single brittle Excel with a proper API + Postgres backend and a React dashboard.

## Roadmap

- **v1** *(in progress)* — full Excel parity: clients, work orders (with quote/approval flow), devices, money in/out, fixed expenses, parts inventory, reports, multi-user auth, Excel import. Self-hosted on a shop PC.
- **v2** — WhatsApp Business API integration: parse inbound messages; auto-reply when the answer is in the DB; surface unanswered messages in the dashboard.
- **v3** — Gemini-powered natural-language command bar: type prose, system records the right transaction / inventory update / WO change.

## Stack

| Layer | Choice |
|---|---|
| Backend | Go 1.23 + chi + sqlc + pgx + goose |
| Database | PostgreSQL 16 |
| Frontend | React 19 + TypeScript + Vite + Tailwind + shadcn/ui + TanStack Query |
| Auth | Sessions (HttpOnly cookies) + argon2id |
| Deploy | Docker Compose + Caddy + Cloudflare Tunnel |

## Quick start (dev)

Prerequisites: Go 1.25+, Node 20+, host PostgreSQL 16 reachable at `localhost:5432`, `make`, `psql`.

```bash
git clone <repo-url> rp-management
cd rp-management

# 1. Configure
cp .env.example .env
# Edit .env: set DATABASE_URL to your local PG (e.g. postgres://USER:PASS@localhost:5432/DBNAME?sslmode=disable)
# and fill COOKIE_SECRET with `head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'`

# 2. Apply migrations
cd backend
make migrate-up

# 3. Seed the first owner user
go run ./cmd/jobs seed-owner --username santi --password dev123 --full-name "Santi"

# 4. Run the API (terminal 1)
go run ./cmd/api
# → http://localhost:8080/healthz returns {"status":"ok"}

# 5. Run the web app (terminal 2)
cd ../frontend
npm install
npm run dev
# → open http://localhost:5173, log in with santi / dev123
```

## Quick start (prod, on the shop PC)

See [`deploy/README.md`](deploy/README.md). Brings up Postgres + API + Web + Caddy + Cloudflare Tunnel via `docker compose up -d`.

## License

TBD.
