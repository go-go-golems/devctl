package tui

import "time"

type RootOptions struct {
	RepoRoot string
	Config   string
	Profile  string
	Strict   bool
	DryRun   bool
	Timeout  time.Duration
}
