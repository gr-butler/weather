package sensors

import (
	"flag"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/host"
)

type Sensors struct {
	Atm    *atmosphere
	Rain   *rainmeter
	Wind   *anemometer
	Closer *i2c.BusCloser
}

func InitSensors(testmode bool, verbose bool) *Sensors {
	s := &Sensors{}

	if _, err := host.Init(); err != nil {
		logger.Fatalf("Failed to init i2c bus [%v]", err)
		return nil
	}
	i2cbus := flag.String("bus", "", "I²C bus (/dev/i2c-1)")

	// Open default I²C bus.
	closer, err := i2creg.Open(*i2cbus)
	if err != nil {
		logger.Fatalf("failed to open I²C: %v", err)
		_ = closer.Close()
		return nil
	}
	s.Closer = &closer
	bus := i2c.Bus(closer)
	//bus.SetSpeed(physic.KiloHertz) // just in case - we're pushing our luck with the IIC bus length

	s.Atm = NewAtmosphere(&bus, testmode)
	s.Rain = NewRainmeter(&bus)
	s.Wind = NewAnemometer(&bus, verbose)

	return s
}
