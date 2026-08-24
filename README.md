# MGKCT Schedule Bot

**Русский** | [English](README-en.md)

Бот для просмотра расписаний МГКЦТ (включая разбиение на первую и вторую смены).

Микросервисная архитектура: Scraper ↔ gRPC/NATS ↔ Tg-bot.

## Нашли ошибку, оптимизацию или идею?

Создавайте **issue**.

---

## 🚀 Deploy

1. **Установить [Docker](https://www.docker.com/)**
2. **Клонировать репозиторий:**

   ```bash
   git clone github.com/arseniizyk/mgkct-schedule-bot
   ```
3. **Получить API через [BotFather](https://t.me/botfather)**
4. **Создать `.env` на основе `.env.example`:**

   ```bash
   cp configs/.env.example configs/.env
   ```

   и указать в нём реальный `TELEGRAM_TOKEN`.
5. **Запустить:**

   ```bash
   task run
   ```

   или напрямую:

   ```bash
   docker compose --env-file=./configs/.env up --build -d
   ```

### Порты на хосте

| Сервис            | Порт | Назначение                          |
| ----------------- | ---- | ----------------------------------- |
| scraper (gRPC)    | 9001 | gRPC API расписаний                 |
| NATS              | 4223 | клиентские подключения (внутри compose — 4222) |
| NATS мониторинг   | 8222 | http://localhost:8222               |
| Adminer           | 8080 | веб-интерфейс для БД                |
| tg-bot healthz    | 8081 | только внутри сети compose          |

### Переменные окружения

Основные переменные описаны в `configs/.env.example`. Ключевое:

* `ENV` — `dev` или `prod`, любое другое значение уронит сервисы на старте;
* `TELEGRAM_TOKEN` — токен бота из BotFather;
* БД скрапера — префикс `SCRAPER_DB_*` (`HOST` и `NAME` обязательны);
* БД бота — префикс `BOT_DB_*`;
* `NATS_URL` по умолчанию `nats://nats:4222` (адрес внутри compose-сети).

### Миграции

Миграции выполняются через [goose](https://github.com/pressly/goose): SQL-файлы
**встроены в бинарники** (`embed`) и применяются автоматически при старте
каждого сервиса. Отдельные контейнеры миграций не нужны.

Чтобы добавить новую миграцию для сервиса `<svc>`:

1. Создайте файл `services/<svc>/internal/infrastructure/db/migrations/NNNNNN_название.sql`
   с секциями:

   ```sql
   -- +goose Up
   CREATE TABLE ...;

   -- +goose Down
   DROP TABLE ...;
   ```

2. Для plpgsql-функций и процедур оборачивайте определение в
   `-- +goose StatementBegin` / `-- +goose StatementEnd`.

Применённые версии фиксируются в таблице `goose_db_version` каждой БД,
повторный старт сервиса повторно миграции не применяет.

---

## 🧩 Архитектура

Проект состоит из двух сервисов, связанных через **gRPC** и **NATS**.

### 1. Scraper

Сервис, который:

* парсит расписания с сайта МГКЦТ;
* сохраняет данные в **PostgreSQL**;
* предоставляет gRPC‑интерфейс:

```protobuf
rpc GetGroupSchedule(GroupScheduleRequest) returns (GroupScheduleResponse);      // Полное расписание группы на последнюю неделю
rpc GetGroupScheduleByWeek(GroupScheduleRequest) returns (GroupScheduleResponse); // Полное расписание группы на конкретную неделю
rpc GetAvailableWeeks(AvailableWeeksRequest) returns (AvailableWeeksResponse);   // Список доступных недель
```

**События через NATS:**

* если расписание обновилось — отправляет событие об изменении;
* если появилась новая неделя — отправляет событие о новой неделе.

### 2. Tg-bot

Сервис, который реализует телеграм-бота имеющего следующие функции:

* показывает расписание по дням с навигацией ◀️ Сегодня ▶️;
* показывает расписание по неделям с навигацией ◀️ Текущая ▶️;
* поддерживает группы второй смены (нумерация пар — по данным сайта, в заголовке дня отображается «2 смена»);
* отправляет уведомления о появлении новой недели;
* уведомляет об изменениях в расписании.

---

## 🧪 Тесты

```bash
task test              # unit-тесты
task test-integration  # интеграционные (нужен запущенный Docker)
```

---

## ✅ Todo

* ~~Рефакторинг~~ ✅
* ~~Фиксы докерфайлов~~ ✅
* ~~Unit тесты~~ ✅
* ~~Адекватный CI/CD~~ ✅
* Багфиксы
* Redis для кеширования
* Поддержка расписания преподавателей
* Telegram Mini App (React/Vue)
