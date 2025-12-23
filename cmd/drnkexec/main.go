package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"
	"time"

	"github.com/drunkbatya/drnkexec/internal/alerts"
	"github.com/drunkbatya/drnkexec/internal/config"
	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/nrpeclient"
	"github.com/drunkbatya/drnkexec/internal/pinger"
	"github.com/drunkbatya/drnkexec/internal/scheduler"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to the YAML config file")
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()
	sugar := logger.Sugar()

	cfg, err := config.Load(*configPath)
	if err != nil {
		sugar.Fatalf("load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	notifiers := buildNotifiers(cfg, sugar)
	alertManager := alerts.NewManager(sugar, notifiers, time.Duration(cfg.Defaults.AlertRepeatIntervalSec)*time.Second)

	nrpeClient := nrpeclient.NewClient(sugar)
	pingChecker := pinger.NewChecker(sugar)

	sched := scheduler.NewScheduler(sugar, nrpeClient, alertManager, pingChecker)
	sugar.Infof("drnkexec started checks=%d", len(cfg.LookupMaps.CheckAssignments))

	sched.Run(ctx, cfg)
}

func buildNotifiers(cfg *model.Config, logger *zap.SugaredLogger) []alerts.Notifier {
	var notifiers []alerts.Notifier
	for _, name := range cfg.AlertManager.Notifiers {
		switch name {
		case "log":
			if n := alerts.NewLoggerNotifier(logger); n != nil {
				notifiers = append(notifiers, n)
			}
		case "telegram":
			if n := alerts.NewTelegramNotifier(cfg.AlertManager.Telegram.BotToken, cfg.AlertManager.Telegram.ChatID); n != nil {
				notifiers = append(notifiers, n)
			}
		default:
			logger.Warnf("notifier %s is not supported", name)
		}
	}
	return notifiers
}
