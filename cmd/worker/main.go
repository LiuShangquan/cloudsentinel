package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloudsentinel/internal/platform/cache"
	"cloudsentinel/internal/platform/config"
	"cloudsentinel/internal/platform/database"
	"cloudsentinel/internal/platform/logger"
	platformmetrics "cloudsentinel/internal/platform/metrics"
	"cloudsentinel/internal/probe"
	"cloudsentinel/internal/workerhealth"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "error", err)
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
	defer db.Close()
	redisClient := cache.NewRedis(cfg.Redis)
	defer redisClient.Close()
	repo := probe.NewRepository(db.GORM)
	publisher := probe.NewPublisher(redisClient.Client, cfg.Probe.StreamKey)
	scheduler := probe.NewScheduler(repo, publisher, cfg.Probe.SchedulerInterval, cfg.Probe.SchedulerBatchSize, log)
	dispatcher := probe.NewRecoveryDispatcher(repo, publisher, cfg.Probe.SchedulerInterval, cfg.Probe.SchedulerBatchSize, log)
	policy := probe.NetworkPolicy{AllowPrivate: cfg.Probe.AllowPrivate, AllowLoopback: cfg.Probe.AllowLoopback}
	workerMetrics := platformmetrics.NewWorker(db.GORM)
	processor := probe.NewProcessor(repo, probe.NewHTTPProbe(policy), probe.NewTCPProbe(policy), cfg.Probe.MessageIdleTimeout, workerMetrics)
	name := consumerName()
	consumer := probe.NewConsumer(redisClient.Client, cfg.Probe.StreamKey, cfg.Probe.ConsumerGroup, name, int64(cfg.Probe.StreamReadCount), cfg.Probe.StreamBlockTimeout, cfg.Probe.MessageIdleTimeout, cfg.Probe.PendingScanPeriod, cfg.Probe.WorkerCount, cfg.Probe.QueueCapacity, processor, log)
	root, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	mux := http.NewServeMux()
	workerHealth := workerhealth.New(db, redisClient)
	mux.HandleFunc("/healthz", workerHealth.Health)
	mux.HandleFunc("/readyz", workerHealth.Ready)
	mux.Handle("/metrics", promhttp.HandlerFor(workerMetrics.Registry, promhttp.HandlerOpts{}))
	metricsServer := &http.Server{Addr: cfg.Metrics.WorkerAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("worker metrics server", "error", err)
			cancel()
		}
	}()
	go func() {
		<-root.Done()
		shutdown, stop := context.WithTimeout(context.Background(), cfg.Probe.ShutdownTimeout)
		defer stop()
		if err := metricsServer.Shutdown(shutdown); err != nil {
			log.Error("shutdown worker metrics", "error", err)
		}
	}()
	go scheduler.Run(root)
	go dispatcher.Run(root)
	log.Info("worker starting", "consumer", name, "workers", cfg.Probe.WorkerCount)
	return consumer.Run(root)
}

func consumerName() string {
	host, _ := os.Hostname()
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s-%d", host, time.Now().UnixNano())
	}
	return host + "-" + hex.EncodeToString(buffer)
}
