//go:build !linux

package runtime

func enableChildSubreaper() error { return nil }
func reapProcessGroup(_ int)      {}
