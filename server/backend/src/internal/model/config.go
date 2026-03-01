package model

type Config struct {
	AlertManager AlertManagerConfig
	Hosts        []HostConfig
	Checks       []CheckConfig
	Defaults     Defaults
	HTTP         HTTPConfig
	Admin        AdminConfig
	LookupMaps   LookupMaps
}

const (
	CheckTypeNRPE = "nrpe"
	CheckTypePing = "ping"
)

type AlertManagerConfig struct {
	Notifiers         []string       `yaml:"notifiers"`
	Telegram          TelegramConfig `yaml:"telegram"`
	RepeatIntervalSec int            `yaml:"repeat_interval_sec"`
}

type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type HostConfig struct {
	Name      string   `yaml:"name"`
	Hostname  string   `yaml:"hostname"`
	ResolveTo string   `yaml:"resolve_to"`
	CheckPing bool     `yaml:"check_ping"`
	Labels    []string `yaml:"labels"`
	Nrpe      HostNrpe `yaml:"nrpe"`
}

type HostNrpe struct {
	TLS NrpeTLSConfig `yaml:"tls"`
}

type NrpeTLSConfig struct {
	Enabled *bool `yaml:"enabled"`
}

type Defaults struct {
	Nrpe                   DefaultsNrpe    `yaml:"nrpe"`
	Scheduler              SchedulerConfig `yaml:"scheduler"`
}

type DefaultsNrpe struct {
	TLS NrpeTLSConfig `yaml:"tls"`
}

type SchedulerConfig struct {
	CheckIntervalSec        int `yaml:"check_interval_sec"`
	RetryIntervalSec        int `yaml:"retry_interval_sec"`
	MinFailBeforeAlert      int `yaml:"min_fail_before_alert"`
	MinSuccessBeforeResolve int `yaml:"min_success_before_resolved"`
	ExecutionTimeoutSec     int `yaml:"execution_timeout_sec"`
}

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

type HTTPConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type AdminConfig struct {
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	SessionTTL int    `yaml:"session_ttl_sec"`
}

type LookupMaps struct {
	ChecksByCheckLabel map[string][]*CheckConfig
	ChecksByHostname   map[string][]*CheckConfig
	HostByHostName     map[string]*HostConfig
	AnyHostChecks      []*CheckConfig
	CheckAssignments   []CheckAssignment
}

type CheckAssignment struct {
	Host  *HostConfig
	Check *CheckConfig
}
