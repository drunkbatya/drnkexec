package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/drunkbatya/drnkexec/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	defaultAlertRepeatIntervalSec  = 120
	defaultCheckIntervalSec        = 60
	defaultRetryIntervalSec        = 15
	defaultRepeatAlertIntervalSec  = 60
	defaultMinFailBeforeAlert      = 1
	defaultMinSuccessBeforeResolve = 1
	defaultExecutionTimeoutSec     = 10
	defaultHTTPHost                = "0.0.0.0"
	defaultHTTPPort                = 8080
)

// Load reads the YAML config from disk, applies defaults and builds lookup maps.
func Load(path string) (*model.Config, error) {
	raw, err := readFile(path)
	if err != nil {
		return nil, err
	}

	diskCfg := struct {
		AlertManager model.AlertManagerConfig `yaml:"alertmanager"`
		Hosts        []model.HostConfig       `yaml:"hosts"`
		Checks       []model.CheckConfig      `yaml:"checks"`
		Defaults     model.CheckDefaults      `yaml:"defaults"`
		HTTP         model.HTTPConfig         `yaml:"http"`
	}{}

	if err := yaml.Unmarshal(raw, &diskCfg); err != nil {
		return nil, wrapYAMLError(path, err)
	}

	applyDefaultSection(&diskCfg.Defaults)
	applyAlertDefaults(&diskCfg.AlertManager)
	applyHTTPDefaults(&diskCfg.HTTP)
	injectPingChecks(&diskCfg.Checks, diskCfg.Hosts, diskCfg.Defaults)
	applyCheckDefaults(diskCfg.Checks, diskCfg.Defaults)

	cfg := &model.Config{
		AlertManager: diskCfg.AlertManager,
		Hosts:        diskCfg.Hosts,
		Checks:       diskCfg.Checks,
		Defaults:     diskCfg.Defaults,
		HTTP:         diskCfg.HTTP,
	}

	if err := compileMaps(cfg); err != nil {
		return nil, err
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("config %s not found", path)
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return data, nil
}

func applyAlertDefaults(cfg *model.AlertManagerConfig) {
	if len(cfg.Notifiers) == 0 {
		cfg.Notifiers = []string{"log"}
		return
	}
	for i := range cfg.Notifiers {
		cfg.Notifiers[i] = strings.ToLower(cfg.Notifiers[i])
	}
}

func applyHTTPDefaults(cfg *model.HTTPConfig) {
	if cfg.Host == "" {
		cfg.Host = defaultHTTPHost
	}
	if cfg.Port == 0 {
		cfg.Port = defaultHTTPPort
	}
}

func wrapYAMLError(path string, err error) error {
	var typeErr *yaml.TypeError
	if errors.As(err, &typeErr) && len(typeErr.Errors) > 0 {
		return fmt.Errorf("decode config %s: %s", path, strings.Join(typeErr.Errors, "; "))
	}
	return fmt.Errorf("decode config %s: %w", path, err)
}

func applyDefaultSection(def *model.CheckDefaults) {
	if def.AlertRepeatIntervalSec <= 0 {
		def.AlertRepeatIntervalSec = defaultAlertRepeatIntervalSec
	}
	if def.CheckIntervalSec <= 0 {
		def.CheckIntervalSec = defaultCheckIntervalSec
	}
	if def.RetryIntervalSec <= 0 {
		def.RetryIntervalSec = defaultRetryIntervalSec
	}
	if def.RepeatAlertIntervalSec <= 0 {
		def.RepeatAlertIntervalSec = defaultRepeatAlertIntervalSec
	}
	if def.MinFailBeforeAlert <= 0 {
		def.MinFailBeforeAlert = defaultMinFailBeforeAlert
	}
	if def.MinSuccessBeforeResolve <= 0 {
		def.MinSuccessBeforeResolve = defaultMinSuccessBeforeResolve
	}
	if def.ExecutionTimeoutSec <= 0 {
		def.ExecutionTimeoutSec = defaultExecutionTimeoutSec
	}
}

func validateAlertManager(cfg *model.AlertManagerConfig) error {
	for _, notifier := range cfg.Notifiers {
		switch notifier {
		case "log":
		case "telegram":
			if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
				return fmt.Errorf("telegram notifier requires bot_token and chat_id")
			}
		default:
			return fmt.Errorf("unsupported notifier %s", notifier)
		}
	}
	return nil
}

