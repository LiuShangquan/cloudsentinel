package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"cloudsentinel/internal/platform/config"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	Client      *redis.Client
	pingTimeout time.Duration
}

func NewRedis(cfg config.RedisConfig) *Redis {
	var tlsConfig *tls.Config
	if cfg.TLSEnabled {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.Host}
	}
	client := redis.NewClient(&redis.Options{
		Addr:         net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		TLSConfig:    tlsConfig,
	})
	return &Redis{Client: client, pingTimeout: cfg.PingTimeout}
}

func (r *Redis) Name() string { return "redis" }

func (r *Redis) Check(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, r.pingTimeout)
	defer cancel()
	if err := r.Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping Redis: %w", err)
	}
	return nil
}

func (r *Redis) Close() error { return r.Client.Close() }
