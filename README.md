# MGKCT Schedule Bot

Бот для просмотра расписаний МГКЦТ.

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

   ```env
   # scraper
   SCRAPER_URL=scraper:9001
   NATS_URL="nats://nats:4222"

   # tg-bot
   TELEGRAM_TOKEN=token

   # postgres
   POSTGRES_PASSWORD=password
   POSTGRES_USER=postgres
   POSTGRES_DB=postgres
   POSTGRES_SSL=disable
   POSTGRES_PORT=5432
   ```
5. **Запустить:**

   ```bash
   docker compose up --build -d
   ```

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

* показывает расписание по неделям;
* позволяет итерироваться по расписанию предыдущих недель;
* отправляет уведомления о появлении новой недели;
* уведомляет об изменениях в расписании.

---

## ✅ Todo

* Рефакторинг ✅
* Багфиксы
* Фиксы докерфайлов
* Unit тесты
* Redis для кеширования
* Адекватный CI/CD
* Поддержка расписания преподавателей
* Telegram Mini App (React/Vue)
