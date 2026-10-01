package sharer

import "github.com/s12chung/ccbox/pkg/util/klean"

// ShareBinder is a Share with the container path its run binds, as a klean.Sharer:
// Begin pairs the Share's host bind path with BindPath.
type ShareBinder struct {
	Share

	BindPath string // the container path bound, whatever HostPath binds
}

// Begin begins the Share, returning its run bind
func (s ShareBinder) Begin() (klean.ShareBind, func() error, error) {
	hostPath, clean, err := s.Share.Begin()
	return klean.ShareBind{HostPath: hostPath, ContainerPath: s.BindPath}, clean, err
}
