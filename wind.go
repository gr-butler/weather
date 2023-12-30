package main

import (
	"time"

	"github.com/pointer2null/weather/buffer"
	"github.com/pointer2null/weather/constants"
	"github.com/sirupsen/logrus"
)

/*
Measuring gusts and wind intensity

Because wind is an element that varies rapidly over very short periods of
time it is sampled at high frequency (every 0.25 sec) to capture the intensity
of gusts, or short-lived peaks in speed, which inflict greatest damage in
storms. The gust speed and direction are defined by the maximum three second
average wind speed occurring in any period.

The gust speed and direction are defined by the maximum three second average wind speed occurring in any period.

A better measure of the overall wind intensity is defined by the average speed
and direction over the ten minute period leading up to the reporting time.
Mean wind over other averaging periods may also be calculated. A gale is
defined as a surface wind of mean speed of 34-40 knots, averaged over a period
of ten minutes. Terms such as 'severe gale', 'storm', etc are also used to
describe winds of 41 knots or greater.

How do we measure the wind.

The anemometer I use generates 1 pulse per revolution and the specifications states
that equates to 1.429 MPH. This will need to be confirmed and calibrated at some time.


https://www.ncbi.nlm.nih.gov/pmc/articles/PMC5948875/

The wind gust speed, Umax, is defined as a short-duration maximum of the horizontal
wind speed during a longer sampling period (T). Mathematically, it is expressed as
the maximum of the moving averages with a moving average window length equal to the
gust duration (tg). Traditionally in meteorological applications, the gusts are
measured and the wind forecasts issued using a gust duration tg =  3 s and a sample
length T =  10 min

*/

const (
	// 1 tick/second = 1.492MPH wind

	WindSpeedBuffer            = "windSpeed"
	WindGustBuffer             = "windGust"
	AverageWindDirectionBuffer = "windDirectionAvg"
)

var (
	rawSpeed     *buffer.SampleBuffer
	rawDirection *buffer.SampleBuffer
)

func (w *weatherstation) StartWindMonitor() {
	w.setupWindSpeedBuffers()
	rawSpeed = buffer.NewBuffer(600) // 10 minute
	rawDirection = buffer.NewBuffer(60)
	// once per second record the wind speed (ticks)
	go w.recordWindSpeedData()
}

func (w *weatherstation) recordWindSpeedData() {
	for s := range time.Tick(time.Second) {
		count := 0 //w.s.GetWindCount()
		rawSpeed.AddItem(float64(count))
		deg := 0.0 // w.s.GetWindDirection()
		rawDirection.AddItem(deg)
		w.calculateValues(s)
	}
}

// calculate average and gust
func (w *weatherstation) calculateValues(t time.Time) {
	// sample the last 3 seconds and calculate the Speed and Gust values
	var numSeconds = 3
	sumraw, _, _ := rawSpeed.SumMinMaxLast(numSeconds)
	speed := (constants.MphPerTick * float64(sumraw)) / float64(numSeconds)
	wsb := w.data.GetBuffer(WindSpeedBuffer)
	wsb.AddItem(speed)
	gust := constants.MphPerTick * calculateGust(rawSpeed)
	wgb := w.data.GetBuffer(WindGustBuffer)
	wgb.AddItem(gust)

	// use bigger sample for promethius - whole 60 second buffers
	average, _, _, _ := rawDirection.GetAverageMinMaxSum()
	w.data.GetBuffer(AverageWindDirectionBuffer).AddItem(float64(average))
	wspeed, _, _, _ := wsb.GetAverageMinMaxSum()
	wgust, _, _, _ := wgb.GetAverageMinMaxSum()

	if t.Second() < 1 {
		logrus.Infof("Wind direction [%3.2f], speed [%3.2f] gust [%3.2f]", average, wspeed, wgust)
	}

	Prom_windspeed.Set(float64(wspeed))
	Prom_windgust.Set(float64(wgust))
	if float64(wspeed) > 1 {
		Prom_windDirection.Set(float64(average))
	}
}

func calculateGust(buf *buffer.SampleBuffer) float64 {
	size := buf.GetSize()
	copy := buf
	movingAvg := make([]float64, size)
	window := 3

	for x := 0; x < size; x++ {
		movingAvg[x] = float64(copy.AverageLastFrom(window, x))
	}
	// return max of the moving average
	max := 0.0
	for _, v := range movingAvg {
		if v > max {
			max = v
		}
	}
	return max
}

func (w *weatherstation) setupWindSpeedBuffers() {

	windSpeedBuffer := buffer.NewBuffer(60)
	// // needs min, max and avg day buffers
	// windMinSpeedDayBuffer := buffer.NewBuffer(24)
	// windMaxnSpeedDayBuffer := buffer.NewBuffer(24)
	// windAvgSpeedDayBuffer := buffer.NewBuffer(24)
	// windSpeedBuffer.SetAutoMinimum(windMinSpeedDayBuffer)
	// windSpeedBuffer.SetAutoMaximum(windMaxnSpeedDayBuffer)
	// windSpeedBuffer.SetAutoAverage(windAvgSpeedDayBuffer)

	windSpeedGustBuffer := buffer.NewBuffer(60)
	// needs min, max and avg day buffers
	// windMinGustDayBuffer := buffer.NewBuffer(24)
	// windMaxGustDayBuffer := buffer.NewBuffer(24)
	// windAvgGustDayBuffer := buffer.NewBuffer(24)
	// windSpeedGustBuffer.SetAutoMinimum(windMinGustDayBuffer)
	// windSpeedGustBuffer.SetAutoMaximum(windMaxGustDayBuffer)
	// windSpeedGustBuffer.SetAutoAverage(windAvgGustDayBuffer)

	// what do we need?
	windAvgDirectionBuffer := buffer.NewBuffer(60)

	w.data.AddBuffer(WindSpeedBuffer, windSpeedBuffer)
	w.data.AddBuffer(WindGustBuffer, windSpeedGustBuffer)
	w.data.AddBuffer(AverageWindDirectionBuffer, windAvgDirectionBuffer)
}
