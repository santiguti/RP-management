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

## License

TBD.
