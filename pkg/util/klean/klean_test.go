package klean

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoiner_Run_LIFO(t *testing.T) {
	var order []string
	var joiner Joiner

	joiner.Push("first", func() error { order = append(order, "first"); return nil })
	joiner.Push("second", func() error { order = append(order, "second"); return nil })
	require.NoError(t, joiner.Run())

	assert.Equal(t, []string{"second", "first"}, order)
}

func TestJoiner_Run_StepErrorContinues(t *testing.T) {
	var order []string
	var joiner Joiner

	joiner.Push("first", func() error { order = append(order, "first"); return nil })
	joiner.Push("boom", func() error { order = append(order, "boom"); return errors.New("boom") })

	err := joiner.Run()
	assert.Equal(t, []string{"boom", "first"}, order)
	assert.EqualError(t, err, "boom: boom")
}

func TestJoiner_Push_NilIsNoStep(t *testing.T) {
	var order []string
	var joiner Joiner

	joiner.Push("nil", nil)
	joiner.Push("real", func() error { order = append(order, "real"); return nil })
	joiner.Push("nil", nil)
	require.NoError(t, joiner.Run())

	assert.Equal(t, []string{"real"}, order)
}

func TestQueue_Run_StopsOnFirstError(t *testing.T) {
	var order []string
	queue := NewQueue(
		func() error { order = append(order, "first"); return nil },
		func() error { order = append(order, "second"); return errors.New("boom") },
		func() error { order = append(order, "third"); return nil },
	)

	err := queue.Run()
	assert.Equal(t, []string{"first", "second"}, order)
	assert.EqualError(t, err, "boom")
}

func TestQueue_Run_AllClear(t *testing.T) {
	var order []string
	queue := NewQueue(
		func() error { order = append(order, "first"); return nil },
		func() error { order = append(order, "second"); return nil },
	)

	require.NoError(t, queue.Run())
	assert.Equal(t, []string{"first", "second"}, order)
}

func TestNewQueue(t *testing.T) {
	var order []string
	queue := NewQueue(
		func() error { order = append(order, "first"); return nil },
		nil,
		func() error { order = append(order, "second"); return nil },
	)

	require.NoError(t, queue.Run())
	assert.Equal(t, []string{"first", "second"}, order)
}

func TestSwallowErr_SwallowsToken(t *testing.T) {
	var order []string
	token := errors.New("skip")

	run := SwallowErr(NewQueue(
		func() error { order = append(order, "first"); return token },
		func() error { order = append(order, "second"); return nil },
	).Run, token)

	require.NoError(t, run())
	assert.Equal(t, []string{"first"}, order)
}

func TestSwallowErr_KeepsOthers(t *testing.T) {
	wantErr := errors.New("boom")
	token := errors.New("skip")

	run := SwallowErr(NewQueue(
		func() error { return wantErr },
		func() error { return nil },
	).Run, token)

	assert.ErrorIs(t, run(), wantErr)
}
