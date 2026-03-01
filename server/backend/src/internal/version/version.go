package version

import "encoding/json"

const appName = "drnkexec-server"

type Info struct {
	Name         string `json:"name"`
	GitCommit    string `json:"git_commit"`
	GitBranch    string `json:"git_branch"`
	GitBranchNum string `json:"git_branch_num"`
	BuildDate    string `json:"build_date"`
	BuildTime    string `json:"build_time"`
	Version      string `json:"version"`
}

var (
	gitCommit    string
	gitBranch    string
	gitBranchNum string
	buildDate    string
	buildTime    string
	buildVersion string
)

var info = Info{
	Name:         appName,
	GitCommit:    gitCommit,
	GitBranch:    gitBranch,
	GitBranchNum: gitBranchNum,
	BuildDate:    buildDate,
	BuildTime:    buildTime,
	Version:      buildVersion,
}

func Printable() (string, error) {
	jsonData, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

func InfoData() Info {
	return info
}
