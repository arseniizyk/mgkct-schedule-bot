module github.com/arseniizyk/mgkct-schedule-bot/libs/database

go 1.25.0

replace github.com/arseniizyk/mgkct-schedule-bot/libs/config => ../../libs/config

require github.com/jackc/pgx/v5 v5.7.6

require (
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
)

require (
	github.com/arseniizyk/mgkct-schedule-bot/libs/config v0.0.0-00010101000000-000000000000
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/text v0.31.0 // indirect
)
