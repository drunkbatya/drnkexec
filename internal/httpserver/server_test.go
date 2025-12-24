package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drunkbatya/drnkexec/internal/downtime"
	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/nrpeclient"
	"github.com/drunkbatya/drnkexec/internal/state"
	"go.uber.org/zap/zaptest"
)

func TestHandleHostsAndChecks(t *testing.T) {
	srv, assignment := newTestServer(t)
	srv.state.Update(assignment, nrpeclient.StatusOK, "up")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts", nil)
	srv.handleHosts(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rr.Code)
	}
	var hosts responseHosts
	if err := json.NewDecoder(rr.Body).Decode(&hosts); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(hosts.Hosts) != 1 || hosts.Hosts[0].Hostname != "alpha" {
		t.Fatalf("unexpected hosts %+v", hosts)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/checks?host_name=missing", nil)
	srv.handleChecks(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing host")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/check?host_name=alpha&check_name=svc", nil)
	srv.handleCheck(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rr.Code)
	}
}

func TestHandleDowntimeLifecycle(t *testing.T) {
	srv, _ := newTestServer(t)
	body := bytes.NewBufferString(`{"name":"maint","host_name":"alpha","duration":5}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/check/downtime", body)
	srv.handleDowntime(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected status %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/check/downtime", nil)
	srv.handleDowntime(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rr.Code)
	}
	var resp struct {
		Items []downtimeResponse `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Name != "maint" {
		t.Fatalf("unexpected downtime response %+v", resp)
	}
}

func newTestServer(t *testing.T) (*Server, model.CheckAssignment) {
	t.Helper()
	cfg := &model.Config{
		Hosts:  []model.HostConfig{{Name: "alpha", Hostname: "alpha"}},
		Checks: []model.CheckConfig{{Name: "svc"}},
	}
	cfg.LookupMaps.CheckAssignments = []model.CheckAssignment{{Host: &cfg.Hosts[0], Check: &cfg.Checks[0]}}
	st := state.NewManager(cfg)
	assignment := cfg.LookupMaps.CheckAssignments[0]
	logger := zaptest.NewLogger(t).Sugar()
	return &Server{state: st, downtime: downtime.NewManager(), logger: logger}, assignment
}
