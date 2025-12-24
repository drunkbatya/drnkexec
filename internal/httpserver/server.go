package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/state"
	"go.uber.org/zap"
)

type Server struct {
	addr   string
	logger *zap.SugaredLogger
	state  *state.Manager
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

type responseHost struct {
	Host   state.HostSummary `json:"host"`
	Checks []state.CheckInfo `json:"checks"`
}

type responseCheck struct {
	Check state.CheckInfo `json:"check"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(cfg model.HTTPConfig, state *state.Manager, logger *zap.SugaredLogger) *Server {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	return &Server{addr: addr, logger: logger, state: state}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/hosts", s.handleHosts)
	mux.HandleFunc("/api/v1/host", s.handleHost)
	mux.HandleFunc("/api/v1/checks", s.handleChecks)
	mux.HandleFunc("/api/v1/check", s.handleCheck)
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

func (s *Server) handleHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	hostname := r.URL.Query().Get("host_name")
	if hostname == "" {
		s.writeError(w, http.StatusBadRequest, "host_name is required")
		return
	}
	summary, ok := s.state.HostSummary(hostname)
	if !ok {
		s.writeError(w, http.StatusNotFound, "host not found")
		return
	}
	checks, _ := s.state.HostChecks(hostname)
	s.writeJSON(w, http.StatusOK, responseHost{Host: summary, Checks: checks})
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
