# MGKCT Schedule Bot

[Русский](README.md) | **English**

A Telegram bot for easy viewing of Minsk State College of Digital Technologies (MGKCT) timetables for students and teachers.

Lightweight Go monolith (Single Binary) with embedded **SQLite (WAL)**, smart adaptive scraping, and instant change notifications.

---

## ✨ Features

- 📅 **Timetables for groups and teachers:**
  - Daily schedule with fast navigation (◀️ / ▶️);
  - Weekly schedule with week switching (◀️ Prev Week / Next Week ▶️);
  - Second-shift support with automatic pair index offsets;
  - Interactive teacher surname search.
- ⏰ **Bell schedule:** updated unified pair timing for all school days.
- ⚡ **Smart Dynamic Polling:**
  - **Turbo mode (Fri 12:00 — Sat morning):** frequent scraping (every 25s) until a new week is published;
  - **Regular mode:** scrapes once a minute during the day;
  - **Night / Sunday mode:** scrapes every 20 minutes to save server resources.
- 🔔 **Clean notifications (no spam):**
  - Notification when a new week's timetable is released (strictly once);
  - Personal notifications on changes only when subjects/classrooms actually change;
  - Rate Limiter (25 msg/s) preventing Telegram API flood bans.
- 💾 **Embedded Database:** Pure Go SQLite (`modernc.org/sqlite`) in WAL mode, requiring no external DBMS, CGO, or Docker.

---

## 🚀 Quick Start

### Option 1: Local Run (No Docker needed)

1. **Clone the repository:**
   ```bash
   git clone https://github.com/arseniizyk/mgkct-schedule-bot.git
   cd mgkct-schedule-bot
   ```

2. **Create configuration file:**
   ```bash
   cp configs/.env.example configs/.env
   ```
   Set your bot token in `configs/.env`:
   ```env
   BOT_TOKEN=123456789:ABCdefGHIjklMNOpqrsTUVwxyz
   ```

3. **Run:**
   ```bash
   task run
   # or directly:
   go run ./cmd/bot
   ```

---

### Option 2: Run via Docker Compose

```bash
task up
# or:
docker compose --env-file=./configs/.env up --build -d
```

Stop:
```bash
task down
```

---

### Option 3: Free Cloud Hosting (Koyeb / Render / Fly.io / VPS)

Thanks to the single-binary Go + SQLite architecture, the project builds into a tiny Docker image (<30 MB):
- Healthcheck port: `8081` (`GET /healthz`);
- Persistent storage volume: `/app/data` (file `/app/data/bot.db`).

---

## ⚙️ Environment Variables

Main settings configured in `configs/.env` or system environment:

| Variable | Default | Description |
|---|---|---|
| `ENV` | `dev` | Application mode (`dev` or `prod`) |
| `BOT_TOKEN` | — | Telegram Bot token from [@BotFather](https://t.me/BotFather) (legacy `TELEGRAM_TOKEN` also supported) |
| `BOT_HEALTH_PORT` | `8081` | HTTP healthcheck server port (`/healthz`) |
| `DB_PATH` | `bot.db` | Path to SQLite database file |
| `SCRAPER_INTERVAL` | `1m` | Standard daytime scraper interval |
| `SCRAPER_PEAK_INTERVAL` | `25s` | Turbo mode interval on Fri-Sat until new week is found |
| `SCRAPER_NIGHT_INTERVAL` | `20m` | Night / Sunday interval (22:00 - 08:00 and Sundays) |

---

## 🛠 Development Commands (Taskfile)

- `task build` — builds binary to `bin/bot`;
- `task run` — runs application locally;
- `task test` — executes unit tests;
- `task lint` / `task lint:fix` — runs `golangci-lint` checks and auto-fixes;
- `task format` — formats source code (`go fmt`);
- `task tidy` — tidies Go module dependencies.

---

## 🏛 Architecture

```
┌────────────────────────────────────────────────────────┐
│               mgkct-bot (Single Binary)                │
│                                                        │
│  ┌──────────────────┐           ┌───────────────────┐  │
│  │  Scraper Engine  │ ─(Chan)─► │  Telegram Engine  │  │
│  │ (Smart Polling)  │           │  (Telebot + Rate) │  │
│  └────────┬─────────┘           └─────────┬─────────┘  │
│           │                               │            │
│           └───────────────┬───────────────┘            │
│                           ▼                            │
│                 Embedded SQLite (WAL)                  │
└────────────────────────────────────────────────────────┘
```
