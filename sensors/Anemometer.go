package sensors

import (
	"time"

	"github.com/pointer2null/weather/buffer"
	"github.com/pointer2null/weather/env"
	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/experimental/devices/ads1x15"
)

type Anemometer struct {
	dirADC   *ads1x15.PinADC
	Bus      *i2c.Bus
	speedBuf *buffer.SampleBuffer
	gustBuf  *buffer.SampleBuffer
	dirBuf   *buffer.SampleBuffer
	DirStr   string
	masthead *i2c.Dev
	args     env.Args
}

var lastVal float64 = 0

func NewAnemometer(bus *i2c.Bus, args env.Args) *Anemometer {
	a := &Anemometer{}
	a.args = args
	a.Bus = bus

	logger.Infof("Starting Masthead I2C [%x] Speed test flag is %v", env.MastHead, *a.args.Speedon)
	a.masthead = &i2c.Dev{Addr: env.MastHead, Bus: *bus}

	logger.Infof("Starting Wind direction ADC I2C [%x] Dir test flag is %v", ads1x15.DefaultOpts.I2cAddress, *a.args.Diron)
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
	// check connection
	if err := a.masthead.Tx([]byte{0x00}, make([]byte, 4)); err != nil {
		logger.Errorf("Masthead did not respond [%v]", err)
		return nil
	}

	sps := env.WindSamplesPerSecond
	if *a.args.Test {
		sps = 1
	}
	// 4 samples per sec, for 1 mins = 60 * 4 = 240
	a.speedBuf = buffer.NewBuffer(sps * env.WindBufferLengthSeconds)
	// 4 samples per sec, for 1 mins = 60 * 4 = 240
	a.gustBuf = buffer.NewBuffer(sps * env.WindBufferLengthSeconds)
	a.dirBuf = buffer.NewBuffer(sps * env.WindBufferLengthSeconds)

	a.monitorWindGPIO()

	return a
}

func (a *Anemometer) monitorWindGPIO() {
	logger.Info("Starting wind sensor")

	period := time.Millisecond * (time.Second / time.Millisecond / env.WindSamplesPerSecond)
	if *a.args.Test {
		logger.Info("Wind sensor period set to 1 second for test")
		period = time.Second * 1
	}

	go func() {
		// record the count every 250ms
		write := []byte{0x00} // we don't need to send any command
		read := make([]byte, 2)
		for range time.Tick(period) {
			if err := a.masthead.Tx(write, read); err != nil {
				logger.Errorf("Failed to request count from masthead [%v]", err)
			}
			pulseCount := uint32(read[0])
			if pulseCount > 25 {
				logger.Errorf("Pulse count error [%v] [%v] [%b]", pulseCount, read, read)
				pulseCount = 0
			}
			a.speedBuf.AddItem(float64(pulseCount))
			a.gustBuf.AddItem(float64(pulseCount))
			if pulseCount > 0 || *a.args.Diron {
				a.dirBuf.AddItem(a.readDirection())
			} else {
				// if we have no wind the dir is garbage
				a.dirBuf.AddItem(a.dirBuf.GetLast())
			}
			if *a.args.Speedon {
				logger.Infof("MPH raw [%.2f], calc [%v] Count read [%v]", (float64(pulseCount) * env.MphPerTick), a.GetSpeed(), pulseCount)
			}
		}
	}()
}

func (a *Anemometer) GetSpeed() float64 { // WindBufferLengthSeconds min rolling average
	// the buffer contains pulse counts.
	avg, _, _, _ := a.speedBuf.GetAverageMinMaxSum()
	// avg ticks per 1/env.WindSamplesPerSecond seconds
	ticksPerSec := avg * env.WindSamplesPerSecond
	if ticksPerSec < 0 {
		d, _, _ := a.speedBuf.GetRawData()
		logger.Errorf("INVALID TicksPerSecond [%v]\n[%v]", ticksPerSec, d)
		ticksPerSec = 0
	}
	// so the avg speed for the last WindBufferLengthSeconds seconds is...
	speed := env.MphPerTick * float64(ticksPerSec)
	if speed > 100 {
		d, _, p := a.speedBuf.GetRawData()
		logger.Errorf("Speed valculation error [%v] pos [%v]\n%v", speed, p, d)
		speed = 0
	}
	return speed
}

func (a *Anemometer) GetGust() float64 { // "the maximum three second average wind speed occurring in any period (10 min)"
	const threeSecond = 3
	data, s, _ := a.gustBuf.GetRawData()
	size := int(s)
	// make an array for the 3 second rolling average
	threeSecMax := 0.0
	x := 0.0

	for i := 0; i < size; i++ {
		x = 0
		for j := 0; j < (env.WindSamplesPerSecond * threeSecond); j++ {
			x += (data[getWrappedIndex(i+j, size)])
		}
		// x is the 3 second average
		if x > threeSecMax {
			threeSecMax = x
		}
	}
	// we still occasionally get stupid values (500MPH)
	// these are either caused by em interference or by
	// switch bounce. Either way we need to filter them out
	// until we can find the root cause and remove it.
	val := (threeSecMax / threeSecond) * env.MphPerTick
	if val > 120 {
		val = lastVal
	}
	lastVal = val
	return val
}

func getWrappedIndex(x int, size int) int {
	if x >= size {
		return x - size
	}
	return x
}

func (a *Anemometer) GetDirection() float64 {
	avg, _, _, _ := a.dirBuf.GetAverageMinMaxSum()
	return float64(avg)
}

func (a *Anemometer) readDirection() float64 {
	sample, err := (*a.dirADC).Read()
	if err != nil {
		logger.Debugf("Error reading wind direction value [%v]", err)
		return a.dirBuf.GetLast()
	}
	deg, str := voltToDegrees(float64(sample.V) / float64(physic.Volt))
	a.DirStr = str
	if *a.args.Diron {
		logger.Infof("Volts [%v], Deg [%v] : %s", float64(sample.V)/float64(physic.Volt), deg, str)
	}
	return deg
}

func voltToDegrees(v float64) (float64, string) {
	// this is based on actual measurements of output voltage for each cardinal point
	// threhold voltage is midway between the two recorded values.
	switch {
	case v < 1.19:
		return 135, "SE"
	case v < 1.46:
		return 180, "S"
	case v < 2.09:
		return 90, "E"
	case v < 2.8:
		return 45, "NE"
	case v < 3.56:
		return 225, "SW"
	case v < 4.2:
		return 0, "N"
	case v < 4.59:
		return 315, "NW"
	default:
		return 270.0, "W"
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
