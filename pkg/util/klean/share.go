package klean

import "fmt"

// ShareBind is a Sharer's bind: the host path bound into the run at the container
// path. The zero value binds nothing.
type ShareBind struct {
	HostPath      string // host path bound into the run; "" binds nothing
	ContainerPath string // the bind's target in the container
}

// Sharer is an interface for handling temp filesystems for the container
type Sharer interface {
	Begin() (ShareBind, func() error, error)
}

// Share merges Sharers into one call
type Share []Sharer

// Begin begins each Sharer in order
func (s Share) Begin(binds ...*ShareBind) (func() error, error) {
	if len(binds) != len(s) {
		return nil, fmt.Errorf("klean: bind pointer count (%d) does not match Sharer count (%d)", len(binds), len(s))
	}
	var joiner Joiner
	for i, sharer := range s {
		bind, clean, err := sharer.Begin()
		joiner.Push("clean share", clean)
		if err != nil {
			return joiner.Run, err
		}
		*binds[i] = bind
	}
	return joiner.Run, nil
}
