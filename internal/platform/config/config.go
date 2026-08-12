package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	App          AppConfig
	Server       ServerConfig
	Log          LogConfig
	Database     DatabaseConfig
	Redis        RedisConfig
	JWT          JWTConfig
	Bootstrap    BootstrapConfig
	Probe        ProbeConfig
	Metrics      MetricsConfig
	Alertmanager AlertmanagerConfig
}

type AppConfig struct {
	Name        string
	Environment string
}

type ServerConfig struct {
	Host              string
	Port              int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type LogConfig struct {
	Level string
}

type DatabaseConfig struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
	PingTimeout     time.Duration
	TLSEnabled      bool
}

type RedisConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PingTimeout  time.Duration
	TLSEnabled   bool
}

type JWTConfig struct {
	Secret    string
	Issuer    string
	ExpiresIn time.Duration
}

type BootstrapConfig struct {
	Username string
	Password string
}

type ProbeConfig struct {
	SchedulerInterval  time.Duration
	SchedulerBatchSize int
	WorkerCount        int
	QueueCapacity      int
	StreamKey          string
	ConsumerGroup      string
	StreamReadCount    int
	StreamBlockTimeout time.Duration
	MessageIdleTimeout time.Duration
	PendingScanPeriod  time.Duration
	ShutdownTimeout    time.Duration
	AllowPrivate       bool
	AllowLoopback      bool
}

type MetricsConfig struct {
	WorkerAddress string
}

type AlertmanagerConfig struct {
	WebhookToken string
}

