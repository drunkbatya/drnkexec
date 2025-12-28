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

	delBody := bytes.NewBufferString(`{"name":"maint","host_name":"alpha"}`)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/check/downtime", delBody)
	srv.handleDowntime(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d on delete", rr.Code)
	}
}

func TestLoginAndAuthFlow(t *testing.T) {
	srv, assignment := newTestServer(t)
	srv.state.Update(assignment, nrpeclient.StatusOK, "up")
	body := bytes.NewBufferString(`{"login":"admin","password":"secret"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", body)
	loginRR := httptest.NewRecorder()
	srv.handleLogin(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("unexpected status %d on login", loginRR.Code)
	}
	authCookie := sessionCookieFromRecorder(t, loginRR)

	handler := srv.requireAuth(srv.handleHosts)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts", nil)
	req.AddCookie(authCookie)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected authorized request, got %d", rr.Code)
	}
}

func TestProtectedEndpointRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t)
	handler := srv.requireAuth(srv.handleHosts)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing session, got %d", rr.Code)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	srv, _ := newTestServer(t)
	body := bytes.NewBufferString(`{"login":"admin","password":"bad"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", body)
	rr := httptest.NewRecorder()
	srv.handleLogin(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid creds, got %d", rr.Code)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	srv, assignment := newTestServer(t)
	srv.state.Update(assignment, nrpeclient.StatusOK, "up")
	body := bytes.NewBufferString(`{"login":"admin","password":"secret"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", body)
	loginRR := httptest.NewRecorder()
	srv.handleLogin(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("unexpected login status %d", loginRR.Code)
	}
	sessionCookie := sessionCookieFromRecorder(t, loginRR)

	logoutHandler := srv.requireAuth(srv.handleLogout)
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/user/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRR := httptest.NewRecorder()
	logoutHandler(logoutRR, logoutReq)
	if logoutRR.Code != http.StatusOK {
		t.Fatalf("logout failed with status %d", logoutRR.Code)
	}

	protected := srv.requireAuth(srv.handleHosts)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts", nil)
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()
	protected(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden after logout, got %d", rr.Code)
	}
}

func sessionCookieFromRecorder(t *testing.T, rr *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	resp := rr.Result()
	defer resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	t.Fatalf("session cookie missing")
	return nil
}

func newTestServer(t *testing.T) (*Server, model.CheckAssignment) {
	t.Helper()
	cfg := &model.Config{
		Hosts:  []model.HostConfig{{Name: "alpha", Hostname: "alpha"}},
		Checks: []model.CheckConfig{{Name: "svc"}},
	}
	cfg.LookupMaps.CheckAssignments = []model.CheckAssignment{{Host: &cfg.Hosts[0], Check: &cfg.Checks[0]}}
	logger := zaptest.NewLogger(t).Sugar()
	st := state.NewManager(logger, cfg)
	assignment := cfg.LookupMaps.CheckAssignments[0]
	dt := downtime.NewManager(logger)
	httpCfg := model.HTTPConfig{Host: "127.0.0.1", Port: 8080}
	admin := model.AdminConfig{Username: "admin", Password: "secret", SessionTTL: 60}
	return New(httpCfg, admin, st, dt, logger), assignment
}
