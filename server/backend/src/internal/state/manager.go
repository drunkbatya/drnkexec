package state

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/nrpeclient"
	"go.uber.org/zap"
)

type Status string

const (
	StatusOK       Status = "ok"
	StatusWarning  Status = "warning"
	StatusCritical Status = "critical"
	StatusUnknown  Status = "unknown"
)

type CheckInfo struct {
	Hostname      string
	CheckName     string
	Status        Status
	Output        string
	UpdatedAt     time.Time
	FailCount     int
	FailThreshold int
}

type HostSummary struct {
	Hostname   string
	CheckCount int
	OK         int
	Warning    int
	Critical   int
	Unknown    int
}

type Manager struct {
	mu         sync.RWMutex
	hostChecks map[string]map[string]*CheckInfo
	hostOrder  []string
	logger     *zap.SugaredLogger
}

func NewManager(logger *zap.SugaredLogger, cfg *model.Config) *Manager {
	hostChecks := make(map[string]map[string]*CheckInfo, len(cfg.Hosts))
	hostOrder := make([]string, 0, len(cfg.Hosts))
	seenHosts := make(map[string]struct{}, len(cfg.Hosts))
	for _, host := range cfg.Hosts {
		hostChecks[host.Hostname] = make(map[string]*CheckInfo)
		if _, ok := seenHosts[host.Hostname]; !ok {
			hostOrder = append(hostOrder, host.Hostname)
			seenHosts[host.Hostname] = struct{}{}
		}
	}
	for _, assignment := range cfg.LookupMaps.CheckAssignments {
		host := assignment.Host.Hostname
		if _, ok := hostChecks[host]; !ok {
			hostChecks[host] = make(map[string]*CheckInfo)
			if _, seen := seenHosts[host]; !seen {
				hostOrder = append(hostOrder, host)
				seenHosts[host] = struct{}{}
			}
		}
		hostChecks[host][assignment.Check.Name] = &CheckInfo{
			Hostname:      host,
			CheckName:     assignment.Check.Name,
			Status:        StatusUnknown,
			FailThreshold: assignment.Check.MinFailBeforeAlert,
		}
	}
	sort.Strings(hostOrder)
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &Manager{hostChecks: hostChecks, hostOrder: hostOrder, logger: logger}
}

func (m *Manager) Update(assignment model.CheckAssignment, status nrpeclient.Status, output string, failCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hostname := assignment.Host.Hostname
	checkname := assignment.Check.Name
	if _, ok := m.hostChecks[hostname]; !ok {
		m.hostChecks[hostname] = make(map[string]*CheckInfo)
		m.hostOrder = append(m.hostOrder, hostname)
		sort.Strings(m.hostOrder)
	}
	info, ok := m.hostChecks[hostname][checkname]
	if !ok {
		info = &CheckInfo{Hostname: hostname, CheckName: checkname}
		m.hostChecks[hostname][checkname] = info
	}
	info.Status = mapStatus(status)
	info.Output = strings.TrimSpace(output)
	info.UpdatedAt = time.Now()
	info.FailCount = failCount
	info.FailThreshold = assignment.Check.MinFailBeforeAlert
}

func (m *Manager) HostSummaries() []HostSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]HostSummary, 0, len(m.hostOrder))
	for _, hostname := range m.hostOrder {
		result = append(result, m.buildHostSummaryLocked(hostname))
	}
	return result
}

func (m *Manager) HostSummary(hostname string) (HostSummary, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	checks, ok := m.hostChecks[hostname]
	if !ok {
		return HostSummary{}, false
	}
	return m.buildHostSummaryFromChecks(hostname, checks), true
}

func (m *Manager) HostChecks(hostname string) ([]CheckInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	checks, ok := m.hostChecks[hostname]
	if !ok {
		return nil, false
	}
	list := make([]CheckInfo, 0, len(checks))
	for _, info := range checks {
		list = append(list, *info)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Hostname == list[j].Hostname {
			return list[i].CheckName < list[j].CheckName
		}
		return list[i].Hostname < list[j].Hostname
	})
	return list, true
}

func (m *Manager) Checks(hostFilter string) ([]CheckInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var hosts []string
	if hostFilter != "" {
		if _, ok := m.hostChecks[hostFilter]; !ok {
			return nil, false
		}
		hosts = []string{hostFilter}
	} else {
		hosts = append(hosts, m.hostOrder...)
	}
	var list []CheckInfo
	for _, hostname := range hosts {
		checks := m.hostChecks[hostname]
		for _, info := range checks {
			list = append(list, *info)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Hostname == list[j].Hostname {
			return list[i].CheckName < list[j].CheckName
		}
		return list[i].Hostname < list[j].Hostname
	})
	return list, true
}

func (m *Manager) Check(hostname, checkname string) (CheckInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	checks, ok := m.hostChecks[hostname]
	if !ok {
		return CheckInfo{}, false
	}
	info, ok := checks[checkname]
	if !ok {
		return CheckInfo{}, false
	}
	return *info, true
}

func (m *Manager) buildHostSummaryLocked(hostname string) HostSummary {
	checks := m.hostChecks[hostname]
	return m.buildHostSummaryFromChecks(hostname, checks)
}

func (m *Manager) buildHostSummaryFromChecks(hostname string, checks map[string]*CheckInfo) HostSummary {
	summary := HostSummary{Hostname: hostname}
	summary.CheckCount = len(checks)
	for _, info := range checks {
		switch info.Status {
		case StatusOK:
			summary.OK++
		case StatusWarning:
			summary.Warning++
		case StatusCritical:
			summary.Critical++
		default:
			summary.Unknown++
		}
	}
	return summary
}

func mapStatus(status nrpeclient.Status) Status {
	switch status {
	case nrpeclient.StatusOK:
		return StatusOK
	case nrpeclient.StatusWarning:
		return StatusWarning
	case nrpeclient.StatusCritical:
		return StatusCritical
	default:
		return StatusUnknown
	}
}
