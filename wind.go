package main

import (
	"time"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/physic"
)

/*
Measuring gusts and wind intensity

Because wind is an element that varies rapidly over very short periods of
time it is sampled at high frequency (every 0.25 sec) to capture the intensity
of gusts, or short-lived peaks in speed, which inflict greatest damage in
storms. The gust speed and direction are defined by the maximum three second
average wind speed occurring in any period.

A better measure of the overall wind intensity is defined by the average speed
and direction over the ten minute period leading up to the reporting time.
Mean wind over other averaging periods may also be calculated. A gale is
defined as a surface wind of mean speed of 34-40 knots, averaged over a period
of ten minutes. Terms such as 'severe gale', 'storm', etc are also used to
describe winds of 41 knots or greater.

How do we measure the wind.

The anemometer I use generates 1 pulse per revolution and the specifications states
that equates to 1.429 MPH. This will need to be confirmed and calibrated at some time.

When the readWindData function is called from main as a go routine, it starts two other
threads: monitorWindGPIO and processWindSpeed.

monitorWindGPIO sits in a forever loop and waits for the GPIO pin to be triggered. On each
tick it calculates the instantanious wind speed based on the time since the last tick was
recorded. The instant speed is summed and the number of readings is counted. At the same time
the maximum value is calculated. The WaitForEdge library will queue events and fall though
immediately if there has been a pulse since the last one. Since we are suseptable to switch
bounce and noise, we can get spurious pulses. We deal with that by clearing out the queue at
the start of each loop.

The second thread processWindSpeed has a ticker that fires every minutes. On each tick it the
wind average is calculated since the last read. The sum and count variables are reset and the
avg and max values recorded.

Values for windspeed, gust and direction are stored in local variable for the local web server
and in the prometeus guages for further processing.
*/

const (
	// 1 tick/second = 1.492MPH wind
	mphPerTick float64 = 1.429
)

var (
	pcount            = 0
	wsum              = 0.0
	wmax              = 0.0
)

func (s *weatherstation) readWindData() {
	go s.monitorWindGPIO()
	go s.recordWindSpeed()
	for range time.Tick(time.Second * 30) {
		s.readWindDirection()
	}
}

// monitorWindGPIO watches the gpio port on tick calculate the instantanious wind speed.
// WaitForEdge returns immediately IF another pulse has arrived since the last call.
// need to make sure any queue is cleared before we restart the loop
// ASSUMPTION: the loop is significantly faster than the incoming pulse rate
func (w *weatherstation) monitorWindGPIO() {
	logger.Info("Starting wind sensor")
	defer func() { _ = (*w.s.windpin).Halt() }()
	for {
		// need to clear out any pulses that maybe queued
		var crud bool
		for {
			// loop and WaitForEdge with very small timeout to clear out any queue
			crud = (*w.s.windpin).WaitForEdge(time.Microsecond)
			// if we hit timeout crud = false, then we exit
			if !crud {
				break
			}
		}
		// wait for the next (hopefully) genuine pulse
		(*w.s.windpin).WaitForEdge(-1)
		// start timer...
		startTime := time.Now()
		(*w.s.windpin).WaitForEdge(-1)
		period := time.Since(startTime).Seconds()
		if period > 0.01 { // TODO fudge - still getting stupid values (latest 2422MPH)
			freq := float64(1 / period)
			speed := freq * mphPerTick
			pcount++
			wsum += speed
			if speed > wmax {
				wmax = speed
			}
		} else { 
			logger.Warnf("IGNORNING: Period [%.4f] Speed [%v]", period, (mphPerTick / period))
		}
	}
}

func (w *weatherstation) recordWindSpeed() {
	avg := 0.0
	// start ticker
	for range time.Tick(time.Minute) {
		if pcount > 0 {
			avg = wsum / float64(pcount)
		}
		windspeed.Set(avg)
		windgust.Set(wmax)
		logger.Infof("Wind Avg [%.2f] Gust [%.2f]", avg, wmax)
		w.windSpeedAvg = avg
		w.windGust = wmax //TODO gust needs to be a 3s long pulse of wind
		wmax = 0.0
		wsum = 0.0
		pcount = 0
		avg = 0.0
	}
}

func (w *weatherstation) readWindDirection() {
	sample, err := (*w.s.windDir).Read()
	if err != nil {
		logger.Errorf("Error reading wind direction value [%v]", err)
		sample.Raw = 0
	}
	w.windVolts = float64(sample.V) / float64(physic.Volt)
	w.windDirection = voltToDegrees(w.windVolts)
	logger.Debugf("Volt [%v], Dir [%v]", w.windVolts, w.windDirection)

	// prometheus data
	logger.Debugf("Setting winddir [%v]", w.s.windDir)
	windDirection.Set(w.windDirection)
}

func voltToDegrees(v float64) float64 {
	// this is based on the sensor datasheet that gives a list of voltages for each direction when set up according
	// to the circuit given. Have noticed the output isn't that accurate relative to the sensor direction...
	switch {
	case v < 0.365:
		return 112.5
	case v < 0.430:
		return 67.5
	case v < 0.535:
		return 90.0
	case v < 0.760:
		return 157.5
	case v < 1.045:
		return 135.0
	case v < 1.295:
		return 202.5
	case v < 1.690:
		return 180.0
	case v < 2.115:
		return 22.5
	case v < 2.590:
		return 45.0
	case v < 3.005:
		return 247.5
	case v < 3.225:
		return 225.0
	case v < 3.635:
		return 337.5
	case v < 3.940:
		return 0
	case v < 4.185:
		return 292.5
	case v < 4.475:
		return 315.0
	default:
		return 270.0
	}
}
