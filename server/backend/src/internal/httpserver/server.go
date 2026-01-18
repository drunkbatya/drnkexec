package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
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
	mux.HandleFunc("/api/v1/admin/checks/detail", s.requireAuth(s.handleCheckDetails))
	mux.HandleFunc("/api/v1/admin/downtime", s.requireAuth(s.handleDowntime))
	mux.HandleFunc("/api/v1/admin/downtime/relative", s.requireAuth(s.handleDowntimeRelative))
	mux.HandleFunc("/api/v1/admin/downtime/absolute", s.requireAuth(s.handleDowntimeAbsolute))
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
// @Param host_name query string false "Comma-separated list of host names (exact match)"
// @Param host_name_regex query string false "Regex or prefix filter by host name"
// @Param statuses query string false "JSON array of statuses to include"
// @Success 200 {object} model.HostsResponse
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
	hostExact := parseCommaSeparatedValues(query.Get("host_name"))
	prefix := query.Get("host_name_regex")
	statusFilters, err := parseStatusesParam(query.Get("statuses"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	all := s.state.HostSummariesFiltered(prefix, statusFilters)
	if len(hostExact) > 0 {
		filtered := make([]state.HostSummary, 0, len(hostExact))
		allowed := make(map[string]struct{}, len(hostExact))
		for _, host := range hostExact {
			allowed[host] = struct{}{}
		}
		for _, item := range all {
			if _, ok := allowed[item.Hostname]; ok {
				filtered = append(filtered, item)
			}
		}
		all = filtered
	}
	total := len(all)
	if offset > total {
		offset = total
	}
	end := offset + count
	if end > total {
		end = total
	}
	now := time.Now()
	downtimeIdx := s.buildDowntimeIndex(now)
	paged := all[offset:end]
	items := make([]model.HostInfo, 0, len(paged))
	for _, summary := range paged {
		names := combineDowntimeNames(downtimeIdx.global, downtimeIdx.host[summary.Hostname])
		items = append(items, toHostInfo(summary, names))
	}
	data := model.HostsResponse{Hosts: items, Count: len(items), Total: total}
	s.writeJSON(w, http.StatusOK, data)
}

// handleChecks godoc
// @Summary List aggregated checks with status statistics
// @Tags checks
// @Produce json
// @Security SessionAuth
// @Param count query int false "Number of records to return (default 20)"
// @Param offset query int false "Number of records to skip (>=0)"
// @Param check_name query string false "Comma-separated list of check names (exact match)"
// @Param check_name_regex query string false "Regex or prefix filter by check name"
// @Param statuses query string false "JSON array of statuses to include"
// @Success 200 {object} model.CheckSummariesResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/admin/checks [get]
func (s *Server) handleChecks(w http.ResponseWriter, r *http.Request) {
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
	checkNameExact := parseCommaSeparatedValues(query.Get("check_name"))
	checkNameFilter := query.Get("check_name_regex")
	statusFilters, err := parseStatusesParam(query.Get("statuses"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	summaries := s.state.CheckSummariesFiltered(checkNameFilter, statusFilters)
	if len(checkNameExact) > 0 {
		filtered := make([]state.CheckSummary, 0, len(checkNameExact))
		allowed := make(map[string]struct{}, len(checkNameExact))
		for _, check := range checkNameExact {
			allowed[check] = struct{}{}
		}
		for _, summary := range summaries {
			if _, ok := allowed[summary.CheckName]; ok {
				filtered = append(filtered, summary)
			}
		}
		summaries = filtered
	}
	total := len(summaries)
	if offset > total {
		offset = total
	}
	end := offset + count
	if end > total {
		end = total
	}
	now := time.Now()
	downtimeIdx := s.buildDowntimeIndex(now)
	checkHostDowntimes := s.buildCheckDowntimesFromHosts(downtimeIdx)
	paged := summaries[offset:end]
	items := make([]model.CheckSummaryInfo, 0, len(paged))
	for _, summary := range paged {
		names := combineDowntimeNames(
			downtimeIdx.global,
			downtimeIdx.checkAggregate[summary.CheckName],
			checkHostDowntimes[summary.CheckName],
		)
		items = append(items, toCheckSummaryInfo(summary, names))
	}
	s.writeJSON(w, http.StatusOK, model.CheckSummariesResponse{Checks: items, Count: len(items), Total: total})
}

// handleCheckDetails godoc
// @Summary List check execution details with optional filters
// @Tags checks
// @Produce json
// @Security SessionAuth
// @Param host_name query string false "Filter by host"
// @Param check_name query string false "Filter by check name"
// @Param statuses query string false "JSON array of statuses to include"
// @Param count query int false "Number of records to return (default 20)"
// @Param offset query int false "Number of records to skip (>=0)"
// @Success 200 {object} model.CheckDetailsResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse "Host not found"
// @Router /api/v1/admin/checks/detail [get]
func (s *Server) handleCheckDetails(w http.ResponseWriter, r *http.Request) {
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
	statusFilters, err := parseStatusesParam(query.Get("statuses"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	checks, ok := s.state.Checks(hostname, statusFilters)
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
	now := time.Now()
	downtimeIdx := s.buildDowntimeIndex(now)
	paged := checks[offset:end]
	items := make([]model.CheckDetailInfo, 0, len(paged))
	for _, chk := range paged {
		names := combineDowntimeNames(
			downtimeIdx.global,
			downtimeIdx.host[chk.Hostname],
			downtimeIdx.checkNames(chk.Hostname, chk.CheckName),
		)
		items = append(items, toCheckDetailInfo(chk, names))
	}
	s.writeJSON(w, http.StatusOK, model.CheckDetailsResponse{Items: items, Count: len(items), Total: total})
}

type downtimeRequest struct {
	HostName  string `json:"host_name"`
	CheckName string `json:"check_name"`
	Name      string `json:"name"`
	From      *int64 `json:"from"`
	Till      *int64 `json:"till"`
	Duration  *int64 `json:"duration"`
}

type downtimeDeleteRequest struct {
	Name string `json:"name"`
}

type downtimeRelativeRequest struct {
	HostName  string `json:"host_name"`
	CheckName string `json:"check_name"`
	Name      string `json:"name"`
	Duration  *int64 `json:"duration"`
}

type downtimeAbsoluteRequest struct {
	HostName  string `json:"host_name"`
	CheckName string `json:"check_name"`
	Name      string `json:"name"`
	From      *int64 `json:"from"`
	Till      *int64 `json:"till"`
}

type downtimeResponse struct {
	Name      string `json:"name"`
	HostName  string `json:"host_name"`
	CheckName string `json:"check_name"`
	From      int64  `json:"from"`
	To        int64  `json:"to"`
}

type downtimeCreateResponse struct {
	Items []downtimeResponse `json:"items"`
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
// @Param payload body downtimeDeleteRequest true "Downtime identifier"
// @Success 200 {object} downtimeDeleteResponse
// @Failure 400 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/admin/downtime [delete]
func (s *Server) handleDowntimeDelete(w http.ResponseWriter, r *http.Request) {
	var req downtimeDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	removed := s.downtime.RemoveByName(req.Name)
	if !removed {
		s.writeError(w, http.StatusNotFound, "downtime not found")
		return
	}
	s.writeJSON(w, http.StatusOK, downtimeDeleteResponse{Removed: req.Name})
}

// handleDowntimeRelative godoc
// @Summary Schedule downtime with relative duration
// @Tags downtimes
// @Accept json
// @Produce json
// @Security SessionAuth
// @Param payload body downtimeRelativeRequest true "Downtime definition (duration required)"
// @Success 201 {object} downtimeCreateResponse
// @Failure 400 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/admin/downtime/relative [post]
func (s *Server) handleDowntimeRelative(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req downtimeRelativeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Duration == nil || *req.Duration <= 0 {
		s.writeError(w, http.StatusBadRequest, "duration must be greater than zero")
		return
	}
	now := time.Now()
	toTime := now.Add(time.Duration(*req.Duration) * time.Second)
	items, err := s.createDowntimeEntries(req.HostName, req.CheckName, req.Name, now, toTime)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeJSON(w, http.StatusCreated, downtimeCreateResponse{Items: items})
}

// handleDowntimeAbsolute godoc
// @Summary Schedule downtime with absolute timestamps
// @Tags downtimes
// @Accept json
// @Produce json
// @Security SessionAuth
// @Param payload body downtimeAbsoluteRequest true "Downtime definition (from/till required)"
// @Success 201 {object} downtimeCreateResponse
// @Failure 400 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/admin/downtime/absolute [post]
func (s *Server) handleDowntimeAbsolute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req downtimeAbsoluteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.From == nil {
		s.writeError(w, http.StatusBadRequest, "from is required")
		return
	}
	if req.Till == nil {
		s.writeError(w, http.StatusBadRequest, "till is required")
		return
	}
	fromTime := time.Unix(*req.From, 0)
	toTime := time.Unix(*req.Till, 0)
	if !toTime.After(fromTime) {
		s.writeError(w, http.StatusBadRequest, "till must be after from")
		return
	}
	items, err := s.createDowntimeEntries(req.HostName, req.CheckName, req.Name, fromTime, toTime)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeJSON(w, http.StatusCreated, downtimeCreateResponse{Items: items})
}

// handleDowntimeList godoc
// @Summary List active downtime entries
// @Tags downtimes
// @Produce json
// @Security SessionAuth
// @Param host_name query string false "Comma-separated list of hosts (exact match)"
// @Param host_name_regex query string false "Regex or prefix filter by host"
// @Param check_name query string false "Comma-separated list of checks (exact match)"
// @Param check_name_regex query string false "Regex or prefix filter by check"
// @Param name query string false "Comma-separated list of downtime names (exact match)"
// @Param name_regex query string false "Regex or prefix filter by downtime name"
// @Param scope query string false "Filter by scope: all, host, check, global"
// @Param count query int false "Number of records to return (default 20)"
// @Param offset query int false "Number of records to skip (>=0)"
// @Success 200 {object} downtimeListResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/admin/downtime [get]
func (s *Server) handleDowntimeList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	hostExact := parseCommaSeparatedValues(query.Get("host_name"))
	checkExact := parseCommaSeparatedValues(query.Get("check_name"))
	nameExact := parseCommaSeparatedValues(query.Get("name"))
	hostRegex := query.Get("host_name_regex")
	checkRegex := query.Get("check_name_regex")
	nameRegex := query.Get("name_regex")
	scope := query.Get("scope")
	count := parseInt(query.Get("count"), defaultPageSize)
	if count <= 0 {
		count = defaultPageSize
	}
	offset := parseInt(query.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	entries := s.downtime.List(hostRegex, checkRegex, nameRegex, time.Now())
	filtered, err := filterDowntimesByScope(entries, scope)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries = filterDowntimesExact(filtered, hostExact, checkExact, nameExact)
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

func (s *Server) createDowntimeEntries(hostName, checkName, name string, fromTime, toTime time.Time) ([]downtimeResponse, error) {
	if hostName == "" && checkName != "" {
		summaries := s.state.HostSummaries()
		if len(summaries) == 0 {
			return nil, errors.New("no hosts available for downtime")
		}
		created := make([]downtime.Entry, 0, len(summaries))
		responses := make([]downtimeResponse, 0, len(summaries))
		for _, summary := range summaries {
			entry, err := s.downtime.Add(summary.Hostname, checkName, name, fromTime, toTime)
			if err != nil {
				for _, added := range created {
					s.downtime.Remove(added.HostName, added.CheckName, added.Name)
				}
				return nil, err
			}
			created = append(created, entry)
			responses = append(responses, toDowntimeResponse(entry))
		}
		return responses, nil
	}
	entry, err := s.downtime.Add(hostName, checkName, name, fromTime, toTime)
	if err != nil {
		return nil, err
	}
	return []downtimeResponse{toDowntimeResponse(entry)}, nil
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

func parseStatusesParam(raw string) ([]model.Status, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("invalid statuses filter")
	}
	return normalizeStatuses(values)
}

func normalizeStatuses(values []string) ([]model.Status, error) {
	if len(values) == 0 {
		return nil, nil
	}
	seen := make(map[model.Status]struct{}, len(values))
	statuses := make([]model.Status, 0, len(values))
	for _, value := range values {
		status, ok := model.ParseStatus(value)
		if !ok {
			return nil, fmt.Errorf("unknown status %q", value)
		}
		if _, exists := seen[status]; exists {
			continue
		}
		seen[status] = struct{}{}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func toHostInfo(summary state.HostSummary, downtimes []string) model.HostInfo {
	return model.HostInfo{
		Hostname:   summary.Hostname,
		CheckCount: summary.CheckCount,
		OK:         summary.OK,
		Warning:    summary.Warning,
		Critical:   summary.Critical,
		Unknown:    summary.Unknown,
		Downtimes:  downtimes,
	}
}

func toCheckSummaryInfo(summary state.CheckSummary, downtimes []string) model.CheckSummaryInfo {
	return model.CheckSummaryInfo{
		CheckName: summary.CheckName,
		HostCount: summary.HostCount,
		OK:        summary.OK,
		Warning:   summary.Warning,
		Critical:  summary.Critical,
		Unknown:   summary.Unknown,
		Downtimes: downtimes,
	}
}

func toCheckDetailInfo(info state.CheckInfo, downtimes []string) model.CheckDetailInfo {
	return model.CheckDetailInfo{
		Hostname:      info.Hostname,
		CheckName:     info.CheckName,
		Status:        info.Status,
		Output:        info.Output,
		UpdatedAt:     info.UpdatedAt,
		FailCount:     info.FailCount,
		FailThreshold: info.FailThreshold,
		Downtimes:     downtimes,
	}
}

type downtimeIndex struct {
	global         []string
	host           map[string][]string
	check          map[string]map[string][]string
	checkAggregate map[string][]string
}

func (s *Server) buildDowntimeIndex(now time.Time) *downtimeIndex {
	entries := s.downtime.List("", "", "", now)
	idx := &downtimeIndex{
		host:           make(map[string][]string),
		check:          make(map[string]map[string][]string),
		checkAggregate: make(map[string][]string),
	}
	for _, entry := range entries {
		switch {
		case entry.HostName == "" && entry.CheckName == "":
			idx.global = appendUniqueName(idx.global, entry.Name)
		case entry.HostName != "" && entry.CheckName == "":
			idx.host[entry.HostName] = appendUniqueName(idx.host[entry.HostName], entry.Name)
		case entry.HostName != "" && entry.CheckName != "":
			hostChecks := idx.check[entry.HostName]
			if hostChecks == nil {
				hostChecks = make(map[string][]string)
				idx.check[entry.HostName] = hostChecks
			}
			hostChecks[entry.CheckName] = appendUniqueName(hostChecks[entry.CheckName], entry.Name)
			idx.checkAggregate[entry.CheckName] = appendUniqueName(idx.checkAggregate[entry.CheckName], entry.Name)
		default:
			idx.global = appendUniqueName(idx.global, entry.Name)
		}
	}
	return idx
}

func (idx *downtimeIndex) checkNames(hostname, checkName string) []string {
	if idx == nil {
		return nil
	}
	if hostChecks, ok := idx.check[hostname]; ok {
		return hostChecks[checkName]
	}
	return nil
}

func (s *Server) buildCheckDowntimesFromHosts(idx *downtimeIndex) map[string][]string {
	result := make(map[string][]string)
	if idx == nil {
		return result
	}
	for host, names := range idx.host {
		if len(names) == 0 {
			continue
		}
		checks, ok := s.state.HostChecks(host)
		if !ok {
			continue
		}
		for _, chk := range checks {
			result[chk.CheckName] = combineDowntimeNames(result[chk.CheckName], names)
		}
	}
	return result
}

func appendUniqueName(names []string, name string) []string {
	if name == "" {
		return names
	}
	for _, existing := range names {
		if existing == name {
			return names
		}
	}
	return append(names, name)
}

func combineDowntimeNames(lists ...[]string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, list := range lists {
		for _, name := range list {
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func filterDowntimesByScope(entries []downtime.Entry, scope string) ([]downtime.Entry, error) {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" || scope == "all" {
		return entries, nil
	}
	matches := func(entry downtime.Entry) bool {
		switch scope {
		case "host":
			return entry.HostName != "" && entry.CheckName == ""
		case "check":
			return entry.HostName != "" && entry.CheckName != ""
		case "global":
			return entry.HostName == ""
		default:
			return false
		}
	}
	if scope != "host" && scope != "check" && scope != "global" {
		return nil, fmt.Errorf("invalid scope %q", scope)
	}
	filtered := make([]downtime.Entry, 0, len(entries))
	for _, entry := range entries {
		if matches(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

func filterDowntimesExact(entries []downtime.Entry, hostExact, checkExact, nameExact []string) []downtime.Entry {
	result := entries
	if len(hostExact) > 0 {
		tmp := make([]downtime.Entry, 0, len(result))
		allowed := make(map[string]struct{}, len(hostExact))
		for _, host := range hostExact {
			allowed[host] = struct{}{}
		}
		for _, entry := range result {
			if _, ok := allowed[entry.HostName]; ok {
				tmp = append(tmp, entry)
			}
		}
		result = tmp
	}
	if len(checkExact) > 0 {
		tmp := make([]downtime.Entry, 0, len(result))
		allowed := make(map[string]struct{}, len(checkExact))
		for _, check := range checkExact {
			allowed[check] = struct{}{}
		}
		for _, entry := range result {
			if _, ok := allowed[entry.CheckName]; ok {
				tmp = append(tmp, entry)
			}
		}
		result = tmp
	}
	if len(nameExact) > 0 {
		tmp := make([]downtime.Entry, 0, len(result))
		allowed := make(map[string]struct{}, len(nameExact))
		for _, name := range nameExact {
			allowed[name] = struct{}{}
		}
		for _, entry := range result {
			if _, ok := allowed[entry.Name]; ok {
				tmp = append(tmp, entry)
			}
		}
		result = tmp
	}
	return result
}

func parseCommaSeparatedValues(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	values := strings.Split(raw, ",")
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
