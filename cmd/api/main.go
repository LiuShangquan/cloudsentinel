package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"cloudsentinel/internal/asset"
	"cloudsentinel/internal/auth"
	"cloudsentinel/internal/health"
	"cloudsentinel/internal/httpserver"
	"cloudsentinel/internal/incident"
	"cloudsentinel/internal/platform/cache"
	"cloudsentinel/internal/platform/config"
	"cloudsentinel/internal/platform/database"
	"cloudsentinel/internal/platform/logger"
	platformmetrics "cloudsentinel/internal/platform/metrics"
	"cloudsentinel/internal/probe"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log, err := logger.New(cfg.App.Environment, cfg.Log.Level, os.Stdout)
	if err != nil {
		return err
	}
	slog.SetDefault(log)

	db, err := database.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("close MySQL", "error", err)
		}
	}()
	redisClient := cache.NewRedis(cfg.Redis)
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Error("close Redis", "error", err)
		}
	}()

	authRepository := auth.NewRepository(db.GORM)
	tokenManager, err := auth.NewTokenManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.ExpiresIn)
	if err != nil {
		return err
	}
	if err := auth.Bootstrap(context.Background(), authRepository, cfg.Bootstrap); err != nil {
		return err
	}
	if cfg.Alertmanager.WebhookToken == "" {
		return errors.New("configuration ALERTMANAGER_WEBHOOK_TOKEN: required")
	}
	if cfg.Alertmanager.WebhookToken == cfg.JWT.Secret {
		return errors.New("configuration ALERTMANAGER_WEBHOOK_TOKEN: must differ from JWT_SECRET")
	}
	authService := auth.NewService(authRepository, tokenManager)
	authHandler := auth.NewHandler(authService)
	assetRepository := asset.NewRepository(db.GORM)
	assetHandler := asset.NewHandler(asset.NewService(assetRepository))
	probeRepository := probe.NewRepository(db.GORM)
	probeHandler := probe.NewHandler(probe.NewTaskService(probeRepository))
	apiMetrics := platformmetrics.NewAPI()
	apiMetrics.Registry.MustRegister(platformmetrics.NewActiveIncidentCollector(db.GORM))
	incidentHandler := incident.NewHandler(incident.NewService(incident.NewRepository(db.GORM)))
	router := httpserver.NewRouter(log, health.NewHandler(db, redisClient), httpserver.Modules{AuthHandler: authHandler, AuthMiddleware: tokenManager.Middleware(), AssetHandler: assetHandler, ProbeHandler: probeHandler, MetricsMiddleware: apiMetrics.Middleware(), MetricsHandler: apiMetrics.Handler(), IncidentHandler: incidentHandler, MachineAuth: incident.MachineAuth(cfg.Alertmanager.WebhookToken)})
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	serverError := make(chan error, 1)
	go func() {
		log.Info("api starting", "address", server.Addr, "environment", cfg.App.Environment)
		serverError <- server.ListenAndServe()
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalContext.Done():
		log.Info("api shutdown requested")
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
