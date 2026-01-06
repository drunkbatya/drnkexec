package model

import "strings"

type Status string

const (
    StatusOK       Status = "ok"
    StatusWarning  Status = "warning"
    StatusCritical Status = "critical"
    StatusUnknown  Status = "unknown"
)

var AllStatuses = []Status{StatusOK, StatusWarning, StatusCritical, StatusUnknown}

func (s Status) String() string {
    return string(s)
}

func ParseStatus(value string) (Status, bool) {
    switch strings.ToLower(strings.TrimSpace(value)) {
    case string(StatusOK):
        return StatusOK, true
    case string(StatusWarning):
        return StatusWarning, true
    case string(StatusCritical):
        return StatusCritical, true
    case string(StatusUnknown):
        return StatusUnknown, true
    default:
        return "", false
    }
}
