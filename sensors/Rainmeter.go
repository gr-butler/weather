package sensors

import (
	"sync"
	"time"

	"github.com/pointer2null/weather/buffer"
	"github.com/pointer2null/weather/constants"
	"github.com/pointer2null/weather/led"
	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/experimental/conn/gpio/gpioutil"
)

type rainmeter struct {
	gpioPin      *gpio.PinIO // Rain bucket tip pin
	accumulation int64
	rainLock     sync.Mutex
	ledOut       *led.LED
	tipBuf       *buffer.SampleBuffer
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

func NewRainmeter(bus *i2c.Bus) *rainmeter {
	r := &rainmeter{}

	// Lookup a rainpin by its number:
	rp := gpioreg.ByName(constants.RainSensorIn)
	if rp == nil {
		logger.Errorf("Failed to find %v - rain pin", constants.RainSensorIn)
		return nil
	}

	logger.Infof("%s: %s", rp, rp.Function())

	// Set up debounced pin
	// Ignore glitches lasting less than 100ms, and ignore repeated edges within 500ms.
	rainpin, err := gpioutil.Debounce(rp, 100*time.Millisecond, 500*time.Millisecond, gpio.FallingEdge)
	if err != nil {
		logger.Errorf("Failed to set debounce [%v]", err)
		return nil
	}
	r.gpioPin = &rainpin

	rainTipLed := gpioreg.ByName(constants.RainTipLed)
	if rainTipLed == nil {
		logger.Errorf("Failed to find %v - rain tip LED pin", constants.RainTipLed)
		// failed raintip LED is not critical
	}
	_ = rainTipLed.Out(gpio.Low)
	r.ledOut = led.NewLED("Rain Tip", &rainTipLed)

	// every 10 seconds for last hour = 3600 / 10 = 360
	r.tipBuf = buffer.NewBuffer(360)
	go r.monitorRainGPIO()
	return r
}

func (r *rainmeter) GetRate() mmHr {

	_, _, _, sum := r.tipBuf.GetAverageMinMaxSum()

	return toMMHr(constants.MMPerBucketTip * float64(sum))
}

func (r *rainmeter) GetAccumulation() mm {
	return toMM(r.accumulation)
}

func (r *rainmeter) ResetAccumulation() {
	r.accumulation = 0
}

func (r *rainmeter) monitorRainGPIO() {
	logger.Info("Starting tip bucket monitor")
	defer func() { _ = (*r.gpioPin).Halt() }()
	rainTip := 0
	go func() {
		for {
			(*r.gpioPin).WaitForEdge(-1)
			if (*r.gpioPin).Read() == gpio.Low {
				rainTip += 1
				r.accumulation += 1
				logger.Infof("Bucket tip. [%v] @ %v", rainTip, time.Now().Format(time.ANSIC))
				r.ledOut.Flash()
			}
		}
	}()
	// record the count every ten seconds
	for range time.Tick(time.Second * 10) {
		r.tipBuf.AddItem(float64(rainTip))
		rainTip = 0
	}
}
