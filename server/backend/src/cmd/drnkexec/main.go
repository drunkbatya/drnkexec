package main

//go:generate sh -c "PATH=$PATH:$(go env GOPATH)/bin swag init --parseInternal --parseDependency --generalInfo main.go --dir .,../../internal/httpserver --output ../../docs"

// @title Drnkexec Monitoring API
// @version 1.0
// @description API for managing monitoring checks, downtimes and sessions.
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey SessionAuth
// @in cookie
// @name drnkexec_session

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/drunkbatya/drnkexec/docs"
	"github.com/drunkbatya/drnkexec/internal/alerts"
	"github.com/drunkbatya/drnkexec/internal/config"
	"github.com/drunkbatya/drnkexec/internal/downtime"
	"github.com/drunkbatya/drnkexec/internal/httpserver"
	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/nrpeclient"
	"github.com/drunkbatya/drnkexec/internal/pinger"
	"github.com/drunkbatya/drnkexec/internal/scheduler"
	"github.com/drunkbatya/drnkexec/internal/state"
	"github.com/drunkbatya/drnkexec/internal/version"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to the YAML config file")
	noAuth := flag.Bool("noauth", false, "Disable authentication (development only)")
	showVersion := flag.Bool("version", false, "Print build information and exit")
	flag.Parse()

	if *showVersion {
		info, err := version.Printable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "version: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(info)
		return
	}

	zapLogger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() { _ = zapLogger.Sync() }()
	logger := zapLogger.Sugar()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	notifiers := buildNotifiers(cfg, logger)
	alertManager := alerts.NewManager(logger, notifiers, time.Duration(cfg.AlertManager.RepeatIntervalSec)*time.Second)

	nrpeClient := nrpeclient.NewClient(logger)
	pingChecker := pinger.NewChecker(logger)
	stateManager := state.NewManager(logger, cfg)
	downtimeManager := downtime.NewManager(logger)
	sched := scheduler.NewScheduler(logger, nrpeClient, alertManager, pingChecker, stateManager, downtimeManager)
	apiServer := httpserver.New(cfg.HTTP, cfg.Admin, stateManager, downtimeManager, sched, logger, *noAuth)
	go func() {
		if err := apiServer.Start(ctx); err != nil {
			logger.Fatalf("http server error: %v", err)
		}
	}()
	logger.Infof("drnkexec started checks=%d", len(cfg.LookupMaps.CheckAssignments))

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
