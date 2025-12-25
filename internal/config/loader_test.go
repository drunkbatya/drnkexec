package config

import (
	"testing"

	"github.com/drunkbatya/drnkexec/internal/model"
)

func TestInheritAlertRepeatIntervalFromDefaults(t *testing.T) {
	def := model.CheckDefaults{AlertRepeatIntervalSec: 45}
	alert := model.AlertManagerConfig{}
	inheritAlertRepeatInterval(def, &alert)
	applyAlertDefaults(&alert)
	if alert.RepeatIntervalSec != 45 {
		t.Fatalf("expected repeat interval 45, got %d", alert.RepeatIntervalSec)
	}
}

func TestInheritAlertRepeatIntervalRespectsExplicitValue(t *testing.T) {
	def := model.CheckDefaults{AlertRepeatIntervalSec: 10}
	alert := model.AlertManagerConfig{RepeatIntervalSec: 90}
	inheritAlertRepeatInterval(def, &alert)
	applyAlertDefaults(&alert)
	if alert.RepeatIntervalSec != 90 {
		t.Fatalf("expected repeat interval 90, got %d", alert.RepeatIntervalSec)
	}
}
