package main

import (
	"time"

	"github.com/pointer2null/weather/utils"
	"github.com/sirupsen/logrus"
	//logger "github.com/sirupsen/logrus"
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


*/

const (
	// 1 tick/second = 1.492MPH wind
	mphPerTick float64 = 1.429
)

var (
	rawSpeed     *utils.SampleBuffer
	rawDirection *utils.SampleBuffer
)

func (w *weatherstation) StartWindMonitor() {
	w.setupWindSpeedBuffers()
	rawSpeed = utils.NewBuffer(60)
	// once per second record the wind speed (ticks)
	go w.recordWindSpeedData()
}

func (w *weatherstation) recordWindSpeedData() {
	for range time.Tick(time.Second) {
		count := w.s.GetWindCount()
		rawSpeed.AddItem(float64(count))
		deg := w.s.GetWindDirection()
		rawDirection.AddItem(deg)
		w.calculateValues()
	}
}

// calculate average and gust
func (w *weatherstation) calculateValues() {
	// sample the last 3 seconds and calculate the Speed and Gust values
	var numSeconds = 3
	sumraw, gustraw, _ := rawSpeed.SumMinMaxLast(numSeconds)
	speed := (mphPerTick * float64(sumraw)) / float64(numSeconds)
	w.data.GetBuffer("windSpeed").AddItem(speed)
	gust := (mphPerTick * float64(gustraw)) / float64(numSeconds)
	w.data.GetBuffer("windGust").AddItem(gust)

	// use bigger sample for wind direction - whole 60 second buffer
	average, _, mn, mx := rawDirection.GetAverageMinMaxSum()
	diff := float64(mx) - float64(mn)
	median := float64(mn) + (diff / 2)
	logrus.Infof("Wind 3 second average [%v], median [%v]", average, median)
	w.data.GetBuffer("windDirectionAvg").AddItem(float64(average))
	w.data.GetBuffer("windDirectionMedian").AddItem(median)

	Prom_windspeed.Set(speed)
	Prom_windgust.Set(gust)
	Prom_windDirection.Set(float64(average))
	Prom_windMedian.Set(median)
}

func (w *weatherstation) setupWindSpeedBuffers() {

	windSpeedBuffer := utils.NewBuffer(60)
	// needs min, max and avg day buffers
	windMinSpeedDayBuffer := utils.NewBuffer(24)
	windMaxnSpeedDayBuffer := utils.NewBuffer(24)
	windAvgSpeedDayBuffer := utils.NewBuffer(24)
	windSpeedBuffer.SetAutoMinimum(windMinSpeedDayBuffer)
	windSpeedBuffer.SetAutoMaximum(windMaxnSpeedDayBuffer)
	windSpeedBuffer.SetAutoAverage(windAvgSpeedDayBuffer)

	windSpeedGustBuffer := utils.NewBuffer(60)
	// needs min, max and avg day buffers
	windMinGustDayBuffer := utils.NewBuffer(24)
	windMaxGustDayBuffer := utils.NewBuffer(24)
	windAvgGustDayBuffer := utils.NewBuffer(24)
	windSpeedGustBuffer.SetAutoMinimum(windMinGustDayBuffer)
	windSpeedGustBuffer.SetAutoMaximum(windMaxGustDayBuffer)
	windSpeedGustBuffer.SetAutoAverage(windAvgGustDayBuffer)

	// what do we need?
	windAvgDirectionBuffer := utils.NewBuffer(60)
	windMedianDirectionBuffer := utils.NewBuffer(60)

	w.data.AddBuffer("windSpeed", windSpeedBuffer)
	w.data.AddBuffer("windGust", windSpeedGustBuffer)
	w.data.AddBuffer("windDirectionAvg", windAvgDirectionBuffer)
	w.data.AddBuffer("windDirectionMedian", windMedianDirectionBuffer)
}
