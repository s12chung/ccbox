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

func TestChain_Run_StopsOnFirstError(t *testing.T) {
	var order []string
	chain := NewChain(
		func() error { order = append(order, "first"); return nil },
		func() error { order = append(order, "second"); return errors.New("boom") },
		func() error { order = append(order, "third"); return nil },
	)

	err := chain.Run()
	assert.Equal(t, []string{"first", "second"}, order)
	assert.EqualError(t, err, "boom")
}

func TestChain_Run_AllClear(t *testing.T) {
	var order []string
	chain := NewChain(
		func() error { order = append(order, "first"); return nil },
		func() error { order = append(order, "second"); return nil },
	)

	require.NoError(t, chain.Run())
	assert.Equal(t, []string{"first", "second"}, order)
}

func TestNewChain(t *testing.T) {
	var order []string
	chain := NewChain(
		func() error { order = append(order, "first"); return nil },
		nil,
		func() error { order = append(order, "second"); return nil },
	)

	require.NoError(t, chain.Run())
	assert.Equal(t, []string{"first", "second"}, order)
}

func TestSwallowErr_SwallowsToken(t *testing.T) {
	var order []string
	token := errors.New("skip")

	run := SwallowErr(NewChain(
		func() error { order = append(order, "first"); return token },
		func() error { order = append(order, "second"); return nil },
	).Run, token)

	require.NoError(t, run())
	assert.Equal(t, []string{"first"}, order)
}

func TestSwallowErr_KeepsOthers(t *testing.T) {
	wantErr := errors.New("boom")
	token := errors.New("skip")

	run := SwallowErr(NewChain(
		func() error { return wantErr },
		func() error { return nil },
	).Run, token)

	assert.ErrorIs(t, run(), wantErr)
}
