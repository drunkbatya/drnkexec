package config

import (
	"testing"

	"github.com/drunkbatya/drnkexec/internal/model"
)

func TestInheritAlertRepeatIntervalFromDefaults(t *testing.T) {
	def := model.Defaults{AlertRepeatIntervalSec: 45}
	alert := model.AlertManagerConfig{}
	inheritAlertRepeatInterval(def, &alert)
	applyAlertDefaults(&alert)
	if alert.RepeatIntervalSec != 45 {
		t.Fatalf("expected repeat interval 45, got %d", alert.RepeatIntervalSec)
	}
}

func TestInheritAlertRepeatIntervalRespectsExplicitValue(t *testing.T) {
	def := model.Defaults{AlertRepeatIntervalSec: 10}
	alert := model.AlertManagerConfig{RepeatIntervalSec: 90}
	inheritAlertRepeatInterval(def, &alert)
	applyAlertDefaults(&alert)
	if alert.RepeatIntervalSec != 90 {
		t.Fatalf("expected repeat interval 90, got %d", alert.RepeatIntervalSec)
	}
}

func TestApplyDefaultSectionSetsNRPETLS(t *testing.T) {
	var def model.Defaults
	applyDefaultsSection(&def)
	if def.Nrpe.TLS.Enabled == nil || !*def.Nrpe.TLS.Enabled {
		t.Fatalf("nrpe tls should default to enabled")
	}
}

func TestHostInheritsNRPETLSDefault(t *testing.T) {
	def := model.Defaults{}
	applyDefaultsSection(&def)
	cfg := &model.Config{Defaults: def, Hosts: []model.HostConfig{{Name: "h", Hostname: "h"}}}
	if err := compileMaps(cfg); err != nil {
		t.Fatalf("compileMaps failed: %v", err)
	}
	if cfg.Hosts[0].Nrpe.TLS.Enabled == nil || !*cfg.Hosts[0].Nrpe.TLS.Enabled {
		t.Fatalf("host should inherit tls enabled")
	}
	manual := false
	cfg.Hosts[0].Nrpe.TLS.Enabled = &manual
	if err := compileMaps(cfg); err != nil {
		t.Fatalf("compileMaps failed: %v", err)
	}
	if cfg.Hosts[0].Nrpe.TLS.Enabled == nil || *cfg.Hosts[0].Nrpe.TLS.Enabled {
		t.Fatalf("host override should persist")
	}
}

func TestSchedulerDefaultsApplied(t *testing.T) {
	var def model.Defaults
	applyDefaultsSection(&def)
	sched := def.Scheduler
	if sched.CheckIntervalSec != defaultCheckIntervalSec || sched.ExecutionTimeoutSec != defaultExecutionTimeoutSec {
		t.Fatalf("scheduler defaults not applied: %+v", sched)
	}
}
