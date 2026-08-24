module github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot

go 1.25.0

replace (
	github.com/arseniizyk/mgkct-schedule-bot/libs/config => ../../libs/config
	github.com/arseniizyk/mgkct-schedule-bot/libs/proto => ../../proto
)

require (
	github.com/arseniizyk/mgkct-schedule-bot/libs/config v0.0.0-20260126102318-d1ad25a18528
	github.com/arseniizyk/mgkct-schedule-bot/libs/proto v0.0.0-20250925210302-2191841f424e
	github.com/golang-migrate/migrate/v4 v4.19.1
	github.com/nats-io/nats.go v1.48.0
	google.golang.org/grpc v1.82.1
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/ilyakaznacheev/cleanenv v1.5.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/klauspost/compress v1.18.3 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/nats-io/nkeys v0.4.14 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/sync v0.20.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
)

require (
	github.com/Masterminds/squirrel v1.5.4
	github.com/jackc/pgx/v5 v5.8.0
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11
	gopkg.in/telebot.v4 v4.0.0-beta.7
)
