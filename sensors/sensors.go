package sensors

import (
	"errors"
	"flag"
	"math"
	"sync"
	"time"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/devices/bmxx80"
	"periph.io/x/periph/experimental/conn/gpio/gpioutil"
	"periph.io/x/periph/experimental/devices/ads1x15"
	"periph.io/x/periph/experimental/devices/mcp9808"
	"periph.io/x/periph/host"
)

/*
 * Sensors is responsible for reading the sensors and converting sensor output to real values.
 */

const (
	//hgToPa float64 = 133.322387415
	paToInchHg float64 = 0.0002953
)

type PressureInHg float64
type PressurehPa float64
type RelHumidity float64
type TemperatureC float64

type Sensors struct {
	IIC  IIC  // I2C bus sensors
	GPIO GPIO // Direct GPIO sensor inputs
}

type GPIO struct {
	Rainpin  *gpio.PinIO // Rain bucket tip pin
	Windpin  *gpio.PinIO // Wind speed pulse
	counters *counters
}

type counters struct {
	rainTip  int
	rainLock sync.Mutex
}

type IIC struct {
	Atm     *bmxx80.Dev     // BME280 Pressure & humidity
	Temp    *mcp9808.Dev    // MCP9808 temperature sensor
	WindDir *ads1x15.PinADC // wind dir ADC output
	Bus     *i2c.BusCloser
}

func (s *Sensors) InitSensors() error {
	if _, err := host.Init(); err != nil {
		logger.Errorf("Failed to init i2c bus [%v]", err)
		return err
	}
	i2cbus := flag.String("bus", "", "I²C bus (/dev/i2c-1)")
	temperatureAddr := flag.Int("address", 0x18, "I²C address")

	// Open default I²C bus.
	bus, err := i2creg.Open(*i2cbus)
	if err != nil {
		logger.Fatalf("failed to open I²C: %v", err)
		_ = bus.Close()
		return err
	}

	// Create a new temperature sensor a sense with default options.
	tempSensor, err := mcp9808.New(bus, &mcp9808.Opts{Addr: *temperatureAddr})
	if err != nil {
		logger.Errorf("failed to open MCP9808 sensor: %v", err)
		_ = bus.Close()
		return err
	}

	logger.Info("Starting BMP280 reader...")
	bme, err := bmxx80.NewI2C(bus, 0x76, &bmxx80.DefaultOpts)
	if err != nil {
		logger.Errorf("failed to initialize bme280: %v", err)
		_ = bus.Close()
		return err
	}

	logger.Info("Starting MCP9808 Temperature Sensor")

	// Lookup a rainpin by its number:
	rp := gpioreg.ByName("GPIO17")
	if rp == nil {
		logger.Error("Failed to find GPIO17")
		_ = bus.Close()
		return err
	}

	logger.Infof("%s: %s", rp, rp.Function())

	// Set up debounced pin
	// Ignore glitches lasting less than 100ms, and ignore repeated edges within 500ms.
	rainpin, err := gpioutil.Debounce(rp, 100*time.Millisecond, 500*time.Millisecond, gpio.FallingEdge)
	if err != nil {
		logger.Errorf("Failed to set debounce [%v]", err)
		_ = bus.Close()
		return err
	}

	// Lookup a windpin by its number:
	windpin := gpioreg.ByName("GPIO27")
	if windpin == nil {
		logger.Error("Failed to find GPIO27")
		_ = bus.Close()
		return err
	}

	logger.Infof("%s: %s", windpin, windpin.Function())

	if err = windpin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		logger.Error(err)
		_ = bus.Close()
		return err
	}

	logger.Info("Starting Wind direction ADC")
	// Create a new ADS1115 ADC.
	adc, err := ads1x15.NewADS1115(bus, &ads1x15.DefaultOpts)
	if err != nil {
		logger.Error(err)
		_ = bus.Close()
		return err
	}

	// Obtain an analog pin from the ADC.
	dirPin, err := adc.PinForChannel(ads1x15.Channel0, 5*physic.Volt, 1*physic.Hertz, ads1x15.SaveEnergy)
	if err != nil {
		logger.Error(err)
		_ = bus.Close()
		return err
	}
	defer func() { _ = dirPin.Halt() }()

	s.IIC.Atm = bme
	s.IIC.Temp = tempSensor
	s.IIC.Bus = &bus
	s.GPIO.Rainpin = &rainpin
	s.GPIO.Windpin = &windpin
	s.IIC.WindDir = &dirPin
	s.GPIO.counters = &counters{rainTip: 0}

	// start rain bucket monitor
	// this will be replaced when we move to the IIC weather head module
	go s.monitorRainGPIO()
	return nil
}

func (s *Sensors) monitorRainGPIO() {
	logger.Info("Starting tip bucket monitor")
	defer func() { _ = (*s.GPIO.Rainpin).Halt() }()
	for {
		(*s.GPIO.Rainpin).WaitForEdge(-1)
		if (*s.GPIO.Rainpin).Read() == gpio.Low {
			s.GPIO.counters.rainLock.Lock()
			defer s.GPIO.counters.rainLock.Unlock()
			s.GPIO.counters.rainTip += 1
			logger.Infof("Bucket tip. [%v] @ %v", s.GPIO.counters.rainTip, time.Now().Format(time.ANSIC))
		}
	}
}

func (s *Sensors) GetWindCount() int {
	return 0
}

// Return the number of tip events since last read
func (s *Sensors) GetRainCount() int {
	s.GPIO.counters.rainLock.Lock()
	defer s.GPIO.counters.rainLock.Unlock()
	count := s.GPIO.counters.rainTip
	s.GPIO.counters.rainTip = 0
	return count
}

func (s *Sensors) GetHumidityAndPressure() (PressurehPa, RelHumidity, error) {
	em := physic.Env{}
	if s.IIC.Atm != nil {
		if err := s.IIC.Atm.Sense(&em); err != nil {
			logger.Errorf("BME280 read failed [%v]", err)
			return 0, 0, errors.New("Atmospheric sensor read failure")
		}
		// convert raw sensor output
		humidity := RelHumidity(math.Round(float64(em.Humidity) / float64(physic.PercentRH)))
		pressure := PressurehPa(math.Round((float64(em.Pressure)/float64(100*physic.Pascal))*100) / 100)
		//pressureInHg := PressureInHg((float64(em.Pressure) / (float64(physic.Pascal))) * paToInchHg)
		return pressure, humidity, nil
	}
	return 0, 0, errors.New("Atmospheric sensor offline")
}

func (s *Sensors) GetTemperature() (TemperatureC, error) {
	hiT := physic.Env{}
	if s.IIC.Temp != nil {
		if err := s.IIC.Temp.Sense(&hiT); err != nil {
			logger.Errorf("MCP9808 read failed [%v]", err)
			return 0, errors.New("Temperature sensor read failure")
		}
		return TemperatureC(hiT.Temperature.Celsius()), nil
	}
	return 0, errors.New("Temperature sensor offline")
}
