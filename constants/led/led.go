package led

import (
	"sync"
	"time"

	"periph.io/x/periph/conn/gpio"
)

type LED struct {
	Name    string
	lock    *sync.Mutex
	on      bool
	blink   chan bool
	Close   chan bool
	gpioPin *gpio.PinIO
}

func NewLED(name string, IOPin *gpio.PinIO) *LED {
	l := &LED{
		Name:    name,
		lock:    &sync.Mutex{},
		on:      false,
		blink:   make(chan bool),
		gpioPin: IOPin,
	}
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
	_ = (*l.gpioPin).Out(gpio.High)
}

func (l *LED) Off() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.on = false
	_ = (*l.gpioPin).Out(gpio.Low)
}

func (l *LED) Flash() {
	if !l.lock.TryLock() {
		// despite the function description, this is a valid use case. We don't want lots of
		// flash requests all queuing waiting on the mutex, if a flash is in progress we can
		// safely discard the current request.
		return
	}
	defer l.lock.Unlock()
	// if the LED is currently off, then flash on
	if l.on {
		_ = (*l.gpioPin).Out(gpio.High)
		time.Sleep(time.Millisecond * 100)
		_ = (*l.gpioPin).Out(gpio.Low)
	} else {
		// 'off' flash
		_ = (*l.gpioPin).Out(gpio.Low)
		time.Sleep(time.Millisecond * 100)
		_ = (*l.gpioPin).Out(gpio.High)
	}
}

func (l *LED) Flicker(pulses int) {
	l.lock.Lock()
	defer l.lock.Unlock()
	if pulses < 1 || pulses > 100 {
		// reject daft or excessive requests
		return
	}
	for i := 0; i < pulses; i++ {
		_ = (*l.gpioPin).Out(gpio.High)
		time.Sleep(time.Millisecond * 100)
		_ = (*l.gpioPin).Out(gpio.Low)
	}
}
