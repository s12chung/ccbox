package cleanup

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
		return nil, fmt.Errorf("cleanup: dir pointer count (%d) does not match Sharer count (%d)", len(dirs), len(s))
	}
	var stack Stack
	for i, sharer := range s {
		dir, clean, err := sharer.Begin()
		stack.Push("settle share", clean)
		if err != nil {
			return stack.Run, err
		}
		*dirs[i] = dir
	}
	return stack.Run, nil
}
