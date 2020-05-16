package littlealbert_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"littlealbert"
	"testing"
)

func Test_Task_Simple(t *testing.T) {
	simple := func(ctx context.Context) littlealbert.Result {
		return littlealbert.Success
	}

	task := littlealbert.Task(simple)

	require.Equal(t, littlealbert.Success, task.Tick(context.Background()))
}

type MaxTick struct {
	counter int
	max     int
}

func (t *MaxTick) Tick(_ context.Context) littlealbert.Result {
	if t.counter >= t.max {
		return littlealbert.Success
	}

	t.counter++

	return littlealbert.Running
}

func Test_Task_Complex(t *testing.T) {
	maxVal := 10

	ticker := &MaxTick{
		max: maxVal,
	}

	for {

		ctx := context.Background()

		if ticker.Tick(ctx) == littlealbert.Success {
			break
		}
	}

	require.Equal(t, maxVal, ticker.counter)
}

func Test_Conditional(t *testing.T) {
	cond := littlealbert.Conditional(func(_ context.Context) bool {
		return false
	})

	require.Equal(t, littlealbert.Failure, cond.Tick(context.Background()))

	cond = littlealbert.Conditional(func(_ context.Context) bool {
		return true
	})

	require.Equal(t, littlealbert.Success, cond.Tick(context.Background()))
}

func Test_Empty_Sequence(t *testing.T) {
	seq := littlealbert.Sequence()

	require.Equal(t, littlealbert.Success, seq.Tick(context.Background()))
}

func Test_All_Succeed_Sequence(t *testing.T) {
	success := func(_ context.Context) littlealbert.Result {
		return littlealbert.Success
	}

	seq := littlealbert.Sequence(
		littlealbert.Task(success),
		littlealbert.Task(success),
		littlealbert.Task(success),
		littlealbert.Task(success),
		littlealbert.Task(success),
		littlealbert.Task(success),
	)

	require.Equal(t, littlealbert.Success, seq.Tick(context.Background()))
}

func Test_One_Failure_Sequence(t *testing.T) {
	success := func(_ context.Context) littlealbert.Result {
		return littlealbert.Success
	}

	seq := littlealbert.Sequence(
		littlealbert.Task(success),
		littlealbert.Task(success),
		littlealbert.Task(success),
		littlealbert.Task(func(_ context.Context) littlealbert.Result {
			return littlealbert.Failure
		}),
		littlealbert.Task(success),
		littlealbert.Task(success),
	)

	require.Equal(t, littlealbert.Failure, seq.Tick(context.Background()))
}

func Test_Running_Sequence(t *testing.T) {
	var touch int
	var once bool

	success := func(_ context.Context) littlealbert.Result {
		touch++

		return littlealbert.Success
	}

	seq := littlealbert.Sequence(
		littlealbert.Task(success),
		littlealbert.Task(success),
		littlealbert.Task(success),
		littlealbert.Task(func(_ context.Context) littlealbert.Result {
			if once {
				return littlealbert.Success
			}

			once = true

			return littlealbert.Running
		}),
		littlealbert.Task(success),
		littlealbert.Task(success),
	)

	require.Equal(t, littlealbert.Running, seq.Tick(context.Background()))
	assert.Equal(t, 3, touch)

	touch = 0
	require.Equal(t, littlealbert.Success, seq.Tick(context.Background()))
	assert.Equal(t, 5, touch)

}
