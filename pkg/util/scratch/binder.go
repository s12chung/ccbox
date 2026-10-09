package scratch

import "github.com/s12chung/ccbox/pkg/util/klean"

// Binder is a Share with the container path its run binds, as a klean.Sharer:
// Begin pairs the Share's host bind path with BindPath.
type Binder struct {
	Share

	BindPath string // the container path bound, whatever HostPath binds
}

// Begin begins the Share, returning its run bind
func (s Binder) Begin() (klean.ShareBind, func() error, error) {
	hostPath, clean, err := s.Share.Begin()
	return klean.ShareBind{HostPath: hostPath, ContainerPath: s.BindPath}, clean, err
}
