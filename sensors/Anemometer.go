package sensors

import (
	"encoding/binary"
	"time"

	"github.com/pointer2null/weather/buffer"
	"github.com/pointer2null/weather/constants"
	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/experimental/devices/ads1x15"
)

type anemometer struct {
	dirADC   *ads1x15.PinADC
	Bus      *i2c.Bus
	speedBuf *buffer.SampleBuffer
	gustBuf  *buffer.SampleBuffer
	dirBuf   *buffer.SampleBuffer
	verbose  bool
	masthead *i2c.Dev
}

const MastHead uint16 = 0x55

func NewAnemometer(bus *i2c.Bus, verbose bool) *anemometer {
	a := &anemometer{}
	a.verbose = verbose
	a.Bus = bus

	a.masthead = &i2c.Dev{Addr: MastHead, Bus: *bus}

	logger.Info("Starting Wind direction ADC")
	// Create a new ADS1115 ADC.
	adc, err := ads1x15.NewADS1115(*a.Bus, &ads1x15.DefaultOpts)
	if err != nil {
		logger.Error(err)
		return nil
	}

	// Obtain an analog pin from the ADC.
	dirPin, err := adc.PinForChannel(ads1x15.Channel3, 5*physic.Volt, 1*physic.Hertz, ads1x15.SaveEnergy)
	if err != nil {
		logger.Error(err)
		return nil
	}
	a.dirADC = &dirPin

	// 4 samples per sec, for 2 mins = 120 * 4 = 480
	a.speedBuf = buffer.NewBuffer(480)
	// 4 samples per sec, for 10 mins = 600 * 4 = 2400
	a.gustBuf = buffer.NewBuffer(2400)
	a.dirBuf = buffer.NewBuffer(1200)
	a.monitorWindGPIO()

	return a
}

func (a *anemometer) monitorWindGPIO() {
	logger.Info("Starting wind sensor")

	go func() {
		// record the count every 250ms
		write := []byte{0x00} // we don't need to send any command
		read := make([]byte, 4)
		for range time.Tick(time.Millisecond * 250) {
			if err := a.masthead.Tx(write, read); err != nil {
				logger.Errorf("Failed to request count from masthead [%v]", err)
			}
			pulseCount := int(binary.LittleEndian.Uint32(read))
			a.speedBuf.AddItem(float64(pulseCount))
			a.gustBuf.AddItem(float64(pulseCount))
			if pulseCount < 1 {
				// if we have no wind the dir is garbage
				a.dirBuf.AddItem(a.dirBuf.GetLast())
			} else {
				a.dirBuf.AddItem(a.readDirection())
			}
			if a.verbose {
				logger.Infof("Dir [%v], MPH [%.2f] Count [%v]", a.readDirection(), (float64(pulseCount*4.0) * constants.MphPerTick), pulseCount)
			}
		}
	}()
}

// https://www.metoffice.gov.uk/weather/guides/observations/how-we-measure-wind

// Because wind is an element that varies rapidly over very short periods of time
// it is sampled at high frequency (every 0.25 sec)

func (a *anemometer) GetSpeed() float64 { // 2 min rolling average
	// the buffer contains pulse counts.
	_, _, _, sum := a.speedBuf.GetAverageMinMaxSum()
	// sum is the total pulse count for 2 mins
	ticksPerSec := sum / (2 * 60)
	// so the avg speed for the last 2 mins is...
	return constants.MphPerTick * float64(ticksPerSec)
}

func (a *anemometer) GetGust() float64 { // "the maximum three second average wind speed occurring in any period (10 min)"
	data, s, _ := a.gustBuf.GetRawData()
	size := int(s)
	// make an array for the 3 second rolling average
	threeSecMax := 0.0
	x := 0.0

	for i := 0; i < size; i++ {
		// 4 samples per second...
		x = (data[getRolledIndex(i, size)] +
			data[getRolledIndex(i+1, size)] +
			data[getRolledIndex(i+2, size)] +
			data[getRolledIndex(i+3, size)] +
			data[getRolledIndex(i+4, size)] +
			data[getRolledIndex(i+5, size)] +
			data[getRolledIndex(i+6, size)] +
			data[getRolledIndex(i+7, size)] +
			data[getRolledIndex(i+8, size)] +
			data[getRolledIndex(i+9, size)] +
			data[getRolledIndex(i+10, size)] +
			data[getRolledIndex(i+11, size)] +
			data[getRolledIndex(i+12, size)] +
			data[getRolledIndex(i+13, size)] +
			data[getRolledIndex(i+14, size)] +
			data[getRolledIndex(i+15, size)])
		if x > threeSecMax {
			threeSecMax = x
		}
	}

	return (threeSecMax / 3) * constants.MphPerTick
}

func getRolledIndex(x int, size int) int {
	if x >= size {
		return x - size
	}
	return x
}

func (a *anemometer) GetDirection() float64 {
	avg, _, _, _ := a.dirBuf.GetAverageMinMaxSum()
	return float64(avg)
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
