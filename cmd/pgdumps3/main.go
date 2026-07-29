package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pocketbase/pocketbase/tools/cron"

	"github.com/advisoreeu/pgdumps3/internal/backup"
	"github.com/advisoreeu/pgdumps3/internal/metrics"
)

const metricsShutdownTimeout = 5 * time.Second

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	config, err := backup.LoadConfig()
	if err != nil {
		return err
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: getLogLevel(config.LogLevel),
	})
	logger := slog.New(jsonHandler)
	slog.SetDefault(logger)

	slog.Info("starting pgdumps3", "version", version)

	pg := backup.NewPostgres(config)

	err = pg.SetVersion()
	if err != nil {
		return err
	}

	ctx := context.Background()

	s3, err := backup.NewS3(ctx, config)
	if err != nil {
		return err
	}

	if config.RestoreKey != "" {
		return backup.Restore(ctx, s3, pg, config, config.RestoreKey)
	}

	var m *metrics.Metrics

	if config.MetricsEnabled {
		m = metrics.New(config.DBName)
		m.SetBuildInfo(version, pg.MajorVersion)

		metricsServer := m.NewServer(config.MetricsAddr)

		go func() {
			slog.Info("metrics server listening", "addr", config.MetricsAddr)

			if serveErr := metricsServer.ListenAndServe(); serveErr != nil &&
				!errors.Is(serveErr, http.ErrServerClosed) {
				slog.Error("metrics server failed", "error", serveErr)
			}
		}()

		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), metricsShutdownTimeout)
			defer cancel()

			if shutdownErr := metricsServer.Shutdown(shutdownCtx); shutdownErr != nil {
				slog.Error("failed to shut down metrics server", "error", shutdownErr)
			}
		}()
	}

	wg := sync.WaitGroup{}
	c := cron.New()
	failCounter := 0

	backupFunction := func() {
		wg.Add(1)
		defer wg.Done()

		if m != nil {
			m.SetInProgress(true)
			defer m.SetInProgress(false)
		}

		start := time.Now()
		size, backupErr := backup.PgDumpToS3(ctx, s3, pg, config)
		duration := time.Since(start)

		if backupErr != nil {
			failCounter++
			slog.Error("backup failed", "error", backupErr, "failure_count", failCounter)

			if m != nil {
				m.RecordFailure(duration)
			}

			return
		}

		if m != nil {
			m.RecordSuccess(size, duration)
		}
	}

	err = c.Add("pgdumps3", config.CronSchedule, backupFunction)
	if err != nil {
		return err
	}

	c.Start()
	slog.Info("cron scheduler started", "schedule", config.CronSchedule)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	slog.Info("shutting down gracefully")

	c.Stop()

	if config.BackupBeforeShutdown {
		backupFunction()
	}

	wg.Wait()

	return nil
}

func getLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
