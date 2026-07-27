//go:build !linux && !darwin

package runstate

func ReadProcessIdentity(_ int) (*ProcessIdentity, error) {
	return nil, ErrProcessIdentityUnsupported
}

func MatchesProcess(_ *ProcessIdentity) (bool, error) {
	return false, ErrProcessIdentityUnsupported
}

func InspectProcess(_ *ProcessIdentity) (ProcessStatus, error) {
	return ProcessAbsent, ErrProcessIdentityUnsupported
}

func ReadProcessGroupID(_ int) (int, error) {
	return 0, ErrProcessGroupUnsupported
}
