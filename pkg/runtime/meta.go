package runtime

type RequestMeta struct {
	RepoRoot string
	Cwd      string
	DryRun   bool
}
