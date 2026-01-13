package smoketest

import (
	"os"
	"path/filepath"
	goruntime "runtime"
)

func findDevctlRootFromCaller() string {
	_, thisFile, _, ok := goruntime.Caller(0)
	if !ok {
		wd, _ := os.Getwd()
		return wd
	}
	// this file: devctl/cmd/devctl/cmds/dev/smoketest/helpers.go
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", ".."))
}
