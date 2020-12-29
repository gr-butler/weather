package main

import (
	"math"
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
recorded.

The second thread processWindSpeed has a ticker that fires every 250ms. On each tick it records
instantanious wind speed. It then clear the value - if it didn't we would not know if the wind
stopped! If we have collected 4 values then we calculate the max and average of those values.
The max gives us the value for the wind gust and the avg for that second is recorded in another
array. When we have accumulated 60 values we work out the average wind speed for the last minute.

Values for windspeed, gust and direction are stored in local variable for the local web server
and in the prometeus guages for further processing.
*/

const (
	// 1 tick/second = 1.492MPH wind
	mphPerTick float64 = 1.429
)

var (
	livespeed float64 = 0
)

func (s *weatherstation) readWindData() {
	go s.monitorWindGPIO()
	go s.processWindSpeed()
	for range time.Tick(time.Second * 10) {
		s.readWindDirection()
	}
}

// monitorWindGPIO watch the gpio port on tick calculate the instantanious wind speed.
// The GPIO pin is NOT debounced in hardware. I started seeing crazy windspeeds occasionally
// e.g. 1423MPH on a clear afternoon! It's difficult to prove what caused that but the best
// guess is EITHER a bounce created two very rapid ticks OR EMF interference generated a series
// of pulses.
// Ideally we need some hardware debounce - until then we can mitigate against this.
// The sensor produces 1 pulse per second for 1.423MPH, 100 pps would be 142.3MPH - not something
// you'd see in the UK and if we did it would probably destroy my sensor array! So if we add a 10ms
// delay to the end of the loop then this would allow any bouncing to stop and we would loop back
// to the blocking WaitForEdge in plenty of time for normal wind speeds.
func (w *weatherstation) monitorWindGPIO() {
	logger.Info("Starting wind sensor")
	defer func() { _ = (*w.s.windpin).Halt() }()
	lasttick := time.Now().Add(time.Duration(-1) * time.Second)
	var edge time.Time
	for {
		(*w.s.windpin).WaitForEdge(-1)
		edge = time.Now()
		period := edge.Sub(lasttick).Milliseconds()
		if period != 0 {
			freq := float64(1000 / period)
			speed := freq * mphPerTick
			livespeed = speed
			lasttick = edge
		}
		time.Sleep(time.Millisecond * 10) // wait for any bouncing to stop
		// 100rps ~ 143MPH -> 10ms per tick, so the above delay won't affect anything
	}
}

func (w *weatherstation) processWindSpeed() {
	samples := make([]float64, 1000)
	pSamples := 0
	avg := 0.0
	max := 0.0
	// initial values
	windspeed.Set(livespeed)
	windgust.Set(livespeed)
	// start ticker
	for t := range time.Tick(time.Millisecond * 250) {
		// record the current speed
		samples[pSamples] = livespeed
		// set livespeed to zero as if the wind stops by next loop we won't know!
		if livespeed > 0 {
			livespeed = 0 // TODO is this also ballsing up calculation
			// are we generating a lot of zeros which are screwing the average?
		}
		pSamples++
		if pSamples == len(samples) {
			pSamples = 0
		}
		if t.Second() == 0 && (pSamples%4) == 0 {
			// happens once per minute
			avg = 0.0
			max = 0.0
			// find max and avg values
			for _, v := range samples {
				avg += v
				if v > max {
					max = v
				}
			}
			avg = avg / float64(len(samples))
			w.windGust = math.Round(max*100) / 100
			windgust.Set(max)
			rAvg := math.Round(avg*100) / 100
			w.windSpeedAvg = rAvg
			windspeed.Set(w.windSpeedAvg)
			logger.Infof("Wind Avg [%.2f] Gust [%.2f]", rAvg, max)
		}
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