func Load() (Config, error) {
	var cfg Config
	var err error

	cfg.App = AppConfig{Name: value("APP_NAME", "cloudsentinel-api"), Environment: value("APP_ENV", "development")}
	cfg.Log = LogConfig{Level: value("LOG_LEVEL", "info")}

	cfg.Server.Host = value("SERVER_HOST", "0.0.0.0")
	if cfg.Server.Port, err = integer("SERVER_PORT", 8080); err != nil {
		return Config{}, err
	}
	if cfg.Server.ReadHeaderTimeout, err = duration("SERVER_READ_HEADER_TIMEOUT", "5s"); err != nil {
		return Config{}, err
	}
	if cfg.Server.ReadTimeout, err = duration("SERVER_READ_TIMEOUT", "10s"); err != nil {
		return Config{}, err
	}
	if cfg.Server.WriteTimeout, err = duration("SERVER_WRITE_TIMEOUT", "15s"); err != nil {
		return Config{}, err
	}
	if cfg.Server.IdleTimeout, err = duration("SERVER_IDLE_TIMEOUT", "60s"); err != nil {
		return Config{}, err
	}
	if cfg.Server.ShutdownTimeout, err = duration("SERVER_SHUTDOWN_TIMEOUT", "10s"); err != nil {
		return Config{}, err
	}

	cfg.Database.Host = value("MYSQL_HOST", "mysql")
	if cfg.Database.Port, err = integer("MYSQL_PORT", 3306); err != nil {
		return Config{}, err
	}
	cfg.Database.Name = value("MYSQL_DATABASE", "cloudsentinel")
	cfg.Database.User = value("MYSQL_USER", "cloudsentinel")
	cfg.Database.Password = os.Getenv("MYSQL_PASSWORD")
	if cfg.Database.MaxOpenConns, err = integer("MYSQL_MAX_OPEN_CONNS", 20); err != nil {
		return Config{}, err
	}
	if cfg.Database.MaxIdleConns, err = integer("MYSQL_MAX_IDLE_CONNS", 10); err != nil {
		return Config{}, err
	}
	if cfg.Database.ConnMaxLifetime, err = duration("MYSQL_CONN_MAX_LIFETIME", "30m"); err != nil {
		return Config{}, err
	}
	if cfg.Database.ConnMaxIdleTime, err = duration("MYSQL_CONN_MAX_IDLE_TIME", "5m"); err != nil {
		return Config{}, err
	}
	if cfg.Database.ConnectTimeout, err = duration("MYSQL_CONNECT_TIMEOUT", "5s"); err != nil {
		return Config{}, err
	}
	if cfg.Database.PingTimeout, err = duration("MYSQL_PING_TIMEOUT", "3s"); err != nil {
		return Config{}, err
	}
	if cfg.Database.TLSEnabled, err = boolean("MYSQL_TLS_ENABLED", false); err != nil {
		return Config{}, err
	}

	cfg.Redis.Host = value("REDIS_HOST", "redis")
	if cfg.Redis.Port, err = integer("REDIS_PORT", 6379); err != nil {
		return Config{}, err
	}
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
	if cfg.Redis.DB, err = integer("REDIS_DB", 0); err != nil {
		return Config{}, err
	}
	if cfg.Redis.PoolSize, err = integer("REDIS_POOL_SIZE", 20); err != nil {
		return Config{}, err
	}
	if cfg.Redis.MinIdleConns, err = integer("REDIS_MIN_IDLE_CONNS", 2); err != nil {
		return Config{}, err
	}
	if cfg.Redis.DialTimeout, err = duration("REDIS_DIAL_TIMEOUT", "5s"); err != nil {
		return Config{}, err
	}
	if cfg.Redis.ReadTimeout, err = duration("REDIS_READ_TIMEOUT", "3s"); err != nil {
		return Config{}, err
	}
	if cfg.Redis.WriteTimeout, err = duration("REDIS_WRITE_TIMEOUT", "3s"); err != nil {
		return Config{}, err
	}
	if cfg.Redis.PingTimeout, err = duration("REDIS_PING_TIMEOUT", "3s"); err != nil {
		return Config{}, err
	}
	if cfg.Redis.TLSEnabled, err = boolean("REDIS_TLS_ENABLED", false); err != nil {
		return Config{}, err
	}

	cfg.JWT.Secret = os.Getenv("JWT_SECRET")
	cfg.JWT.Issuer = value("JWT_ISSUER", "cloudsentinel-api")
	if cfg.JWT.ExpiresIn, err = duration("JWT_EXPIRES_IN", "2h"); err != nil {
		return Config{}, err
	}
	cfg.Bootstrap.Username = os.Getenv("BOOTSTRAP_USER_USERNAME")
	cfg.Bootstrap.Password = os.Getenv("BOOTSTRAP_USER_PASSWORD")

	if cfg.Probe.SchedulerInterval, err = duration("PROBE_SCHEDULER_INTERVAL", "5s"); err != nil {
		return Config{}, err
	}
	if cfg.Probe.SchedulerBatchSize, err = integer("PROBE_SCHEDULER_BATCH_SIZE", 100); err != nil {
		return Config{}, err
	}
	if cfg.Probe.WorkerCount, err = integer("PROBE_WORKER_COUNT", 4); err != nil {
		return Config{}, err
	}
	if cfg.Probe.QueueCapacity, err = integer("PROBE_QUEUE_CAPACITY", 100); err != nil {
		return Config{}, err
	}
	cfg.Probe.StreamKey = value("PROBE_STREAM_KEY", "cloudsentinel:probe:executions")
	cfg.Probe.ConsumerGroup = value("PROBE_CONSUMER_GROUP", "cloudsentinel-probe-workers")
	if cfg.Probe.StreamReadCount, err = integer("PROBE_STREAM_READ_COUNT", 10); err != nil {
		return Config{}, err
	}
	if cfg.Probe.StreamBlockTimeout, err = duration("PROBE_STREAM_BLOCK_TIMEOUT", "5s"); err != nil {
		return Config{}, err
	}
	if cfg.Probe.MessageIdleTimeout, err = duration("PROBE_MESSAGE_IDLE_TIMEOUT", "30m"); err != nil {
		return Config{}, err
	}
	if cfg.Probe.PendingScanPeriod, err = duration("PROBE_PENDING_SCAN_INTERVAL", "30s"); err != nil {
		return Config{}, err
	}
	if cfg.Probe.ShutdownTimeout, err = duration("PROBE_SHUTDOWN_TIMEOUT", "30s"); err != nil {
		return Config{}, err
	}
	if cfg.Probe.AllowPrivate, err = boolean("PROBE_ALLOW_PRIVATE_NETWORKS", true); err != nil {
		return Config{}, err
	}
	if cfg.Probe.AllowLoopback, err = boolean("PROBE_ALLOW_LOOPBACK", false); err != nil {
		return Config{}, err
	}

	cfg.Metrics.WorkerAddress = value("WORKER_METRICS_ADDRESS", ":9091")
	cfg.Alertmanager.WebhookToken = os.Getenv("ALERTMANAGER_WEBHOOK_TOKEN")
	return cfg, nil
}

func value(key, fallback string) string {
	if raw, ok := os.LookupEnv(key); ok {
		return raw
	}
	return fallback
}

func integer(key string, fallback int) (int, error) {
	raw := value(key, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("configuration %s: invalid integer", key)
	}
	return parsed, nil
}

func duration(key, fallback string) (time.Duration, error) {
	raw := value(key, fallback)
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("configuration %s: invalid duration", key)
	}
	return parsed, nil
}

func boolean(key string, fallback bool) (bool, error) {
	raw := value(key, strconv.FormatBool(fallback))
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("configuration %s: invalid boolean", key)
	}
	return parsed, nil
}
