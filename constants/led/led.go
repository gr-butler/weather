package led

import (
	"sync"

	"periph.io/x/periph/conn/gpio"
)

type LED struct {
	Name    string
	Lock    sync.Mutex
	State   bool
	Flash   chan bool
	gpioPin *gpio.PinIO
}

func NewLED(name string, IOPin *gpio.PinIO) *LED {
	l := &LED{
		Name:    name,
		Lock:    sync.Mutex{},
		State:   false,
		Flash:   make(chan bool),
		gpioPin: IOPin,
	}
	return l
}
