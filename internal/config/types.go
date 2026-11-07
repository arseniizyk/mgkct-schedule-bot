package config

import "time"

type Env string

const (
	EnvDev  Env = "dev"
	EnvProd Env = "prod"
)

type Config struct {
	Env Env `env:"ENV" env-default:"prod"`

	Bot     BotConfig     `env-prefix:"BOT_"`
	DB      DBConfig      `env-prefix:"DB_"`
	Scraper ScraperConfig `env-prefix:"SCRAPER_"`
	MTProto MTProtoConfig `env-prefix:"MTPROTO_"`
}

type MTProtoConfig struct {
	Enabled       bool   `env:"ENABLED" env-default:"true"`
	AppID         int    `env:"APP_ID" env-default:"2040"`
	AppHash       string `env:"APP_HASH" env-default:"b18441a1ff607e10a989891a5462e627"`
	SessionPath   string `env:"SESSION_PATH" env-default:"data/mtproto.session"`
	ProxyURL      string `env:"PROXY_URL" env-default:""`
	MTProxyAddr   string `env:"MTPROXY_ADDR" env-default:""`
	MTProxySecret string `env:"MTPROXY_SECRET" env-default:""`
}

type BotConfig struct {
	Token         string  `env:"TOKEN"`
	HealthPort    string  `env:"HEALTH_PORT" env-default:"8081"`
	WebAppURL     string  `env:"WEB_APP_URL" env-default:""`
	APIURL        string  `env:"API_URL" env-default:""`
	AdminIDs      []int64 `env:"ADMIN_IDS" env-separator:","`
	WebhookURL    string  `env:"WEBHOOK_URL" env-default:""`
	WebhookSecret string  `env:"WEBHOOK_SECRET" env-default:""`
}

type DBConfig struct {
	Path string `env:"PATH" env-default:"bot.db"`
}

type ScraperConfig struct {
	Interval      time.Duration `env:"INTERVAL" env-default:"1m"`
	PeakInterval  time.Duration `env:"PEAK_INTERVAL" env-default:"25s"`
	NightInterval time.Duration `env:"NIGHT_INTERVAL" env-default:"20m"`
}
