# AGENTS.md

## Repo layout

- Multi-module Go monorepo (Go 1.27), **no root go.mod**. Each module is separate:
  - `services/scraper`, `services/tg-bot` — the two apps
  - `libs/config` — shared config loader (cleanenv)
  - `proto` — directory, but its module path is `github.com/arseniizyk/mgkct-schedule-bot/libs/proto` (path ≠ module name)
- Services reference `libs/config` and `proto` via `replace` directives pointing at `../../libs/config` and `../../proto`. Run all go commands from inside the module dir (`services/scraper`, `services/tg-bot`).
- Entrypoints: `services/<svc>/cmd/<svc>/main.go` (no more `cmd/migrate`).
- Service internals follow the same layering in both: `internal/{app,domain,infrastructure,repository,service,transport}` (tg-bot also has `usecases`).

## Commands

- Full stack: `task run` (= `docker compose --env-file=./configs/.env up --build -d`), stop+purge: `task down`. Requires `configs/.env` copied from `configs/.env.example` with a real `TELEGRAM_TOKEN`.
- Root orchestration over all modules (`scraper`, `tg-bot`, `config`, `proto` includes): `task test` (unit), `task test-integration` (needs Docker; build tag `integration`), `task lint` / `task lint:fix` / `task format`, `task deps:tidy`.
- Per-module tasks live in each module's own Taskfile (`services/<svc>/Taskfile.yml`, `libs/config/Taskfile.yml`, `proto/Taskfile.yml`): `task <mod>:build|vet|test|test:integration|format|lint|lint:fix|tidy`.
- Lint/format use a pinned golangci-lint auto-installed into `./bin` on first use (root task `install-golangci-lint`; config at repo root `.golangci.yml`). `bin/` is gitignored. Integration-tagged code needs `-tags=integration`.

## Config & environment

- Config is env-var based (cleanenv), loaded either from process env or a file passed via `-config_path` flag (Docker uses `env_file: ./configs/.env`). There is no default config file locally — set vars or pass a path when running outside Docker.
- DB env vars use prefixes: scraper DB = `SCRAPER_DB_*`, bot DB = `BOT_DB_*` (see `libs/config/config.go` `env-prefix` tags). `HOST` and `NAME` are required.
- `ENV` must be `dev` or `prod`; anything else panics at startup. `NATS_URL` defaults to `nats://nats:4222` (container hostname — won't work outside compose). Bot health HTTP port = `BOT_HEALTH_PORT` (default 8081).
- Host-side NATS port is **4223** (`4223:4222` mapping); scraper gRPC is on 9001, Adminer on 8080.

## Migrations & codegen

- Migrations use **embedded goose**: SQL files live in `services/<svc>/internal/infrastructure/db/migrations/*.sql` (`embed` cannot reach parent dirs) and are applied automatically at service startup via `goose.UpContext` in each service's `internal/infrastructure/db` package. Version state sits in the `goose_db_version` table; restarts are idempotent.
- To add a migration for `<svc>`: create `NNNNNN_name.sql` with `-- +goose Up` / `-- +goose Down` sections. Wrap plpgsql function bodies in `-- +goose StatementBegin` / `-- +goose StatementEnd`, otherwise goose splits them at inner semicolons.
- `proto/*.pb.go` are generated from `proto/scraper.proto` by `task proto:gen` (pinned buf + protoc plugins auto-installed into `./bin`). Regenerate rather than hand-editing.

## Healthchecks & lifecycle

- compose healthchecks: scraper — gRPC health protocol (`grpc_health_probe` baked into the image); tg-bot — HTTP GET `/healthz` (pings the DB pool, defined in `services/tg-bot/internal/app/health.go`).
- Graceful shutdown: both apps stop on SIGINT/SIGTERM; tg-bot calls `bot.Stop()` on first signal (telebot does not trap signals itself), then closes gRPC/NATS/pool via defers. Scraper marks gRPC NOT_SERVING, drains NATS last.

## Conventions

- Docs, comments, and commit-facing text are in Russian; keep new user-facing strings and docs consistent.
- Logging is `slog` JSON (debug level in dev, info in prod); errors use the `op` prefix pattern (`const op = "pkg.Func"` + `fmt.Errorf("%s: ...")`) — see `libs/config/config.go`.
- Tests are table-driven with testify; integration tests use testcontainers (`-tags=integration`) and reuse the embedded goose migrations through `db.Connect`.
