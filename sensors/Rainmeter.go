package sensors

import (
	"time"

	"github.com/gr-butler/weather/buffer"
	"github.com/gr-butler/weather/env"
	"github.com/gr-butler/weather/led"
	logger "github.com/sirupsen/logrus"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/gpio/gpioutil"
	"periph.io/x/conn/v3/i2c"
)

type rainmeter struct {
	gpioPin           *gpio.PinIO // Rain bucket tip pin
	dayAccumulation   int64
	accumulationSince int64
	ledOut            *led.LED
	tipBuf            *buffer.SampleBuffer
	args              env.Args
}

type mmHr float64
type mm float64

func (m mmHr) Float64() float64 {
	return float64(m)
}

func toMMHr(v float64) mmHr {
	return mmHr(v)
}

func (m mm) Float64() float64 {
	return float64(m)
}

func toMM(v int64) mm {
	return mm(v)
}

func NewRainmeter(bus *i2c.Bus, args env.Args) *rainmeter {
	r := &rainmeter{}
	r.args = args

	// Lookup a rainpin by its number:
	rp := gpioreg.ByName(env.RainSensorIn)
	if rp == nil {
		logger.Errorf("Failed to find %v - rain pin", env.RainSensorIn)
		return nil
	}

	logger.Infof("Rain Pin: %s: %s", rp, rp.Function())

	// Set up debounced pin
	// Ignore glitches lasting less than 10ms, and ignore repeated edges within 500ms.
	rainpin, err := gpioutil.Debounce(rp, 10*time.Millisecond, 500*time.Millisecond, gpio.FallingEdge)
	if err != nil {
		logger.Errorf("Failed to set debounce [%v]", err)
		return nil
	}
	r.gpioPin = &rainpin

	r.ledOut = led.NewLED("Rain Tip", env.RainTipLed)

	// every 10 seconds for last hour = 3600 / 10 = 360
	r.tipBuf = buffer.NewBuffer(360)
	r.monitorRainGPIO()
	return r
}

func (r *rainmeter) GetRate() mmHr {
	_, _, _, sum := r.tipBuf.GetAverageMinMaxSum()
	return toMMHr(env.MMPerBucketTip * float64(sum))
}

func (r *rainmeter) GetMinuteRate() mm {
	sum, _, _ := r.tipBuf.SumMinMaxLast(6) // last minute
	return mm(int64(env.MMPerBucketTip * sum))
}

func (r *rainmeter) GetDayAccumulation() mm {
	return toMM(r.dayAccumulation)
}

func (r *rainmeter) ResetDayAccumulation() {
	r.dayAccumulation = 0
}

// returns the accumulation since last called.
func (r *rainmeter) GetAccumulation() mm {
	a := r.accumulationSince
	r.accumulationSince = 0
	return toMM(a)
}

func (r *rainmeter) monitorRainGPIO() {
	logger.Info("Starting tip bucket monitor")
	rainTip := 0
	go func() {
		defer func() { _ = (*r.gpioPin).Halt() }()
		for {
			(*r.gpioPin).WaitForEdge(-1)
			if (*r.gpioPin).Read() == gpio.Low {
				rainTip += 1             // for rates
				r.dayAccumulation += 1   // for day
				r.accumulationSince += 1 // for accumulations

				logger.Infof("Bucket tip. [%v] @ %v", rainTip, time.Now().Format(time.ANSIC))

				r.ledOut.Flash()
			}
		}
	}()
	go func() {
		// record the count every ten seconds
		for range time.Tick(time.Second * 10) {
			r.tipBuf.AddItem(float64(rainTip))
			rainTip = 0
		}
	}()
}

func (r *rainmeter) GetLED() *led.LED {
	return r.ledOut
}
