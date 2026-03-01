package state

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/drunkbatya/drnkexec/internal/model"
	"go.uber.org/zap"
)

type CheckInfo struct {
	Hostname      string
	CheckName     string
	Status        model.Status
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

type CheckSummary struct {
	CheckName string
	HostCount int
	OK        int
	Warning   int
	Critical  int
	Unknown   int
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
			Status:        model.StatusUnknown,
			FailThreshold: assignment.Check.MinFailBeforeAlert,
		}
	}
	sort.Strings(hostOrder)
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &Manager{hostChecks: hostChecks, hostOrder: hostOrder, logger: logger}
}

func (m *Manager) Update(assignment model.CheckAssignment, status model.Status, output string, failCount int) {
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
	info.Status = status
	info.Output = strings.TrimSpace(output)
	info.UpdatedAt = time.Now()
	info.FailCount = failCount
	info.FailThreshold = assignment.Check.MinFailBeforeAlert
}

func (m *Manager) HostSummaries() []HostSummary {
	return m.HostSummariesFiltered("", nil)
}

func (m *Manager) HostSummariesFiltered(pattern string, statuses []model.Status) []HostSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]HostSummary, 0, len(m.hostOrder))
	var matcher func(string) bool
	if pattern != "" {
		re, err := regexp.Compile(pattern)
		if err == nil {
			matcher = re.MatchString
		} else {
			matcher = func(s string) bool { return strings.HasPrefix(s, pattern) }
		}
	}
	statusFilter := buildStatusFilter(statuses)
	for _, hostname := range m.hostOrder {
		if matcher != nil && !matcher(hostname) {
			continue
		}
		checks := m.hostChecks[hostname]
		if len(statusFilter) > 0 && !hostMatchesStatuses(checks, statusFilter) {
			continue
		}
		result = append(result, m.buildHostSummaryFromChecks(hostname, checks))
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

func (m *Manager) Checks(hostFilter string, statuses []model.Status) ([]CheckInfo, bool) {
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
	statusFilter := buildStatusFilter(statuses)
	var list []CheckInfo
	for _, hostname := range hosts {
		checks := m.hostChecks[hostname]
		for _, info := range checks {
			if len(statusFilter) > 0 {
				if _, ok := statusFilter[info.Status]; !ok {
					continue
				}
			}
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

func (m *Manager) CheckSummaries() []CheckSummary {
	return m.CheckSummariesFiltered("", nil)
}

func (m *Manager) CheckSummariesFiltered(pattern string, statuses []model.Status) []CheckSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var matcher func(string) bool
	if pattern != "" {
		re, err := regexp.Compile(pattern)
		if err == nil {
			matcher = re.MatchString
		} else {
			matcher = func(s string) bool { return strings.HasPrefix(s, pattern) }
		}
	}
	statusFilter := buildStatusFilter(statuses)
	summaryMap := make(map[string]*CheckSummary)
	matchStatus := make(map[string]bool)
	names := make([]string, 0)
	for _, hostname := range m.hostOrder {
		checks := m.hostChecks[hostname]
		for checkName, info := range checks {
			if matcher != nil && !matcher(checkName) {
				continue
			}
			summary, ok := summaryMap[checkName]
			if !ok {
				summary = &CheckSummary{CheckName: checkName}
				summaryMap[checkName] = summary
				names = append(names, checkName)
			}
			summary.HostCount++
			switch info.Status {
			case model.StatusOK:
				summary.OK++
			case model.StatusWarning:
				summary.Warning++
			case model.StatusCritical:
				summary.Critical++
			default:
				summary.Unknown++
			}
			if len(statusFilter) == 0 {
				matchStatus[checkName] = true
			} else if _, ok := statusFilter[info.Status]; ok {
				matchStatus[checkName] = true
			}
		}
	}
	sort.Strings(names)
	result := make([]CheckSummary, 0, len(names))
	for _, name := range names {
		if len(statusFilter) > 0 && !matchStatus[name] {
			continue
		}
		result = append(result, *summaryMap[name])
	}
	return result
}

func (m *Manager) buildHostSummaryFromChecks(hostname string, checks map[string]*CheckInfo) HostSummary {
	summary := HostSummary{Hostname: hostname}
	summary.CheckCount = len(checks)
	for _, info := range checks {
		switch info.Status {
		case model.StatusOK:
			summary.OK++
		case model.StatusWarning:
			summary.Warning++
		case model.StatusCritical:
			summary.Critical++
		default:
			summary.Unknown++
		}
	}
	return summary
}

func buildStatusFilter(statuses []model.Status) map[model.Status]struct{} {
	if len(statuses) == 0 {
		return nil
	}
	filter := make(map[model.Status]struct{}, len(statuses))
	for _, status := range statuses {
		filter[status] = struct{}{}
	}
	return filter
}

func hostMatchesStatuses(checks map[string]*CheckInfo, filter map[model.Status]struct{}) bool {
	if len(filter) == 0 {
		return true
	}
	for _, info := range checks {
		if _, ok := filter[info.Status]; ok {
			return true
		}
	}
	return false
}
