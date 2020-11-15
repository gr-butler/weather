package main

import (
	"math"
	"time"

	logger "github.com/sirupsen/logrus"
)

const (
	mmPerBucket float64 = 0.2794
)

func (s *weatherstation) monitorRainGPIO() {
	logger.Info("Starting tip bucket")
	for {
		(*s.rainpin).WaitForEdge(-1)
		s.count++
		s.lastTip = time.Now()
	}
}

func (s *weatherstation) readRainData() {
	go s.monitorRainGPIO()
	for x := range time.Tick(time.Minute) {
		min := x.Minute()
		// store the bucket tip count for the last minute
		s.btips[min] = s.count
		// reset the bucket tip counter
		s.count = 0
		mmRainPerHour.Set(s.getMMLastHour())
		mmRainPerMin.Set(s.getAvgMMLast3Min())
	}
}

func (s *weatherstation) getMMLastHour() float64 {
	total := s.count
	for _, x := range s.btips {
		total += x
	}
	return math.Round(float64(total)*mmPerBucket*100) / 100
}

func (s *weatherstation) getAvgMMLast3Min() float64 {
	min := time.Now().Minute()
	count := 0
	if min >= 2 {
		count = s.count + s.btips[min-1] + s.btips[min-2]
	} else if min == 1 {
		count = s.count + s.btips[0] + s.btips[59]
	} else if min == 0 {
		count = s.count + s.btips[59] + s.btips[58]
	}
	return (float64(count) / 3 * mmPerBucket)
}