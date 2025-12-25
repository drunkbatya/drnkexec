package model

// Config represents the fully parsed configuration with lookup tables for fast scheduling operations.
type Config struct {
	AlertManager AlertManagerConfig
	Hosts        []HostConfig
	Checks       []CheckConfig
	Defaults     CheckDefaults
	HTTP         HTTPConfig
	LookupMaps   LookupMaps
}

const (
	CheckTypeNRPE = "nrpe"
	CheckTypePing = "ping"
)

// AlertManagerConfig holds integration options.
type AlertManagerConfig struct {
	Notifiers         []string       `yaml:"notifiers"`
	Telegram          TelegramConfig `yaml:"telegram"`
	RepeatIntervalSec int            `yaml:"repeat_interval_sec"`
}

// TelegramConfig is used for Telegram alerting options.
type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

// HostConfig describes a host entry from the configuration.
type HostConfig struct {
	Name      string   `yaml:"name"`
	Hostname  string   `yaml:"hostname"`
	ResolveTo string   `yaml:"resolve_to"`
	CheckPing bool     `yaml:"check_pind"`
	Labels    []string `yaml:"labels"`
}

// CheckConfig represents a single NRPE check definition.
type CheckConfig struct {
	Name                    string   `yaml:"name"`
	Command                 string   `yaml:"command"`
	Arguments               []string `yaml:"args"`
	Type                    string   `yaml:"type"`
	MatchLabels             []string `yaml:"match_labels"`
	MatchHosts              []string `yaml:"match_hosts"`
	AnyHost                 bool     `yaml:"any_host"`
	ExecutionTimeoutSec     int      `yaml:"execution_timeout_sec"`
	CheckIntervalSec        int      `yaml:"check_interval_sec"`
	RetryIntervalSec        int      `yaml:"retry_interval_sec"`
	MinFailBeforeAlert      int      `yaml:"min_fail_before_alert"`
	MinSuccessBeforeResolve int      `yaml:"min_success_before_resolved"`
}

// CheckDefaults is the defaults section of the YAML.
type CheckDefaults struct {
	AlertRepeatIntervalSec  int `yaml:"alert_repeat_interval_sec"`
	CheckIntervalSec        int `yaml:"check_interval_sec"`
	RetryIntervalSec        int `yaml:"retry_interval_sec"`
	MinFailBeforeAlert      int `yaml:"min_fail_before_alert"`
	MinSuccessBeforeResolve int `yaml:"min_success_before_resolved"`
	ExecutionTimeoutSec     int `yaml:"execution_timeout_sec"`
}

type HTTPConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// LookupMaps houses helper maps for fast lookup during scheduling.
type LookupMaps struct {
	ChecksByCheckLabel map[string][]*CheckConfig
	ChecksByHostname   map[string][]*CheckConfig
	HostByHostName     map[string]*HostConfig
	AnyHostChecks      []*CheckConfig
	CheckAssignments   []CheckAssignment
}

// CheckAssignment binds a check to a host once the config is compiled.
type CheckAssignment struct {
	Host  *HostConfig
	Check *CheckConfig
}
