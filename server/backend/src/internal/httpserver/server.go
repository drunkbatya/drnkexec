package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/drunkbatya/drnkexec/internal/downtime"
	"github.com/drunkbatya/drnkexec/internal/httpserver/session"
	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/state"
	"github.com/drunkbatya/drnkexec/internal/version"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
)

type Server struct {
	addr       string
	logger     *zap.SugaredLogger
	state      *state.Manager
	downtime   *downtime.Manager
	admin      model.AdminConfig
	sessions   *session.Manager
	sessionTTL time.Duration
	runner     CheckRunner
	noAuth     bool
}

const (
	sessionCookieName = "drnkexec_session"
	defaultPageSize   = 20
)

type responseHosts struct {
	Hosts []state.HostSummary `json:"hosts"`
	Count int                 `json:"count"`
	Total int                 `json:"total"`
}

type responseChecks struct {
	Items []state.CheckInfo `json:"items"`
	Count int               `json:"count"`
	Total int               `json:"total"`
}

type responseCheck struct {
	Check state.CheckInfo `json:"check"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type appHealthResponse struct {
	Status string `json:"status"`
}

type CheckRunner interface {
	TriggerCheck(hostname, checkname string) error
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type loginResponse struct {
	Login string `json:"login"`
}

type logoutResponse struct {
	LoggedOut bool `json:"logged_out"`
}

type checkNowRequest struct {
	HostName  string `json:"host_name"`
	CheckName string `json:"check_name"`
}

type checkNowResponse struct {
	Triggered bool `json:"triggered"`
}

func New(cfg model.HTTPConfig, admin model.AdminConfig, state *state.Manager, downtime *downtime.Manager, runner CheckRunner, logger *zap.SugaredLogger, noAuth bool) *Server {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	ttl := time.Duration(admin.SessionTTL) * time.Second
	return &Server{
		addr:       addr,
		logger:     logger,
		state:      state,
		downtime:   downtime,
		admin:      admin,
		sessions:   session.NewManager(ttl),
		sessionTTL: ttl,
		runner:     runner,
		noAuth:     noAuth,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/login", s.handleLogin)
	mux.HandleFunc("/api/v1/user/logout", s.requireAuth(s.handleLogout))
	mux.HandleFunc("/api/v1/app/health", s.handleAppHealth)
	mux.HandleFunc("/api/v1/app/version", s.handleAppVersion)
	mux.HandleFunc("/api/v1/admin/hosts", s.requireAuth(s.handleHosts))
	mux.HandleFunc("/api/v1/admin/checks", s.requireAuth(s.handleChecks))
	mux.HandleFunc("/api/v1/admin/check", s.requireAuth(s.handleCheck))
	mux.HandleFunc("/api/v1/admin/downtime", s.requireAuth(s.handleDowntime))
	mux.HandleFunc("/api/v1/admin/check/now", s.requireAuth(s.handleCheckNow))
	docsRoot := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/docs/", http.StatusTemporaryRedirect)
	})
	swaggerHandler := httpSwagger.Handler(httpSwagger.URL("/api/docs/doc.json"))
	mux.Handle("/api/docs", s.requireAuthRedirect(docsRoot, "/login"))
	mux.Handle("/api/docs/", s.requireAuthRedirect(swaggerHandler, "/login"))
	srv := &http.Server{Addr: s.addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	s.logger.Infof("http server listening on %s", s.addr)
	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// handleLogin godoc
// @Summary Authenticate admin user
// @Description Validates credentials and starts an authenticated session.
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body loginRequest true "Login credentials"
// @Success 200 {object} loginResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Router /api/v1/user/login [post]
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Login == "" || req.Password == "" {
		s.writeError(w, http.StatusBadRequest, "login and password are required")
		return
	}
	if req.Login != s.admin.Username || req.Password != s.admin.Password {
		s.writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.sessions.Create(req.Login)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	s.setSessionCookie(w, token)
	s.writeJSON(w, http.StatusOK, loginResponse{Login: req.Login})
}

// handleLogout godoc
// @Summary Terminate current admin session
// @Tags auth
// @Produce json
// @Security SessionAuth
// @Success 200 {object} logoutResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/user/logout [post]
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		s.sessions.Delete(cookie.Value)
	}
	s.clearSessionCookie(w)
	s.writeJSON(w, http.StatusOK, logoutResponse{LoggedOut: true})
}

// handleCheckNow godoc
// @Summary Trigger a check immediately
// @Tags checks
// @Accept json
// @Produce json
// @Security SessionAuth
// @Param payload body checkNowRequest true "Target host and check"
// @Success 202 {object} checkNowResponse
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/v1/admin/check/now [post]
func (s *Server) handleCheckNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.runner == nil {
		s.writeError(w, http.StatusServiceUnavailable, "scheduler not available")
		return
	}
	var req checkNowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.HostName == "" || req.CheckName == "" {
		s.writeError(w, http.StatusBadRequest, "host_name and check_name are required")
		return
	}
	if err := s.runner.TriggerCheck(req.HostName, req.CheckName); err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusAccepted, checkNowResponse{Triggered: true})
}

// handleAppHealth godoc
// @Summary Report application health
// @Tags app
// @Produce json
// @Success 200 {object} appHealthResponse
// @Failure 405 {object} errorResponse
// @Router /api/v1/app/health [get]
func (s *Server) handleAppHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s.writeJSON(w, http.StatusOK, appHealthResponse{Status: "ok"})
}

// handleAppVersion godoc
// @Summary Return build metadata
// @Tags app
// @Produce json
// @Success 200 {object} version.Info
// @Failure 405 {object} errorResponse
// @Router /api/v1/app/version [get]
func (s *Server) handleAppVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s.writeJSON(w, http.StatusOK, version.InfoData())
}

// handleHosts godoc
// @Summary List monitored hosts
// @Tags hosts
// @Produce json
// @Security SessionAuth
// @Param count query int false "Number of records to return (default 20)"
// @Param offset query int false "Number of records to skip (>=0)"
// @Success 200 {object} responseHosts
// @Failure 403 {object} errorResponse
// @Router /api/v1/admin/hosts [get]
func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	query := r.URL.Query()
	count := parseInt(query.Get("count"), defaultPageSize)
	if count <= 0 {
		count = defaultPageSize
	}
	offset := parseInt(query.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	all := s.state.HostSummaries()
	total := len(all)
	if offset > total {
		offset = total
	}
	end := offset + count
	if end > total {
		end = total
	}
	items := all[offset:end]
	data := responseHosts{Hosts: items, Count: len(items), Total: total}
	s.writeJSON(w, http.StatusOK, data)
}

// handleChecks godoc
// @Summary List checks with optional filters
// @Tags checks
// @Produce json
// @Security SessionAuth
// @Param host_name query string false "Filter by host"
// @Param check_name query string false "Filter by check name"
// @Param count query int false "Number of records to return (default 20)"
// @Param offset query int false "Number of records to skip (>=0)"
// @Success 200 {object} responseChecks
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse "Host not found"
// @Router /api/v1/admin/checks [get]
func (s *Server) handleChecks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	query := r.URL.Query()
	hostname := query.Get("host_name")
	checkName := query.Get("check_name")
	count := parseInt(query.Get("count"), defaultPageSize)
	if count <= 0 {
		count = defaultPageSize
	}
	offset := parseInt(query.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	checks, ok := s.state.Checks(hostname)
	if !ok && hostname != "" {
		s.writeError(w, http.StatusNotFound, "host not found")
		return
	}
	if checkName != "" {
		filtered := make([]state.CheckInfo, 0)
		for _, chk := range checks {
			if chk.CheckName == checkName {
				filtered = append(filtered, chk)
			}
		}
		checks = filtered
	}
	total := len(checks)
	if offset > total {
		offset = total
	}
	end := offset + count
	if end > total {
		end = total
	}
	items := checks[offset:end]
	s.writeJSON(w, http.StatusOK, responseChecks{Items: items, Count: len(items), Total: total})
}

// handleCheck godoc
// @Summary Get details for a single check
// @Tags checks
// @Produce json
// @Security SessionAuth
// @Param host_name query string true "Host name"
// @Param check_name query string true "Check name"
// @Success 200 {object} responseCheck
// @Failure 400 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/admin/check [get]
func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	query := r.URL.Query()
	hostname := query.Get("host_name")
	checkname := query.Get("check_name")
	if hostname == "" || checkname == "" {
		s.writeError(w, http.StatusBadRequest, "host_name and check_name are required")
		return
	}
	check, ok := s.state.Check(hostname, checkname)
	if !ok {
		s.writeError(w, http.StatusNotFound, "check not found")
		return
	}
	s.writeJSON(w, http.StatusOK, responseCheck{Check: check})
}

type downtimeRequest struct {
	HostName  string `json:"host_name"`
	CheckName string `json:"check_name"`
	Name      string `json:"name"`
	From      *int64 `json:"from"`
	To        *int64 `json:"to"`
	Duration  *int64 `json:"duration"`
}

func (r downtimeRequest) timeRange(now time.Time) (time.Time, time.Time, error) {
	if r.Duration != nil && *r.Duration > 0 {
		start := now
		return start, start.Add(time.Duration(*r.Duration) * time.Second), nil
	}
	if r.From != nil && r.To != nil {
		start := time.Unix(*r.From, 0)
		end := time.Unix(*r.To, 0)
		if !end.After(start) {
			return time.Time{}, time.Time{}, fmt.Errorf("to must be after from")
		}
		return start, end, nil
	}
	return time.Time{}, time.Time{}, errors.New("either duration or from/to must be provided")
}

type downtimeResponse struct {
	Name      string `json:"name"`
	HostName  string `json:"host_name"`
	CheckName string `json:"check_name"`
	From      int64  `json:"from"`
	To        int64  `json:"to"`
}

type downtimeDeleteResponse struct {
	Removed string `json:"removed"`
}

type downtimeListResponse struct {
	Items []downtimeResponse `json:"items"`
	Count int                `json:"count"`
	Total int                `json:"total"`
}

func (s *Server) handleDowntime(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleDowntimeCreate(w, r)
	case http.MethodDelete:
		s.handleDowntimeDelete(w, r)
	case http.MethodGet:
		s.handleDowntimeList(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleDowntimeDelete godoc
// @Summary Remove a downtime entry
// @Tags downtimes
// @Accept json
// @Produce json
// @Security SessionAuth
// @Param payload body downtimeRequest true "Downtime filter"
// @Success 200 {object} downtimeDeleteResponse
// @Failure 400 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/admin/downtime [delete]
func (s *Server) handleDowntimeDelete(w http.ResponseWriter, r *http.Request) {
	var req downtimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.HostName == "" && req.CheckName != "" {
		s.writeError(w, http.StatusBadRequest, "host_name is required when check_name is provided")
		return
	}
	removed := s.downtime.Remove(req.HostName, req.CheckName, req.Name)
	if !removed {
		s.writeError(w, http.StatusNotFound, "downtime not found")
		return
	}
	s.writeJSON(w, http.StatusOK, downtimeDeleteResponse{Removed: req.Name})
}

// handleDowntimeCreate godoc
// @Summary Schedule downtime for checks
// @Tags downtimes
// @Accept json
// @Produce json
// @Security SessionAuth
// @Param payload body downtimeRequest true "Downtime definition"
// @Success 201 {object} downtimeResponse
// @Failure 400 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/admin/downtime [post]
func (s *Server) handleDowntimeCreate(w http.ResponseWriter, r *http.Request) {
	var req downtimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	fromTime, toTime, err := req.timeRange(time.Now())
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entry, err := s.downtime.Add(req.HostName, req.CheckName, req.Name, fromTime, toTime)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeJSON(w, http.StatusCreated, toDowntimeResponse(entry))
}

// handleDowntimeList godoc
// @Summary List active downtime entries
// @Tags downtimes
// @Produce json
// @Security SessionAuth
// @Param host_name query string false "Filter by host"
// @Param check_name query string false "Filter by check"
// @Param name query string false "Filter by downtime name"
// @Param count query int false "Number of records to return (default 20)"
// @Param offset query int false "Number of records to skip (>=0)"
// @Success 200 {object} downtimeListResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/admin/downtime [get]
func (s *Server) handleDowntimeList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	host := query.Get("host_name")
	check := query.Get("check_name")
	name := query.Get("name")
	count := parseInt(query.Get("count"), defaultPageSize)
	if count <= 0 {
		count = defaultPageSize
	}
	offset := parseInt(query.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	entries := s.downtime.List(host, check, name, time.Now())
	total := len(entries)
	if offset > total {
		offset = total
	}
	end := offset + count
	if end > total {
		end = total
	}
	items := make([]downtimeResponse, 0, end-offset)
	for _, entry := range entries[offset:end] {
		items = append(items, toDowntimeResponse(entry))
	}
	s.writeJSON(w, http.StatusOK, downtimeListResponse{Items: items, Count: len(items), Total: total})
}

func toDowntimeResponse(entry downtime.Entry) downtimeResponse {
	return downtimeResponse{Name: entry.Name, HostName: entry.HostName, CheckName: entry.CheckName, From: entry.From.Unix(), To: entry.To.Unix()}
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, errorResponse{Error: message})
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.noAuth {
			next(w, r)
			return
		}
		token, ok := s.authenticateRequest(w, r)
		if !ok {
			s.writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		s.setSessionCookie(w, token)
		next(w, r)
	}
}

func (s *Server) requireAuthHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.noAuth {
			next.ServeHTTP(w, r)
			return
		}
		token, ok := s.authenticateRequest(w, r)
		if !ok {
			s.writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		s.setSessionCookie(w, token)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAuthRedirect(next http.Handler, redirectPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.noAuth {
			next.ServeHTTP(w, r)
			return
		}
		token, ok := s.authenticateRequest(w, r)
		if !ok {
			http.Redirect(w, r, redirectPath, http.StatusFound)
			return
		}
		s.setSessionCookie(w, token)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authenticateRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		s.clearSessionCookie(w)
		return "", false
	}
	if !s.sessions.Validate(cookie.Value) {
		s.clearSessionCookie(w)
		return "", false
	}
	return cookie.Value, true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	expire := time.Now().Add(s.sessionTTL)
	maxAge := int(s.sessionTTL / time.Second)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  expire,
		MaxAge:   maxAge,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func parseInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
