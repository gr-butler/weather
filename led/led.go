package led

import (
	"sync"
	"time"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
)

type LED struct {
	Name    string
	lock    *sync.Mutex
	on      bool
	blink   chan bool
	Close   chan bool
	gpioPin gpio.PinIO
}

func NewLED(name string, GPIOPin string) *LED {
	logger.Infof("Creating new LED on pin [%v] called [%v]", GPIOPin, name)
	l := &LED{
		Name:  name,
		lock:  &sync.Mutex{},
		on:    false,
		blink: make(chan bool),
	}
	l.gpioPin = gpioreg.ByName(GPIOPin)
	if l.gpioPin == nil {
		logger.Errorf("Failed to find %v pin", GPIOPin)

	}

	// flicker to show it's working
	_ = l.gpioPin.Out(gpio.Low)
	_ = l.gpioPin.Out(gpio.High)
	time.Sleep(time.Millisecond * 250)
	_ = l.gpioPin.Out(gpio.Low)
	time.Sleep(time.Millisecond * 100)
	_ = l.gpioPin.Out(gpio.High)
	time.Sleep(time.Millisecond * 100)
	_ = l.gpioPin.Out(gpio.Low)
	time.Sleep(time.Millisecond * 100)
	_ = l.gpioPin.Out(gpio.High)
	time.Sleep(time.Millisecond * 100)
	_ = l.gpioPin.Out(gpio.Low)

	go func() {
		for { //nolint: gosimple
			select {
			case <-l.blink:
				l.Flash()
			case <-l.Close:
				l.Off()
				return
			}
		}
	}()
	return l
}

func (l *LED) On() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.on = true
	if l.gpioPin != nil {
		_ = l.gpioPin.Out(gpio.High)
	}
}

func (l *LED) Off() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.on = false
	if l.gpioPin != nil {
		_ = l.gpioPin.Out(gpio.Low)
	}
}

func (l *LED) Flash() {
	if l.gpioPin == nil {
		logger.Infof("No such LED [%v]", l.gpioPin)
		return
	}
	if !l.lock.TryLock() {
		// despite the function description, this is a valid use case. We don't want lots of
		// flash requests all queuing waiting on the mutex, if a flash is in progress we can
		// safely discard the current request.
		logger.Infof("LED Locked[%v]", l.gpioPin)
		return
	}
	defer l.lock.Unlock()
	// if the LED is currently off, then flash on
	if !l.on {
		_ = l.gpioPin.Out(gpio.High)
		time.Sleep(time.Millisecond * 100)
		_ = l.gpioPin.Out(gpio.Low)
	} else {
		// 'off' flash
		_ = l.gpioPin.Out(gpio.Low)
		time.Sleep(time.Millisecond * 100)
		_ = l.gpioPin.Out(gpio.High)
	}
}

func (l *LED) Flicker(pulses int) {
	if l.gpioPin == nil {
		return
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	if pulses < 1 || pulses > 100 {
		// reject daft or excessive requests
		return
	}
	for i := 0; i < pulses; i++ {
		_ = l.gpioPin.Out(gpio.High)
		time.Sleep(time.Millisecond * 100)
		_ = l.gpioPin.Out(gpio.Low)
	}
}

func (l *LED) IsOn() bool {
	return l.on
}
