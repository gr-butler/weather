package main

import (
	"math"
	"time"

	"github.com/pointer2null/weather/utils"
	logger "github.com/sirupsen/logrus"
)

const (
	mmPerBucket float64 = 0.2794
	hourRateMin int     = 10 // number of minutes to average for hourly rate
)

func (w *weatherstation) readRainData() {
	for x := range time.Tick(time.Minute) {
		min := x.Minute()

		logger.Infof("Rain mm 24h [%.2f] total hr [%.2f], Rain rate [%.2f]", w.getLast24HRain(), mmhr, rate)
	}
}

func (w *weatherstation) getMMLastHour() float64 {
	total := w.count
	for _, x := range w.btips {
		total += x
	}
	return math.Round(float64(total)*mmPerBucket*100) / 100
}

func (w *weatherstation) getLast24HRain() float64 {
	total := 0.0
	for _, x := range w.rainTotals {
		total += x
	}
	return math.Round(float64(total)*100) / 100
}

// work out the rate per hour assuming it continues as it has in the last x minutes
func (w *weatherstation) getHourlyRate(minute int) float64 {
	count := SumLastRange(minute, hourRateMin, float64(w.count), &w.btips)

	hourMultiplier := float64(60 / hourRateMin)

	return (float64(count) * mmPerBucket * hourMultiplier)
}

func (w *weatherstation) setupRainBuffers() {

	rainSecondBuffer := utils.NewBuffer(60)
	rainAvgMinuteBuffer := utils.NewBuffer(60)
	rainSecondBuffer.SetAutoAverage(rainAvgMinuteBuffer)
	rainAvgHourBuffer := utils.NewBuffer(24)
	rainAvgMinuteBuffer.SetAutoAverage(rainAvgHourBuffer)

	rainMinMinuteBuffer := utils.NewBuffer(60)
	rainSecondBuffer.SetAutoMinimum(rainMinMinuteBuffer)
	rainMinHourBuffer := utils.NewBuffer(24)
	rainMinMinuteBuffer.SetAutoMinimum(rainMinHourBuffer)

	rainMaxMinuteBuffer := utils.NewBuffer(60)
	rainSecondBuffer.SetAutoMaximum(rainMaxMinuteBuffer)
	rainMaxHourBuffer := utils.NewBuffer(24)
	rainMaxMinuteBuffer.SetAutoMaximum(rainMaxHourBuffer)

	w.data.AddBuffer("rain", rainSecondBuffer)
}
