-- +goose Up
-- Таблица для персистентного хранения дат отправки вечернего дайджеста.
-- Предотвращает повторную рассылку при перезапуске бота в течение одних суток.
CREATE TABLE IF NOT EXISTS digest_sent_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    kind        TEXT    NOT NULL,          -- 'group' или 'teacher'
    target_key  TEXT    NOT NULL,          -- "<group_id>:<subgroup>" или "<teacher_name>"
    sent_date   TEXT    NOT NULL,          -- дата отправки в формате YYYY-MM-DD
    UNIQUE(kind, target_key, sent_date)
);

CREATE INDEX IF NOT EXISTS idx_digest_sent_log_date ON digest_sent_log(sent_date);

-- +goose Down
DROP INDEX IF EXISTS idx_digest_sent_log_date;
DROP TABLE IF EXISTS digest_sent_log;
