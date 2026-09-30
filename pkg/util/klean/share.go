package klean

import "fmt"

// Sharer is an interface for handling temp filesystems for the container
type Sharer interface {
	Begin() (string, func() error, error)
}

// Share merges Sharers into one call
type Share []Sharer

// Begin begins each Sharer in order
func (s Share) Begin(dirs ...*string) (func() error, error) {
	if len(dirs) != len(s) {
		return nil, fmt.Errorf("klean: dir pointer count (%d) does not match Sharer count (%d)", len(dirs), len(s))
	}
	var joiner Joiner
	for i, sharer := range s {
		dir, clean, err := sharer.Begin()
		joiner.Push("settle share", clean)
		if err != nil {
			return joiner.Run, err
		}
		*dirs[i] = dir
	}
	return joiner.Run, nil
}
