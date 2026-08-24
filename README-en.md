# MGKCT Schedule Bot

[Русский](README.md) | **English**

A bot for viewing MGKCT (Minsk State College of Digital Technologies) timetables, including first/second shift split.

Microservice architecture: Scraper ↔ gRPC/NATS ↔ Tg-bot.

## Found a bug, optimization, or have an idea?

Open an **issue**.

---

## 🚀 Deploy

1. **Install [Docker](https://www.docker.com/)**
2. **Clone the repository:**

   ```bash
   git clone github.com/arseniizyk/mgkct-schedule-bot
   ```
3. **Get the API token from [BotFather](https://t.me/botfather)**
4. **Create `.env` based on `.env.example`:**

   ```bash
   cp configs/.env.example configs/.env
   ```

   and set a real `TELEGRAM_TOKEN` in it.
5. **Run:**

   ```bash
   task run
   ```

   or directly:

   ```bash
   docker compose --env-file=./configs/.env up --build -d
   ```

### Host ports

| Service         | Port | Purpose                                        |
| --------------- | ---- | ---------------------------------------------- |
| scraper (gRPC)  | 9001 | gRPC timetable API                             |
| NATS            | 4223 | client connections (4222 inside compose)       |
| NATS monitoring | 8222 | http://localhost:8222                          |
| Adminer         | 8080 | web UI for databases                           |
| tg-bot healthz  | 8081 | only inside the compose network                |

### Environment variables

The main variables are described in `configs/.env.example`. Key points:

* `ENV` — `dev` or `prod`; any other value crashes services at startup;
* `TELEGRAM_TOKEN` — bot token from BotFather;
* scraper DB uses the `SCRAPER_DB_*` prefix (`HOST` and `NAME` are required);
* bot DB uses the `BOT_DB_*` prefix;
* `NATS_URL` defaults to `nats://nats:4222` (address inside the compose network).

### Migrations

Migrations run via [goose](https://github.com/pressly/goose): SQL files are
**embedded into the binaries** (`embed`) and applied automatically at each
service startup. No separate migration containers are needed.

To add a new migration for service `<svc>`:

1. Create a file `services/<svc>/internal/infrastructure/db/migrations/NNNNNN_name.sql`
   with the sections:

   ```sql
   -- +goose Up
   CREATE TABLE ...;

   -- +goose Down
   DROP TABLE ...;
   ```

2. Wrap plpgsql functions/procedures in
   `-- +goose StatementBegin` / `-- +goose StatementEnd`.

Applied versions are tracked in each database's `goose_db_version` table;
restarting a service never re-applies migrations.

---

## 🧩 Architecture

The project consists of two services connected via **gRPC** and **NATS**.

### 1. Scraper

A service that:

* parses timetables from the MGKCT website;
* stores data in **PostgreSQL**;
* exposes a gRPC interface:

```protobuf
rpc GetGroupSchedule(GroupScheduleRequest) returns (GroupScheduleResponse);      // Full group timetable for the latest week
rpc GetGroupScheduleByWeek(GroupScheduleRequest) returns (GroupScheduleResponse); // Full group timetable for a specific week
rpc GetAvailableWeeks(AvailableWeeksRequest) returns (AvailableWeeksResponse);   // List of available weeks
```

**Events via NATS:**

* when a timetable changes — publishes an update event;
* when a new week appears — publishes a new-week event.

### 2. Tg-bot

A service implementing the Telegram bot with the following features:

* day-by-day timetable view with ◀️ Today ▶️ navigation;
* weekly timetable view with ◀️ Current ▶️ navigation;
* second-shift group support (pair numbers follow the website data; the day header shows "2 смена" marker);
* notifications about a new week becoming available;
* notifications about timetable changes.

---

## 🧪 Tests

```bash
task test              # unit tests
task test-integration  # integration tests (requires running Docker)
```

---

## ✅ Todo

* ~~Refactoring~~ ✅
* ~~Dockerfile fixes~~ ✅
* ~~Unit tests~~ ✅
* ~~Proper CI/CD~~ ✅
* Bugfixes
* Redis caching
* Teachers' timetable support
* Telegram Mini App (React/Vue)
