package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/drunkbatya/drnkexec/internal/downtime"
	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/state"
	"go.uber.org/zap"
)

type Server struct {
	addr     string
	logger   *zap.SugaredLogger
	state    *state.Manager
	downtime *downtime.Manager
}

type responseHosts struct {
	Hosts []state.HostSummary `json:"hosts"`
}

type responseChecks struct {
	Items    []state.CheckInfo `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Total    int               `json:"total"`
}

type responseCheck struct {
	Check state.CheckInfo `json:"check"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(cfg model.HTTPConfig, state *state.Manager, downtime *downtime.Manager, logger *zap.SugaredLogger) *Server {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	return &Server{addr: addr, logger: logger, state: state, downtime: downtime}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/hosts", s.handleHosts)
	mux.HandleFunc("/api/v1/checks", s.handleChecks)
	mux.HandleFunc("/api/v1/check", s.handleCheck)
	mux.HandleFunc("/api/v1/check/downtime", s.handleDowntime)
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

func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	data := responseHosts{Hosts: s.state.HostSummaries()}
	s.writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleChecks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	query := r.URL.Query()
	hostname := query.Get("host_name")
	checkName := query.Get("check_name")
	page := parseInt(query.Get("page"), 1)
	pageSize := parseInt(query.Get("page_size"), 20)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
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
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := checks[start:end]
	s.writeJSON(w, http.StatusOK, responseChecks{Items: items, Page: page, PageSize: pageSize, Total: total})
}

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

type downtimeResponse struct {
	Name      string `json:"name"`
	HostName  string `json:"host_name"`
	CheckName string `json:"check_name"`
	From      int64  `json:"from"`
	To        int64  `json:"to"`
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
	s.writeJSON(w, http.StatusOK, struct {
		Removed string `json:"removed"`
	}{Removed: req.Name})
}

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
	var fromTime, toTime time.Time
	if req.Duration != nil && *req.Duration > 0 {
		fromTime = time.Now()
		toTime = fromTime.Add(time.Duration(*req.Duration) * time.Second)
	} else if req.From != nil && req.To != nil {
		fromTime = time.Unix(*req.From, 0)
		toTime = time.Unix(*req.To, 0)
	} else {
		s.writeError(w, http.StatusBadRequest, "either duration or from/to must be provided")
		return
	}
	entry, err := s.downtime.Add(req.HostName, req.CheckName, req.Name, fromTime, toTime)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeJSON(w, http.StatusCreated, toDowntimeResponse(entry))
}

func (s *Server) handleDowntimeList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	host := query.Get("host_name")
	check := query.Get("check_name")
	name := query.Get("name")
	page := parseInt(query.Get("page"), 1)
	pageSize := parseInt(query.Get("page_size"), 20)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	entries := s.downtime.List(host, check, name, time.Now())
	total := len(entries)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := make([]downtimeResponse, 0, end-start)
	for _, entry := range entries[start:end] {
		items = append(items, toDowntimeResponse(entry))
	}
	s.writeJSON(w, http.StatusOK, struct {
		Items    []downtimeResponse `json:"items"`
		Page     int                `json:"page"`
		PageSize int                `json:"page_size"`
		Total    int                `json:"total"`
	}{Items: items, Page: page, PageSize: pageSize, Total: total})
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
