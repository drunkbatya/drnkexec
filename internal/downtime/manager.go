package downtime

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type Entry struct {
	Name      string
	HostName  string
	CheckName string
	From      time.Time
	To        time.Time
	CreatedAt time.Time
}

type Manager struct {
	mu          sync.RWMutex
	entries     map[string]map[string]Entry
	hostEntries map[string]map[string]Entry
}

func (m *Manager) Remove(host, check, name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case host == "" && check == "":
		return m.removeGlobalLocked(name)
	case host != "" && check == "":
		return m.removeHostLocked(host, name)
	case host != "" && check != "":
		return m.removeCheckLocked(host, check, name)
	default:
		return false
	}
}

func NewManager() *Manager {
	return &Manager{entries: make(map[string]map[string]Entry), hostEntries: make(map[string]map[string]Entry)}
}

func (m *Manager) Add(host, check, name string, from, to time.Time) (Entry, error) {
	if !to.After(from) {
		return Entry{}, fmt.Errorf("to must be after from")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if host == "" {
		return m.addGlobalLocked(name, from, to)
	}
	if check == "" {
		return m.addHostLocked(host, name, from, to)
	}
	return m.addCheckLocked(host, check, name, from, to)
}

func (m *Manager) Active(host, check string, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.globalActiveLocked(now) || m.hostActiveLocked(host, now) || m.checkActiveLocked(host, check, now)
}

func (m *Manager) AddHost(host, name string, from, to time.Time) (Entry, error) {
	if host == "" {
		return m.Add("", "", name, from, to)
	}
	if !to.After(from) {
		return Entry{}, fmt.Errorf("to must be after from")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addHostLocked(host, name, from, to)
}

func (m *Manager) ActiveHost(host string, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hostActiveLocked(host, now)
}

func (m *Manager) List(hostFilter, checkFilter, nameFilter string, now time.Time) []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Entry
	for key, bucket := range m.entries {
		if key == "::" {
			continue
		}
		for name, entry := range bucket {
			if now.After(entry.To) {
				delete(bucket, name)
				continue
			}
			if matchesFilter(entry, hostFilter, checkFilter, nameFilter) {
				result = append(result, entry)
			}
		}
		if len(bucket) == 0 {
			delete(m.entries, key)
		}
	}
	for host, bucket := range m.hostEntries {
		for name, entry := range bucket {
			if now.After(entry.To) {
				delete(bucket, name)
				continue
			}
			if matchesFilter(entry, hostFilter, checkFilter, nameFilter) {
				result = append(result, entry)
			}
		}
		if len(bucket) == 0 {
			delete(m.hostEntries, host)
		}
	}
	for name, entry := range m.entries["::"] {
		if now.After(entry.To) {
			delete(m.entries["::"], name)
			continue
		}
		if matchesFilter(entry, hostFilter, checkFilter, nameFilter) {
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].HostName == result[j].HostName {
			if result[i].CheckName == result[j].CheckName {
				return result[i].From.Before(result[j].From)
			}
			return result[i].CheckName < result[j].CheckName
		}
		return result[i].HostName < result[j].HostName
	})
	return result
}

func matchesFilter(entry Entry, hostFilter, checkFilter, nameFilter string) bool {
	if hostFilter != "" && entry.HostName != hostFilter {
		return false
	}
	if checkFilter != "" && entry.CheckName != checkFilter {
		return false
	}
	if nameFilter != "" && entry.Name != nameFilter {
		return false
	}
	return true
}

func buildKey(host, check string) string {
	return host + "::" + check
}

func (m *Manager) hostActiveLocked(host string, now time.Time) bool {
	bucket, ok := m.hostEntries[host]
	if !ok {
		return false
	}
	active := false
	for name, entry := range bucket {
		if now.After(entry.To) {
			delete(bucket, name)
			continue
		}
		if !now.Before(entry.From) && now.Before(entry.To) {
			active = true
		}
	}
	if len(bucket) == 0 {
		delete(m.hostEntries, host)
	}
	return active
}

func (m *Manager) checkActiveLocked(host, check string, now time.Time) bool {
	key := buildKey(host, check)
	bucket, ok := m.entries[key]
	if !ok {
		return false
	}
	active := false
	for name, entry := range bucket {
		if now.After(entry.To) {
			delete(bucket, name)
			continue
		}
		if !now.Before(entry.From) && now.Before(entry.To) {
			active = true
		}
	}
	if len(bucket) == 0 {
		delete(m.entries, key)
	}
	return active
}

func (m *Manager) globalActiveLocked(now time.Time) bool {
	bucket, ok := m.entries["::"]
	if !ok {
		return false
	}
	active := false
	for name, entry := range bucket {
		if now.After(entry.To) {
			delete(bucket, name)
			continue
		}
		if !now.Before(entry.From) && now.Before(entry.To) {
			active = true
		}
	}
	if len(bucket) == 0 {
		delete(m.entries, "::")
	}
	return active
}

func (m *Manager) addCheckLocked(host, check, name string, from, to time.Time) (Entry, error) {
	key := buildKey(host, check)
	bucket, ok := m.entries[key]
	if !ok {
		bucket = make(map[string]Entry)
		m.entries[key] = bucket
	}
	if _, exists := bucket[name]; exists {
		return Entry{}, fmt.Errorf("downtime %s already exists for host %s check %s", name, host, check)
	}
	entry := Entry{Name: name, HostName: host, CheckName: check, From: from, To: to, CreatedAt: time.Now()}
	bucket[name] = entry
	return entry, nil
}

func (m *Manager) addHostLocked(host, name string, from, to time.Time) (Entry, error) {
	bucket, ok := m.hostEntries[host]
	if !ok {
		bucket = make(map[string]Entry)
		m.hostEntries[host] = bucket
	}
	if _, exists := bucket[name]; exists {
		return Entry{}, fmt.Errorf("downtime %s already exists for host %s", name, host)
	}
	entry := Entry{Name: name, HostName: host, CheckName: "", From: from, To: to, CreatedAt: time.Now()}
	bucket[name] = entry
	return entry, nil
}

func (m *Manager) addGlobalLocked(name string, from, to time.Time) (Entry, error) {
	bucket, ok := m.entries["::"]
	if !ok {
		bucket = make(map[string]Entry)
		m.entries["::"] = bucket
	}
	if _, exists := bucket[name]; exists {
		return Entry{}, fmt.Errorf("global downtime %s already exists", name)
	}
	entry := Entry{Name: name, HostName: "", CheckName: "", From: from, To: to, CreatedAt: time.Now()}
	bucket[name] = entry
	return entry, nil
}

func (m *Manager) removeCheckLocked(host, check, name string) bool {
	key := buildKey(host, check)
	bucket, ok := m.entries[key]
	if !ok {
		return false
	}
	if _, ok := bucket[name]; !ok {
		return false
	}
	delete(bucket, name)
	if len(bucket) == 0 {
		delete(m.entries, key)
	}
	return true
}

func (m *Manager) removeHostLocked(host, name string) bool {
	bucket, ok := m.hostEntries[host]
	if !ok {
		return false
	}
	if _, ok := bucket[name]; !ok {
		return false
	}
	delete(bucket, name)
	if len(bucket) == 0 {
		delete(m.hostEntries, host)
	}
	return true
}

func (m *Manager) removeGlobalLocked(name string) bool {
	bucket, ok := m.entries["::"]
	if !ok {
		return false
	}
	if _, ok := bucket[name]; !ok {
		return false
	}
	delete(bucket, name)
	if len(bucket) == 0 {
		delete(m.entries, "::")
	}
	return true
}
