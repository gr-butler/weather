package sensors

import (
	"sync"
	"time"

	"github.com/pointer2null/weather/constants"
	"github.com/pointer2null/weather/utils"
	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/experimental/devices/ads1x15"
)

type anemometer struct {
	gpioPin    *gpio.PinIO
	pulseCount int
	windLock   sync.Mutex
	dirADC     *ads1x15.PinADC
	Bus        *i2c.Bus
	speedBuf   *utils.SampleBuffer
	gustBuf    *utils.SampleBuffer
	dirBuf     *utils.SampleBuffer
}

func (a *anemometer) NewAnemometer(bus *i2c.Bus) *anemometer {
	a = &anemometer{}
	a.Bus = bus
	// Lookup a windpin by its number:
	windpin := gpioreg.ByName(constants.WindSensorIn)
	if windpin == nil {
		logger.Errorf("Failed to find %v - wind pin", constants.WindSensorIn)
		return nil
	}

	logger.Infof("%s: %s", windpin, windpin.Function())

	if err := windpin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		logger.Error(err)
		return nil
	}
	a.gpioPin = &windpin

	logger.Info("Starting Wind direction ADC")
	// Create a new ADS1115 ADC.
	adc, err := ads1x15.NewADS1115(*a.Bus, &ads1x15.DefaultOpts)
	if err != nil {
		logger.Error(err)
		return nil
	}

	// Obtain an analog pin from the ADC.
	dirPin, err := adc.PinForChannel(ads1x15.Channel0, 5*physic.Volt, 1*physic.Hertz, ads1x15.SaveEnergy)
	if err != nil {
		logger.Error(err)
		return nil
	}
	a.dirADC = &dirPin

	// 4 samples per sec, for 2 mins = 120 * 4 = 480
	a.speedBuf = utils.NewBuffer(480)
	// 4 samples per sec, for 10 mins = 600 * 4 = 2400
	a.gustBuf = utils.NewBuffer(2400)
	a.dirBuf = utils.NewBuffer(480)
	go a.monitorWindGPIO()

	return a
}

func (a *anemometer) monitorWindGPIO() {
	logger.Info("Starting wind sensor")
	defer func() { _ = (*a.gpioPin).Halt() }()
	pulseCount := 0
	// count any pulses
	go func() {
		for {
			(*a.gpioPin).WaitForEdge(-1)
			pulseCount += 1
		}
	}()
	// record the count every 250ms
	for range time.Tick(time.Millisecond * 250) {
		a.speedBuf.AddItem(float64(pulseCount))
		pulseCount = 0
		a.dirBuf.AddItem(a.readDirection())
	}
}

// https://www.metoffice.gov.uk/weather/guides/observations/how-we-measure-wind

// Because wind is an element that varies rapidly over very short periods of time
// it is sampled at high frequency (every 0.25 sec)

func (a *anemometer) GetSpeed() float64 { // 2 min rolling average
	// the buffer contains pulse counts.
	avg, _, _, _ := a.speedBuf.GetAverageMinMaxSum()
	return constants.MphPerTick * float64(avg)
}

func (a *anemometer) GetGust() float64 { // "the maximum three second average wind speed occurring in any period (10 min)"
	data, s, _ := a.gustBuf.GetRawData()
	size := int(s)
	// make an array for the 3 second rolling average
	avg := 0.0
	x := 0.0

	for i := 0; i < size; i++ {
		x = (data[getRolledIndex(i, size)] +
			data[getRolledIndex(i+1, size)] +
			data[getRolledIndex(i+2, size)]) / 3
		if x > avg {
			avg = x
		}
	}
	return avg
}

func getRolledIndex(x int, size int) int {
	if x >= size {
		return x - size
	}
	return x
}

func (a *anemometer) GetDirection() float64 {
	avg, _, _, _ := a.dirBuf.GetAverageMinMaxSum()
	return voltToDegrees(float64(avg))
}

func (a *anemometer) readDirection() float64 {
	sample, err := (*a.dirADC).Read()
	if err != nil {
		logger.Debugf("Error reading wind direction value [%v]", err)
		sample.Raw = 0
	}

	return voltToDegrees(float64(sample.V) / float64(physic.Volt))
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
