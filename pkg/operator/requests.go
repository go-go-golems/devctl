package operator

import "time"

type Selection struct {
	Services []string
}

type PipelinePolicy struct {
	ConfigPath   string
	Cwd          string
	Strict       bool
	DryRun       bool
	Timeout      time.Duration
	SkipBuild    bool
	SkipPrepare  bool
	SkipValidate bool
	BuildSteps   []string
	PrepareSteps []string
}

type UpRequest struct {
	RepoRoot string
	Profile  string
	Select   Selection
	Policy   PipelinePolicy
}

type DownRequest struct {
	RepoRoot string
	Select   Selection
	Timeout  time.Duration
}

type RestartRequest struct {
	RepoRoot string
	Profile  string
	Select   Selection
	Policy   PipelinePolicy
}

type SnapshotRequest struct {
	RepoRoot      string
	IncludeRuns   bool
	IncludeHealth bool
}

type DoctorRequest struct {
	RepoRoot   string
	ConfigPath string
	Profile    string
	Cwd        string
	Timeout    time.Duration
	Plugins    bool
}
