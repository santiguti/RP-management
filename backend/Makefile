SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c

DOTENV     := ../.env
GOOSE_DIR  := internal/db/migrations

# Run a goose subcommand against $DATABASE_URL (loaded from ../.env if not
# already exported by the caller). Uses `go tool goose` so the version is
# pinned in go.mod — no separate install step.
#
# Note: goose stores its own version table (`goose_db_version`) in the default
# schema (typically `public`). We don't override that — putting it in `rp`
# would create a bootstrap chicken-and-egg, since goose tries to write its
# state before running the first migration that creates the `rp` schema.
define goose
	@set -a; [ -f $(DOTENV) ] && . $(DOTENV); set +a; \
	go tool goose -dir $(GOOSE_DIR) postgres "$$DATABASE_URL" $(1)
endef

.PHONY: migrate-up migrate-down migrate-reset migrate-status migrate-create build test run

build:
	go build ./...

test:
	go test ./...

run:
	go run ./cmd/api

migrate-up:
	$(call goose,up)

migrate-down:
	$(call goose,down)

migrate-reset:
	$(call goose,reset)

migrate-status:
	$(call goose,status)

# usage: make migrate-create NAME=add_clients_table
migrate-create:
	@test -n "$(NAME)" || (echo "usage: make migrate-create NAME=<short_description>" && exit 1)
	@set -a; [ -f $(DOTENV) ] && . $(DOTENV); set +a; \
	go tool goose -dir $(GOOSE_DIR) -s create $(NAME) sql
