# AGENTS.md

## Архитектура репозитория

- Единый Go-модуль (`github.com/arseniizyk/mgkct-schedule-bot`, Go 1.27), корневой `go.mod`.
- Монолитное приложение, объединяющее функции парсера расписания (Scraper) и Telegram-бота:
  - Точка входа: `cmd/bot/main.go`.
  - Слой инициализации и graceful shutdown: `internal/app`.
  - Конфигурация: `internal/config` (cleanenv + godotenv).
  - Доменные сущности: `internal/domain/entities` (чистые структуры Go без protobuf).
  - Хранилище данных: `internal/repository` (SQLite через `modernc.org/sqlite` в WAL-режиме).
  - База данных и миграции: `internal/infrastructure/db` (embedded goose миграции в `internal/infrastructure/db/migrations/000001_init.sql`).
  - Парсинг: `internal/infrastructure/parser` (Colly + goquery, раздельный парсинг нескольких недель).
  - Бизнес-логика: `internal/service` (`ScheduleService`, `TeacherService`, `ScraperService` со Smart Polling).
  - Telegram: `internal/infrastructure/telegram` (обработчики telebot, клавиатуры, форматирование, рассылка с троттлингом).

## Команды

- Сборка бинарника: `task build` (`go build -o bin/bot ./cmd/bot`).
- Локальный запуск: `task run` (`go run ./cmd/bot`).
- Запуск тестов: `task test` (`go test -v ./internal/...`).
- Линтер и форматирование: `task lint`, `task lint:fix`, `task format`.
- Очистка модулей: `task tidy` (`go mod tidy`).
- Контейнеризация: `task up` (`docker compose --env-file=./configs/.env up --build -d`), `task down`.

## Конфигурация и переменные окружения

- Конфигурация загружается автоматически из `configs/.env` или переменных окружения процесса.
- `ENV`: `dev` или `prod`.
- `BOT_TOKEN` (или legacy `TELEGRAM_TOKEN`): токен Telegram-бота.
- `BOT_HEALTH_PORT`: порт HTTP healthcheck-сервера (по умолчанию 8081).
- `DB_PATH`: путь к файлу SQLite (по умолчанию `bot.db`).
- `SCRAPER_INTERVAL`: стандартный дневной интервал опроса (по умолчанию `1m`).
- `SCRAPER_PEAK_INTERVAL`: пиковый турбо-интервал (по умолчанию `25s`, активен в Пт 12:00 — Сб утро до нахождения новой недели).
- `SCRAPER_NIGHT_INTERVAL`: ночной интервал (по умолчанию `20m`, с 22:00 до 08:00 и по воскресеньям).

## Миграции

- Миграции встроены в бинарник через `embed` и применяются автоматически при старте приложения через `goose.UpContext`.
- Таблицы: `schedules`, `teacher_schedules`, `users`, `goose_db_version`.
- Все SQL-триггеры для SQLite должны оборачиваться в `-- +goose StatementBegin` / `-- +goose StatementEnd`.

## Healthcheck и Graceful Shutdown

- HTTP GET `/healthz` (порт 8081) проверяет пинг базы данных SQLite.
- При получении SIGINT/SIGTERM: бот останавливает LongPoller (`bot.Stop()`), отменяет контекст скрапера, закрывает HTTP health-сервер и закрывает подключение к SQLite.

## Конвенции

- Документация, комментарии и сообщения коммитов — на русском языке.
- Логирование — `slog` JSON. Ошибки содержат префикс операции (`const op = "pkg.Func"`).
