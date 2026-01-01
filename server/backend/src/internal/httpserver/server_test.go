package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drunkbatya/drnkexec/internal/downtime"
	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/nrpeclient"
	"github.com/drunkbatya/drnkexec/internal/state"
	"go.uber.org/zap/zaptest"
)

type stubRunner struct {
	host  string
	check string
	err   error
	calls int
}

func (s *stubRunner) TriggerCheck(hostname, checkname string) error {
	s.host = hostname
	s.check = checkname
	s.calls++
	return s.err
}

func TestHandleHostsAndChecks(t *testing.T) {
	srv, assignment, _ := newTestServer(t)
	srv.state.Update(assignment, nrpeclient.StatusOK, "up", 0)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/hosts", nil)
	srv.handleHosts(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rr.Code)
	}
	var hosts responseHosts
	if err := json.NewDecoder(rr.Body).Decode(&hosts); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(hosts.Hosts) != 1 || hosts.Hosts[0].Hostname != "alpha" || hosts.Count <= 0 || hosts.Total != 1 {
		t.Fatalf("unexpected hosts %+v", hosts)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/checks?host_name=missing", nil)
	srv.handleChecks(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing host")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/check?host_name=alpha&check_name=svc", nil)
	srv.handleCheck(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rr.Code)
	}
}

func TestHandleDowntimeLifecycle(t *testing.T) {
	srv, _, _ := newTestServer(t)
	body := bytes.NewBufferString(`{"name":"maint","host_name":"alpha","duration":5}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/downtime", body)
	srv.handleDowntime(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected status %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/downtime", nil)
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
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/downtime", delBody)
	srv.handleDowntime(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d on delete", rr.Code)
	}
}

func TestHandleCheckNowSuccess(t *testing.T) {
	srv, _, runner := newTestServer(t)
	body := bytes.NewBufferString(`{"host_name":"alpha","check_name":"svc"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/check/now", body)
	srv.handleCheckNow(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("unexpected status %d", rr.Code)
	}
	if runner.calls != 1 || runner.host != "alpha" || runner.check != "svc" {
		t.Fatalf("runner was not triggered correctly: %+v", runner)
	}
}

func TestHandleCheckNowValidation(t *testing.T) {
	srv, _, runner := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/check/now", bytes.NewBufferString(`{}`))
	srv.handleCheckNow(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if runner.calls != 0 {
		t.Fatalf("runner should not be called on validation error")
	}
}

func TestHandleCheckNowError(t *testing.T) {
	srv, _, runner := newTestServer(t)
	runner.err = errors.New("missing")
	body := bytes.NewBufferString(`{"host_name":"alpha","check_name":"svc"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/check/now", body)
	srv.handleCheckNow(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestAppHealth(t *testing.T) {
	srv, _, _ := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/health", nil)
	srv.handleAppHealth(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rr.Code)
	}
	var resp appHealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("unexpected response %+v", resp)
	}
}

func TestAppVersion(t *testing.T) {
	srv, _, _ := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/version", nil)
	srv.handleAppVersion(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rr.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["name"] == "" {
		t.Fatalf("expected name in version response, got %+v", resp)
	}
}

func TestLoginAndAuthFlow(t *testing.T) {
	srv, assignment, _ := newTestServer(t)
	srv.state.Update(assignment, nrpeclient.StatusOK, "up", 0)
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
	srv, _, _ := newTestServer(t)
	handler := srv.requireAuth(srv.handleHosts)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing session, got %d", rr.Code)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	srv, _, _ := newTestServer(t)
	body := bytes.NewBufferString(`{"login":"admin","password":"bad"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", body)
	rr := httptest.NewRecorder()
	srv.handleLogin(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid creds, got %d", rr.Code)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	srv, assignment, _ := newTestServer(t)
	srv.state.Update(assignment, nrpeclient.StatusOK, "up", 0)
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

func TestRequireAuthHandler(t *testing.T) {
	srv, _, _ := newTestServer(t)
	protected := srv.requireAuthHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/hosts", nil)
	protected.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without auth, got %d", rr.Code)
	}

	loginBody := bytes.NewBufferString(`{"login":"admin","password":"secret"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", loginBody)
	loginRR := httptest.NewRecorder()
	srv.handleLogin(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("login failed with %d", loginRR.Code)
	}
	sessionCookie := sessionCookieFromRecorder(t, loginRR)

	rr = httptest.NewRecorder()
	authReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/hosts", nil)
	authReq.AddCookie(sessionCookie)
	protected.ServeHTTP(rr, authReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected OK with auth, got %d", rr.Code)
	}
}

func TestRequireAuthRedirect(t *testing.T) {
	srv, _, _ := newTestServer(t)
	redirected := srv.requireAuthRedirect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "/login")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/docs/", nil)
	redirected.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected redirect without auth, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %s", loc)
	}

	loginBody := bytes.NewBufferString(`{"login":"admin","password":"secret"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", loginBody)
	loginRR := httptest.NewRecorder()
	srv.handleLogin(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("login failed with %d", loginRR.Code)
	}
	sessionCookie := sessionCookieFromRecorder(t, loginRR)

	rr = httptest.NewRecorder()
	authReq := httptest.NewRequest(http.MethodGet, "/api/docs/", nil)
	authReq.AddCookie(sessionCookie)
	redirected.ServeHTTP(rr, authReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected OK with auth, got %d", rr.Code)
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

func newTestServer(t *testing.T) (*Server, model.CheckAssignment, *stubRunner) {
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
	runner := &stubRunner{}
	return New(httpCfg, admin, st, dt, runner, logger, false), assignment, runner
}
