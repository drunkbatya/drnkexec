package model

import "time"

type HostsResponse struct {
	Hosts []HostInfo `json:"hosts"`
	Count int        `json:"count"`
	Total int        `json:"total"`
}

type HostInfo struct {
	Hostname   string   `json:"hostname"`
	CheckCount int      `json:"check_count"`
	OK         int      `json:"ok"`
	Warning    int      `json:"warning"`
	Critical   int      `json:"critical"`
	Unknown    int      `json:"unknown"`
	Downtimes  []string `json:"downtimes,omitempty"`
}

type CheckSummariesResponse struct {
	Checks []CheckSummaryInfo `json:"checks"`
	Count  int                `json:"count"`
	Total  int                `json:"total"`
}

type CheckSummaryInfo struct {
	CheckName string   `json:"check_name"`
	HostCount int      `json:"host_count"`
	OK        int      `json:"ok"`
	Warning   int      `json:"warning"`
	Critical  int      `json:"critical"`
	Unknown   int      `json:"unknown"`
	Downtimes []string `json:"downtimes,omitempty"`
}

type CheckDetailsResponse struct {
	Items []CheckDetailInfo `json:"items"`
	Count int               `json:"count"`
	Total int               `json:"total"`
}

type CheckDetailInfo struct {
	Hostname      string    `json:"hostname"`
	CheckName     string    `json:"check_name"`
	Status        Status    `json:"status"`
	Output        string    `json:"output"`
	UpdatedAt     time.Time `json:"updated_at"`
	FailCount     int       `json:"fail_count"`
	FailThreshold int       `json:"fail_threshold"`
	Downtimes     []string  `json:"downtimes,omitempty"`
}