func applyCheckDefaults(checks []model.CheckConfig, def model.CheckDefaults) {
	for i := range checks {
		check := &checks[i]
		if check.Type == "" {
			check.Type = model.CheckTypeNRPE
		}
		if check.CheckIntervalSec <= 0 {
			check.CheckIntervalSec = def.CheckIntervalSec
		}
		if check.RetryIntervalSec <= 0 {
			check.RetryIntervalSec = def.RetryIntervalSec
		}
		if check.RepeatAlertIntervalSec <= 0 {
			check.RepeatAlertIntervalSec = def.RepeatAlertIntervalSec
		}
		if check.MinFailBeforeAlert <= 0 {
			check.MinFailBeforeAlert = def.MinFailBeforeAlert
		}
		if check.MinSuccessBeforeResolve <= 0 {
			check.MinSuccessBeforeResolve = def.MinSuccessBeforeResolve
		}
		if check.ExecutionTimeoutSec <= 0 {
			check.ExecutionTimeoutSec = def.ExecutionTimeoutSec
		}
	}
}

func injectPingChecks(checks *[]model.CheckConfig, hosts []model.HostConfig, def model.CheckDefaults) {
	for _, host := range hosts {
		if !host.CheckPing {
			continue
		}
		pingCheck := model.CheckConfig{
			Name:                    "Ping",
			Command:                 "builtin_ping",
			Type:                    model.CheckTypePing,
			MatchHosts:              []string{host.Hostname},
			ExecutionTimeoutSec:     def.ExecutionTimeoutSec,
			CheckIntervalSec:        def.CheckIntervalSec,
			RetryIntervalSec:        def.RetryIntervalSec,
			RepeatAlertIntervalSec:  def.RepeatAlertIntervalSec,
			MinFailBeforeAlert:      def.MinFailBeforeAlert,
			MinSuccessBeforeResolve: def.MinSuccessBeforeResolve,
		}
		*checks = append(*checks, pingCheck)
	}
}

func compileMaps(cfg *model.Config) error {
	lookup := model.LookupMaps{
		ChecksByCheckLabel: make(map[string][]*model.CheckConfig),
		ChecksByHostname:   make(map[string][]*model.CheckConfig, len(cfg.Hosts)),
		HostByHostName:     make(map[string]*model.HostConfig, len(cfg.Hosts)),
	}

	for i := range cfg.Checks {
		check := &cfg.Checks[i]
		if check.AnyHost {
			lookup.AnyHostChecks = append(lookup.AnyHostChecks, check)
		}
		for _, label := range check.MatchLabels {
			lookup.ChecksByCheckLabel[label] = append(lookup.ChecksByCheckLabel[label], check)
		}
		for _, hostname := range check.MatchHosts {
			lookup.ChecksByHostname[hostname] = append(lookup.ChecksByHostname[hostname], check)
		}
	}

	for i := range cfg.Hosts {
		host := &cfg.Hosts[i]
		if host.Hostname == "" {
			return fmt.Errorf("host entry %s is missing hostname", host.Name)
		}
		lookup.HostByHostName[host.Hostname] = host
		assigned := make([]*model.CheckConfig, 0)
		seen := make(map[string]struct{})

		addCheck := func(c *model.CheckConfig) {
			if c == nil {
				return
			}
			if _, ok := seen[c.Name]; ok {
				return
			}
			seen[c.Name] = struct{}{}
			assigned = append(assigned, c)
			lookup.CheckAssignments = append(lookup.CheckAssignments, model.CheckAssignment{
				Host:  host,
				Check: c,
			})
		}

		for _, c := range lookup.ChecksByHostname[host.Hostname] {
			addCheck(c)
		}
		for _, label := range host.Labels {
			for _, c := range lookup.ChecksByCheckLabel[label] {
				addCheck(c)
			}
		}
		for _, c := range lookup.AnyHostChecks {
			addCheck(c)
		}

		lookup.ChecksByHostname[host.Hostname] = assigned
	}

	cfg.LookupMaps = lookup
	return nil
}

func validateConfig(cfg *model.Config) error {
	if cfg.HTTP.Port <= 0 {
		return fmt.Errorf("http port must be positive")
	}
	if err := validateAlertManager(&cfg.AlertManager); err != nil {
		return err
	}
	if cfg.AlertManager.Telegram.BotToken == "" || cfg.AlertManager.Telegram.ChatID == "" {

	}
	if len(cfg.Hosts) == 0 {
		return fmt.Errorf("config defines no hosts")
	}
	if len(cfg.Checks) == 0 {
		return fmt.Errorf("config defines no checks")
	}

	for _, check := range cfg.Checks {
		if check.Name == "" {
			return fmt.Errorf("a check is missing name")
		}
		if check.Command == "" {
			return fmt.Errorf("check %s missing command", check.Name)
		}
		switch check.Type {
		case model.CheckTypeNRPE, model.CheckTypePing:
		default:
			return fmt.Errorf("check %s has unsupported type %s", check.Name, check.Type)
		}
		if len(check.MatchLabels) == 0 && len(check.MatchHosts) == 0 && !check.AnyHost {
			return fmt.Errorf("check %s must match at least one host/label or any_host", check.Name)
		}
	}

	return nil
}
