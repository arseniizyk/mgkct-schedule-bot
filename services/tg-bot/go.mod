module github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot

go 1.25.0

replace (
	github.com/arseniizyk/mgkct-schedule-bot/libs/config => ../../libs/config
	github.com/arseniizyk/mgkct-schedule-bot/libs/database => ../../libs/database
	github.com/arseniizyk/mgkct-schedule-bot/libs/proto => ../../libs/proto
)

require (
	github.com/arseniizyk/mgkct-schedule-bot/libs/config v0.0.0-00010101000000-000000000000
	github.com/arseniizyk/mgkct-schedule-bot/libs/database v0.0.0-00010101000000-000000000000
	github.com/arseniizyk/mgkct-schedule-bot/libs/proto v0.0.0-20250903204728-9e1c94e1aa27
	github.com/nats-io/nats.go v1.45.0
	google.golang.org/grpc v1.75.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/nats-io/nkeys v0.4.11 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/sync v0.16.0 // indirect
)

require (
	github.com/Masterminds/squirrel v1.5.4
	github.com/jackc/pgx/v5 v5.7.6
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250826171959-ef028d996bc1 // indirect
	google.golang.org/protobuf v1.36.8
	gopkg.in/telebot.v4 v4.0.0-beta.5
)
