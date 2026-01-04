package downtime

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

func TestAddAndListScopes(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	mgr := NewManager(logger)
	now := time.Now()
	if _, err := mgr.Add("host1", "check1", "c1", now.Add(-time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatalf("add check failed: %v", err)
	}
	if _, err := mgr.AddHost("host2", "h1", now.Add(-time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatalf("add host failed: %v", err)
	}
	if _, err := mgr.Add("", "", "global", now.Add(-time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatalf("add global failed: %v", err)
	}
	list := mgr.List("", "", "", now)
	if len(list) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(list))
	}
	if !mgr.Active("host1", "check1", now) {
		t.Fatalf("expected active for host1/check1")
	}
	if !mgr.Active("host2", "checkX", now) {
		t.Fatalf("expected host level active")
	}
	if !mgr.Active("any", "any", now) {
		t.Fatalf("expected global active")
	}
}

func TestRemoveEntries(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	mgr := NewManager(logger)
	now := time.Now()
	_, _ = mgr.Add("", "", "global", now.Add(-time.Minute), now.Add(time.Minute))
	_, _ = mgr.Add("host", "", "host", now.Add(-time.Minute), now.Add(time.Minute))
	_, _ = mgr.Add("host", "check", "check", now.Add(-time.Minute), now.Add(time.Minute))

	if !mgr.Remove("", "", "global") {
		t.Fatalf("expected remove global")
	}
	if mgr.Remove("", "", "global") {
		t.Fatalf("expected second remove to fail")
	}
	if !mgr.Remove("host", "", "host") {
		t.Fatalf("expected remove host")
	}
	if !mgr.Remove("host", "check", "check") {
		t.Fatalf("expected remove check")
	}
}

func TestRemoveByName(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	mgr := NewManager(logger)
	now := time.Now()
	if _, err := mgr.Add("hostA", "checkA", "entry-check", now.Add(-time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatalf("add check failed: %v", err)
	}
	if _, err := mgr.AddHost("hostB", "entry-host", now.Add(-time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatalf("add host failed: %v", err)
	}
	if _, err := mgr.Add("", "", "entry-global", now.Add(-time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatalf("add global failed: %v", err)
	}

	if !mgr.RemoveByName("entry-check") {
		t.Fatalf("expected remove by name for check scope")
	}
	if !mgr.RemoveByName("entry-host") {
		t.Fatalf("expected remove by name for host scope")
	}
	if !mgr.RemoveByName("entry-global") {
		t.Fatalf("expected remove by name for global scope")
	}
	if mgr.RemoveByName("missing") {
		t.Fatalf("expected missing name removal to fail")
	}
}
