package sensors

import (
	"testing"

	"github.com/pointer2null/weather/buffer"
	"github.com/pointer2null/weather/env"
	"github.com/stretchr/testify/require"
)

func Test_anemometer_GetSpeed(t *testing.T) {
	a := Anemometer{
		dirADC:   nil,
		Bus:      nil,
		speedBuf: buffer.NewBuffer(env.WindSamplesPerSecond * env.WindBufferLengthSeconds),
		gustBuf:  buffer.NewBuffer(env.WindSamplesPerSecond * env.WindBufferLengthSeconds),
		dirBuf:   &buffer.SampleBuffer{},
		masthead: nil,
		args:     env.Args{},
	}

	s := a.GetSpeed()
	require.Equal(t, float64(0), s)

	// the first time a value is set in a buffer, it is filled with that value so easy to populate
	a.speedBuf.AddItem(float64(1))
	a.gustBuf.AddItem(float64(1))
	// 1 pick per 1/4 second with current values.

	avg, _, _, _ := a.speedBuf.GetAverageMinMaxSum()

	require.Equal(t, buffer.Average(1), avg)

	ticksSecond := avg * env.WindSamplesPerSecond

	require.Equal(t, buffer.Average(4), ticksSecond)

	calc := a.GetSpeed()

	require.Equal(t, float64(ticksSecond*env.MphPerTick), calc)
}
