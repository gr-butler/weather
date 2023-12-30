package sensors

import (
	"flag"
	"math"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/devices/bmxx80"
	"periph.io/x/periph/experimental/devices/ads1x15"
	"periph.io/x/periph/experimental/devices/mcp9808"
	"periph.io/x/periph/host"
)

/*
 * Sensors is responsible for reading the sensors and converting sensor output to real values.
 */

type PressureInHg float64
type PressurehPa float64
type RelHumidity float64
type TemperatureC float64

type Sensors struct {
	IIC  IIC       // I2C bus sensors
	Port GPIO_port // Direct GPIO sensor inputs
}

type GPIO_port struct {
	windsensor *anemometer
	rainsensor *rainmeter
}

type IIC struct {
	Atm     *bmxx80.Dev     // BME280 Pressure & humidity
	Temp    *mcp9808.Dev    // MCP9808 temperature sensor
	WindDir *ads1x15.PinADC // wind dir ADC output
	Bus     *i2c.BusCloser
}

func (s *Sensors) InitSensors() error {
	s.Port = GPIO_port{}
	s.Port.rainsensor = &rainmeter{}
	s.Port.windsensor = &anemometer{}

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
	s.IIC.Bus = &bus

	logger.Info("Starting MCP9808 Temperature Sensor")
	// Create a new temperature sensor with hig res
	tempSensor, err := mcp9808.New(bus, &mcp9808.Opts{Addr: *temperatureAddr, Res: mcp9808.High})
	if err != nil {
		logger.Errorf("failed to open MCP9808 sensor: %v", err)
		_ = bus.Close()
		return err
	}
	s.IIC.Temp = tempSensor

	logger.Info("Starting BMP280 reader...")
	bme, err := bmxx80.NewI2C(bus, 0x76, &bmxx80.DefaultOpts)
	if err != nil {
		logger.Errorf("failed to initialize bme280: %v", err)
		_ = bus.Close()
		return err
	}
	s.IIC.Atm = bme

	logger.Info("Sensors initialized.")
	// start rain bucket monitor
	// this will be replaced when we move to the IIC weather head module

	go s.monitorWindGPIO()
	return nil
}

func (s *Sensors) monitorWindGPIO() {
	logger.Info("Starting wind sensor")
	defer func() { _ = (*s.Port.windsensor.gpioPin).Halt() }()
	for {
		func() {
			(*s.Port.windsensor.gpioPin).WaitForEdge(-1)
			s.Port.windsensor.pulseCount += 1
		}()
	}
}

func (s *Sensors) GetWindCount() int {
	s.Port.windsensor.windLock.Lock()
	defer s.Port.windsensor.windLock.Unlock()
	count := s.Port.windsensor.pulseCount
	s.Port.windsensor.pulseCount = 0
	return count
}

// Return the number of tip events since last read
func (s *Sensors) GetRainCount() int {
	s.Port.rainsensor.rainLock.Lock()
	defer s.Port.rainsensor.rainLock.Unlock()
	// count := s.Port.rainsensor.rainTip
	// s.Port.rainsensor.rainTip = 0
	return 0
}

func (s *Sensors) GetWindDirection() float64 {
	sample, err := (*s.IIC.WindDir).Read()
	if err != nil {
		logger.Debugf("Error reading wind direction value [%v]", err)
		sample.Raw = 0
	}

	return voltToDegrees(float64(sample.V) / float64(physic.Volt))
}

func (s *Sensors) GetHumidityAndPressure() (PressurehPa, RelHumidity) {
	em := physic.Env{}
	if s.IIC.Atm != nil {
		if err := s.IIC.Atm.Sense(&em); err != nil {
			logger.Errorf("BME280 read failed [%v]", err)
			return 0, 0
		}
		// convert raw sensor output
		humidity := RelHumidity(math.Round(float64(em.Humidity) / float64(physic.PercentRH)))
		pressure := PressurehPa(math.Round((float64(em.Pressure)/float64(100*physic.Pascal))*100) / 100)
		//pressureInHg := PressureInHg((float64(em.Pressure) / (float64(physic.Pascal))) * paToInchHg)
		return pressure, humidity
	}
	return 0, 0
}

func (s *Sensors) GetTemperature() TemperatureC {
	hiT := physic.Env{}
	if s.IIC.Temp != nil {
		if err := s.IIC.Temp.Sense(&hiT); err != nil {
			logger.Errorf("MCP9808 read failed [%v]", err)
			return 0
		}
		return TemperatureC(hiT.Temperature.Celsius())
	}
	return 0
}
