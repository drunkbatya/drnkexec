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
	if info.Status != model.StatusUnknown {
		t.Fatalf("expected status unknown, got %s", info.Status)
	}
}

func TestManagerUpdateAndQueries(t *testing.T) {
	cfg := testConfig()
	logger := zaptest.NewLogger(t).Sugar()
	m := NewManager(logger, cfg)
	assignment := cfg.LookupMaps.CheckAssignments[0]
	m.Update(assignment, nrpeclient.StatusWarning, " some output \n", 2)

	info, ok := m.Check("alpha", "check-alpha")
	if !ok {
		t.Fatalf("expected check info")
	}
	if info.Status != model.StatusWarning {
		t.Fatalf("expected warning, got %s", info.Status)
	}
	if info.Output != "some output" {
		t.Fatalf("unexpected output %q", info.Output)
	}
	if info.FailCount != 2 {
		t.Fatalf("expected fail count 2, got %d", info.FailCount)
	}
	if info.FailThreshold != assignment.Check.MinFailBeforeAlert {
		t.Fatalf("expected fail threshold %d, got %d", assignment.Check.MinFailBeforeAlert, info.FailThreshold)
	}

	checks, ok := m.Checks("", nil)
	if !ok || len(checks) != 2 {
		t.Fatalf("expected checks slice, got %v", checks)
	}

	if _, ok := m.Checks("missing", nil); ok {
		t.Fatalf("expected false for unknown host")
	}

	summary, ok := m.HostSummary("alpha")
	if !ok {
		t.Fatalf("expected host summary")
	}
	if summary.Warning != 1 {
		t.Fatalf("expected warning count 1, got %+v", summary)
	}
	if summary.CheckCount != 1 {
		t.Fatalf("expected check count 1, got %+v", summary)
	}
}

func TestCheckSummaries(t *testing.T) {
	cfg := testConfig()
	logger := zaptest.NewLogger(t).Sugar()
	m := NewManager(logger, cfg)
	assignments := cfg.LookupMaps.CheckAssignments
	m.Update(assignments[0], nrpeclient.StatusCritical, "bad", 1)
	m.Update(assignments[1], nrpeclient.StatusOK, "ok", 0)

	summaries := m.CheckSummaries()
	if len(summaries) != 2 {
		t.Fatalf("expected 2 check summaries, got %d", len(summaries))
	}
	var alphaSummary *CheckSummary
	for i := range summaries {
		if summaries[i].CheckName == "check-alpha" {
			alphaSummary = &summaries[i]
			break
		}
	}
	if alphaSummary == nil {
		t.Fatalf("missing check-alpha summary")
	}
	if alphaSummary.HostCount != 1 || alphaSummary.Critical != 1 {
		t.Fatalf("unexpected alpha summary %+v", alphaSummary)
	}
}

func TestCheckSummariesStatusFilter(t *testing.T) {
	cfg := testConfig()
	logger := zaptest.NewLogger(t).Sugar()
	m := NewManager(logger, cfg)
	assignments := cfg.LookupMaps.CheckAssignments
	m.Update(assignments[0], nrpeclient.StatusCritical, "bad", 1)
	m.Update(assignments[1], nrpeclient.StatusOK, "ok", 0)

	critical := m.CheckSummariesFiltered("", []model.Status{model.StatusCritical})
	if len(critical) != 1 || critical[0].CheckName != "check-alpha" {
		t.Fatalf("expected only check-alpha, got %+v", critical)
	}

	warnings := m.CheckSummariesFiltered("", []model.Status{model.StatusWarning})
	if len(warnings) != 0 {
		t.Fatalf("expected no warning summaries, got %+v", warnings)
	}
}

func TestHostSummariesStatusFilter(t *testing.T) {
	cfg := testConfig()
	logger := zaptest.NewLogger(t).Sugar()
	m := NewManager(logger, cfg)
	assignments := cfg.LookupMaps.CheckAssignments
	m.Update(assignments[0], nrpeclient.StatusCritical, "bad", 1)
	m.Update(assignments[1], nrpeclient.StatusOK, "ok", 0)

	criticalOnly := m.HostSummariesFiltered("", []model.Status{model.StatusCritical})
	if len(criticalOnly) != 1 || criticalOnly[0].Hostname != "alpha" {
		t.Fatalf("expected only alpha host, got %+v", criticalOnly)
	}

	all := m.HostSummariesFiltered("", []model.Status{model.StatusCritical, model.StatusOK})
	if len(all) != 2 {
		t.Fatalf("expected both hosts, got %d", len(all))
	}
}

func TestChecksStatusFilter(t *testing.T) {
	cfg := testConfig()
	logger := zaptest.NewLogger(t).Sugar()
	m := NewManager(logger, cfg)
	assignments := cfg.LookupMaps.CheckAssignments
	m.Update(assignments[0], nrpeclient.StatusCritical, "bad", 1)
	m.Update(assignments[1], nrpeclient.StatusOK, "ok", 0)

	checks, ok := m.Checks("", []model.Status{model.StatusCritical})
	if !ok || len(checks) != 1 || checks[0].Hostname != "alpha" {
		t.Fatalf("expected only alpha critical check, got %+v", checks)
	}

	checks, ok = m.Checks("beta", []model.Status{model.StatusCritical})
	if !ok {
		t.Fatalf("expected beta host lookup")
	}
	if len(checks) != 0 {
		t.Fatalf("expected no critical checks for beta, got %+v", checks)
	}

	if _, ok := m.Checks("missing", []model.Status{model.StatusCritical}); ok {
		t.Fatalf("expected missing host to return false")
	}
}

func testConfig() *model.Config {
	cfg := &model.Config{
		Hosts: []model.HostConfig{
			{Name: "Alpha", Hostname: "alpha"},
			{Name: "Beta", Hostname: "beta"},
		},
		Checks: []model.CheckConfig{
			{Name: "check-alpha", MinFailBeforeAlert: 3},
			{Name: "check-beta", MinFailBeforeAlert: 2},
		},
	}
	cfg.LookupMaps.CheckAssignments = []model.CheckAssignment{
		{Host: &cfg.Hosts[0], Check: &cfg.Checks[0]},
		{Host: &cfg.Hosts[1], Check: &cfg.Checks[1]},
	}
	return cfg
}
