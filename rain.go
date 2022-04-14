package main

import (
	"math"
	"time"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/gpio"
)

const (
	mmPerBucket float64 = 0.2794
	hourRateMin int     = 10 // number of minutes to average for hourly rate
)

func (w *weatherstation) monitorRainGPIO() {
	logger.Info("Starting tip bucket")
	defer func() { _ = (*w.s.GPIO.Rainpin).Halt() }()
	for {
		(*w.s.GPIO.Rainpin).WaitForEdge(-1)
		if (*w.s.GPIO.Rainpin).Read() == gpio.Low {
			w.count++
			w.lastTip = time.Now()
			logger.Infof("Bucket tip. [%v]", w.count)
		}
		time.Sleep(time.Second)
	}
}

func (w *weatherstation) readRainData() {
	go w.monitorRainGPIO()
	lastHour := 0
	for x := range time.Tick(time.Minute) {
		min := x.Minute()
		// store day total (mm)
		if x.Hour() == lastHour {
			w.rainTotals[x.Hour()] += w.count * mmPerBucket
		} else {
			// first time we write for this hour, reset the count
			w.rainTotals[x.Hour()] = w.count * mmPerBucket
			lastHour = x.Hour()
		}

		rainDayTotal.Set(w.getLast24HRain())
		// store the bucket tip count for the last minute
		w.btips[min] = w.count
		// reset the bucket tip counter
		w.count = 0

		mmhr := w.getMMLastHour()
		rate := w.getHourlyRate(min)
		mmRainPerHour.Set(mmhr)
		rainRatePerHour.Set(rate)
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
