module github.com/arseniizyk/mgkct-schedule-bot/services/scraper

go 1.25.0

replace (
	github.com/arseniizyk/mgkct-schedule-bot/libs/config => ../../libs/config
	github.com/arseniizyk/mgkct-schedule-bot/libs/database => ../../libs/database
	github.com/arseniizyk/mgkct-schedule-bot/libs/proto => ../../libs/proto
)

require (
	github.com/Masterminds/squirrel v1.5.4
	github.com/PuerkitoBio/goquery v1.10.3
	github.com/arseniizyk/mgkct-schedule-bot/libs/database v0.0.0-00010101000000-000000000000
	github.com/arseniizyk/mgkct-schedule-bot/libs/proto v0.0.0-20250921184721-009ae94e57f3
	github.com/gocolly/colly v1.2.0
	github.com/nats-io/nats.go v1.46.0
	google.golang.org/grpc v1.75.1
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
	golang.org/x/sync v0.18.0 // indirect
)

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/antchfx/htmlquery v1.3.4 // indirect
	github.com/antchfx/xmlquery v1.4.4 // indirect
	github.com/antchfx/xpath v1.3.5 // indirect
	github.com/arseniizyk/mgkct-schedule-bot/libs/config v0.0.0-00010101000000-000000000000
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/jackc/pgx/v5 v5.7.6
	github.com/kennygrant/sanitize v1.2.4 // indirect
	github.com/saintfish/chardet v0.0.0-20230101081208-5e3ef4b5456d // indirect
	github.com/temoto/robotstxt v1.1.2 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250922171735-9219d122eba9 // indirect
	google.golang.org/protobuf v1.36.9
)
