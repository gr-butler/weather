package main

import (
	"time"

	"github.com/pointer2null/weather/utils"
	logger "github.com/sirupsen/logrus"
)

const (
	mmPerBucketTip float64 = 0.2794
	hourRateMins   int     = 10 // number of minutes to average for hourly rate
	millisToSec    int64   = 1000
)

func (w *weatherstation) StartRainMonitor() {
	w.setupRainBuffers()
	go w.readRainData()
}

// once per minute the number of bucket tips are read and we store this in the minute buffer
// once per hour on the overflow we calculate the min/max for that hour and save to their buffers
func (w *weatherstation) readRainData() {
	for range time.Tick(time.Minute) {
		count := w.s.GetRainCount()

		// add this to the rain minute buffer
		rbuff := w.data.GetBuffer("rain")
		rbuff.AddItem(float64(count))

		// Does this belong here? Or should this file just be about recording the data?
		mmLastMinute := float64(count) * mmPerBucketTip
		sum, _, _ := rbuff.SumMinMaxLast(hourRateMins)
		tenMinSum_mm := sum * utils.Sum(mmLastMinute)
		hourRate_mm := (float64(tenMinSum_mm) * 60) / float64(hourRateMins)

		Prom_rainRatePerHour.Set(hourRate_mm)

		// day totals - get the hour sum
		_, _, _, s := rbuff.GetAutoSum().GetAverageMinMaxSum()
		day := float64(s) * mmPerBucketTip
		Prom_rainDayTotal.Set(day)

		logger.Infof("Rain [%.2f] -> hourly rate [%.2f], 24 hour total [%.2f]", mmLastMinute, hourRate_mm, s)
	}
}

func (w *weatherstation) setupRainBuffers() {

	rainMinuteBuffer := utils.NewBuffer(60)

	// add on auto hour buffers to track day values
	rainMinimumHourBuffer := utils.NewBuffer(24)
	rainMinuteBuffer.SetAutoMinimum(rainMinimumHourBuffer)
	rainMaximumHourBuffer := utils.NewBuffer(24)
	rainMinuteBuffer.SetAutoMaximum(rainMaximumHourBuffer)
	rainSumHourBuffer := utils.NewBuffer(24)
	rainMinuteBuffer.SetAutoSum(rainSumHourBuffer)

	w.data.AddBuffer("rain", rainMinuteBuffer)
}
