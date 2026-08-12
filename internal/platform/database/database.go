package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"cloudsentinel/internal/platform/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	GORM        *gorm.DB
	SQL         *sql.DB
	pingTimeout time.Duration
}

func Open(cfg config.DatabaseConfig) (*Database, error) {
	parameters := url.Values{}
	parameters.Set("charset", "utf8mb4")
	parameters.Set("parseTime", "true")
	parameters.Set("loc", "UTC")
	parameters.Set("timeout", cfg.ConnectTimeout.String())
	if cfg.TLSEnabled {
		// The built-in "true" profile enables TLS with normal certificate and
		// hostname verification. A skip-verify mode is intentionally absent.
		parameters.Set("tls", "true")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?%s",
		cfg.User,
		cfg.Password,
		net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		cfg.Name,
		parameters.Encode(),
	)

	gormDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("obtain MySQL connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	return &Database{
		GORM:        gormDB,
		SQL:         sqlDB,
		pingTimeout: cfg.PingTimeout,
	}, nil
}

func (d *Database) Name() string { return "mysql" }

func (d *Database) Check(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, d.pingTimeout)
	defer cancel()
	return d.SQL.PingContext(ctx)
}

func (d *Database) Close() error { return d.SQL.Close() }
