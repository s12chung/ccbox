package cleanup

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStack_Run_LIFO(t *testing.T) {
	var order []string
	var stack Stack

	stack.Push("first", func() error { order = append(order, "first"); return nil })
	stack.Push("second", func() error { order = append(order, "second"); return nil })
	require.NoError(t, stack.Run())

	assert.Equal(t, []string{"second", "first"}, order)
}

func TestStack_Run_StepErrorContinues(t *testing.T) {
	var order []string
	var stack Stack

	stack.Push("first", func() error { order = append(order, "first"); return nil })
	stack.Push("boom", func() error { order = append(order, "boom"); return errors.New("boom") })

	err := stack.Run()
	assert.Equal(t, []string{"boom", "first"}, order)
	assert.EqualError(t, err, "boom: boom")
}

func TestStack_Push_NilIsNoStep(t *testing.T) {
	var order []string
	var stack Stack

	stack.Push("nil", nil)
	stack.Push("real", func() error { order = append(order, "real"); return nil })
	stack.Push("nil", nil)
	require.NoError(t, stack.Run())

	assert.Equal(t, []string{"real"}, order)
}
