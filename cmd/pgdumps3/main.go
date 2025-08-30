package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/pocketbase/pocketbase/tools/cron"

	"github.com/advisoreeu/pgdumps3/internal/backup"
)

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

	wg := sync.WaitGroup{}
	c := cron.New()
	failCounter := 0

	backupFunction := func() {
		wg.Add(1)
		defer wg.Done()

		err = backup.PgDumpToS3(ctx, s3, pg, config)
		if err != nil {
			failCounter++
			slog.Error("backup failed", "error", err, "failure_count", failCounter)
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
