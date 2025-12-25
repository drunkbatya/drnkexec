package state

import (
	"testing"

	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/nrpeclient"
	"go.uber.org/zap/zaptest"
)

func TestManagerInitializationSeedsUnknownChecks(t *testing.T) {
	cfg := testConfig()
	logger := zaptest.NewLogger(t).Sugar()
	m := NewManager(logger, cfg)

	if summaries := m.HostSummaries(); len(summaries) != 2 {
		t.Fatalf("expected 2 host summaries, got %d", len(summaries))
	}
	info, ok := m.Check("alpha", "check-alpha")
	if !ok {
		t.Fatalf("expected check info")
	}
	if info.Status != StatusUnknown {
		t.Fatalf("expected status unknown, got %s", info.Status)
	}
}

func TestManagerUpdateAndQueries(t *testing.T) {
	cfg := testConfig()
	logger := zaptest.NewLogger(t).Sugar()
	m := NewManager(logger, cfg)
	assignment := cfg.LookupMaps.CheckAssignments[0]
	m.Update(assignment, nrpeclient.StatusWarning, " some output \n")

	info, ok := m.Check("alpha", "check-alpha")
	if !ok {
		t.Fatalf("expected check info")
	}
	if info.Status != StatusWarning {
		t.Fatalf("expected warning, got %s", info.Status)
	}
	if info.Output != "some output" {
		t.Fatalf("unexpected output %q", info.Output)
	}

	checks, ok := m.Checks("")
	if !ok || len(checks) != 2 {
		t.Fatalf("expected checks slice, got %v", checks)
	}

	if _, ok := m.Checks("missing"); ok {
		t.Fatalf("expected false for unknown host")
	}

	summary, ok := m.HostSummary("alpha")
	if !ok {
		t.Fatalf("expected host summary")
	}
	if summary.Warning != 1 {
		t.Fatalf("expected warning count 1, got %+v", summary)
	}
}

func testConfig() *model.Config {
	cfg := &model.Config{
		Hosts: []model.HostConfig{
			{Name: "Alpha", Hostname: "alpha"},
			{Name: "Beta", Hostname: "beta"},
		},
		Checks: []model.CheckConfig{
			{Name: "check-alpha"},
			{Name: "check-beta"},
		},
	}
	cfg.LookupMaps.CheckAssignments = []model.CheckAssignment{
		{Host: &cfg.Hosts[0], Check: &cfg.Checks[0]},
		{Host: &cfg.Hosts[1], Check: &cfg.Checks[1]},
	}
	return cfg
}
