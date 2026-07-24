//go:build !linux

package runstate

func ReadProcessIdentity(_ int) (*ProcessIdentity, error) {
	return nil, ErrProcessIdentityUnsupported
}

func MatchesProcess(_ *ProcessIdentity) (bool, error) {
	return false, ErrProcessIdentityUnsupported
}
