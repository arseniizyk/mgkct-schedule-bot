# AGENTS.md

## Repository Architecture

- Single Go module (`github.com/arseniizyk/mgkct-schedule-bot`, Go 1.27), root `go.mod`.
- Monolithic application combining schedule scraping (Scraper) and Telegram bot functionality:
  - Entry point: `cmd/bot/main.go`.
  - Initialization and graceful shutdown layer: `internal/app`.
  - Configuration: `internal/config` (cleanenv + godotenv).
  - Domain entities: `internal/domain/entities` (pure Go structures without protobuf).
  - Data storage: `internal/repository` (SQLite via `modernc.org/sqlite` in WAL mode).
  - Database and migrations: `internal/infrastructure/db` (embedded goose migrations in `internal/infrastructure/db/migrations/000001_init.sql`).
  - Parsing: `internal/infrastructure/parser` (Colly + goquery, independent parsing across multiple weeks).
  - Business logic: `internal/service` (`ScheduleService`, `TeacherService`, `ScraperService` with Smart Polling).
  - Telegram: `internal/infrastructure/telegram` (telebot handlers, custom keyboards, formatting, throttled broadcasts).

## Development Commands

- Build binary: `task build` (`go build -o bin/bot ./cmd/bot`).
- Run locally: `task run` (`go run ./cmd/bot`).
- Run tests: `task test` (`go test -v ./internal/...`).
- Linter and formatting: `task lint`, `task lint:fix`, `task format`.
- Clean dependencies: `task tidy` (`go mod tidy`).
- Docker containers: `task up` (`docker compose --env-file=./configs/.env up --build -d`), `task down`.

## Configuration and Environment Variables

- Configuration is automatically loaded from `configs/.env` or process environment variables.
- `ENV`: `dev` or `prod`.
- `BOT_TOKEN` (or legacy `TELEGRAM_TOKEN`): Telegram bot token.
- `BOT_HEALTH_PORT`: HTTP health check port (default: 8081).
- `BOT_WEB_APP_URL`: Public HTTPS URL for Telegram Mini App.
- `BOT_API_URL`: Custom Telegram Bot API URL (e.g. Cloudflare Worker reverse proxy).
- `DB_PATH`: Path to SQLite database file (default: `bot.db`).
- `SCRAPER_INTERVAL`: Default daytime polling interval (default: `1m`).
- `SCRAPER_PEAK_INTERVAL`: Peak turbo interval (default: `25s`, active Fri 12:00 — Sat morning until a new week schedule is published).
- `SCRAPER_NIGHT_INTERVAL`: Night interval (default: `20m`, 22:00 to 08:00 and on Sundays).

## Migrations

- Migrations are embedded in the binary via `embed` and applied automatically on application startup using `goose.UpContext`.
- Tables: `schedules`, `teacher_schedules`, `users`, `goose_db_version`.
- All SQLite SQL triggers must be wrapped in `-- +goose StatementBegin` / `-- +goose StatementEnd`.

## Health Check and Graceful Shutdown

- HTTP GET `/healthz` (port 8081) verifies SQLite database connectivity with ping.
- On SIGINT/SIGTERM: the bot stops long polling (`bot.Stop()`), cancels scraper context, shuts down HTTP health server, and closes SQLite database connection.

## Branching Model and Git Workflow

- **`main` (Production):**
  - Protected by Branch Protection (direct push and force push are prohibited).
  - Merging is permitted **strictly from the `dev` branch** via Pull Request.
  - Requires at least 1 approving review and passing all status checks: `Build & Test`, `golangci-lint`, `govulncheck`, `PR into main only from dev`.
  - Rules apply to all users, including repository administrators.
- **`dev` (Staging / Main Development Branch):**
  - Protected by Branch Protection (direct push and force push are prohibited).
  - Features and bugfixes are merged via Pull Request from working branches (`feat/*`, `fix/*`, `perf/*`).
  - Requires at least 1 approving review and passing all status checks: `Build & Test`, `golangci-lint`, `govulncheck`.
- **Pull Request Rules:**
  - **Reviewers:** Every PR requests reviews from `@abesanets` and `@arseniizyk`. An approving review from either maintainer is required to merge.
  - **Assignee:** The PR author **must always** be assigned to the PR.
- **Contributor Guidelines:**
  - New branches must be created from `dev`.
  - Local development and debugging must use a personal test bot (created via `@BotFather` with token placed in local `configs/.env`). Repository secrets are never shared with contributors.
  - Completed changes must be submitted via Pull Request targeting the `dev` branch only.

## Conventions

- Commit messages must follow Conventional Commits in English (strict, concise, no verbose descriptions).
- Pull Request descriptions must be in English: strictly to the point (what, why, how) with minimal emoji usage.
- Structured logging using `slog` JSON format. Errors include the operation prefix (`const op = "pkg.Func"`).
- Documentation and code comments can be in Russian or English.
