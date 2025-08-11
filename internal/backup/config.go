package backup

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds the application configuration.
type Config struct {
	TimeZone     *time.Location `env:"TIME_ZONE"                     envDefault:"UTC"`
	S3Region     string         `env:"S3_REGION"                     envDefault:"us-east-1"`
	S3PathPrefix string         `env:"S3_PATH_PREFIX"                envDefault:"backups"`
	DBUser       string         `env:"DB_USER,required"`
	DBPassword   string         `env:"DB_PASSWORD,required"`
	S3Endpoint   string         `env:"S3_ENDPOINT"`
	S3AccessID   string         `env:"S3_ACCESS_KEY_ID,required"`
	DBName       string         `env:"DB_NAME,required"`
	S3SecretKey  string         `env:"S3_SECRET_ACCESS_KEY,required"`
	S3Bucket     string         `env:"S3_BUCKET,required"`
	DBHost       string         `env:"DB_HOST,required"`
	LogLevel     string         `env:"LOG_LEVEL"                     envDefault:"info"`
	CronSchedule string         `env:"CRON_SCHEDULE"                 envDefault:"@daily"`
	RestoreKey   string         `env:"RESTORE_KEY"`
	DumpInfix    string         `env:"DUMP_INFIX"                    envDefault:""`
	DumpSuffix   string         `env:"DUMP_SUFFIX"                   envDefault:".sql.gz"`
	DBPort       int            `env:"DB_PORT"                       envDefault:"5432"`
	S3SSL        bool           `env:"S3_SSL"                        envDefault:"true"`
}

// LoadConfig loads the configuration from environment variables.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	return cfg, nil
}
