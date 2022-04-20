package sensors

import (
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
	windsensor *windsensor
	rainsensor *rainsensor
}

type windsensor struct {
	windpin    *gpio.PinIO // Wind speed pulse
	pulseCount int
	lastRead   int64
	windLock   sync.Mutex
}

type rainsensor struct {
	rainpin  *gpio.PinIO // Rain bucket tip pin
	rainTip  int
	lastRead int64
	rainLock sync.Mutex
}

type IIC struct {
	Atm     *bmxx80.Dev     // BME280 Pressure & humidity
	Temp    *mcp9808.Dev    // MCP9808 temperature sensor
	WindDir *ads1x15.PinADC // wind dir ADC output
	Bus     *i2c.BusCloser
}

func (s *Sensors) InitSensors() error {
	s.GPIO = GPIO{}
	s.GPIO.rainsensor = &rainsensor{}
	s.GPIO.windsensor = &windsensor{}

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
	s.GPIO.rainsensor.rainpin = &rainpin
	s.GPIO.windsensor.windpin = &windpin
	s.IIC.WindDir = &dirPin

	// start rain bucket monitor
	// this will be replaced when we move to the IIC weather head module
	go s.monitorRainGPIO()
	go s.monitorWindGPIO()
	return nil
}

func (s *Sensors) monitorRainGPIO() {
	logger.Info("Starting tip bucket monitor")
	defer func() { _ = (*s.GPIO.rainsensor.rainpin).Halt() }()
	for {
		func() {
			(*s.GPIO.rainsensor.rainpin).WaitForEdge(-1)
			if (*s.GPIO.rainsensor.rainpin).Read() == gpio.Low {
				s.GPIO.rainsensor.rainLock.Lock()
				defer s.GPIO.rainsensor.rainLock.Unlock()
				s.GPIO.rainsensor.rainTip += 1
				logger.Infof("Bucket tip. [%v] @ %v", s.GPIO.rainsensor.rainTip, time.Now().Format(time.ANSIC))
			}
		}()
	}
}

func (s *Sensors) monitorWindGPIO() {
	logger.Info("Starting wind sensor")
	defer func() { _ = (*s.GPIO.windsensor.windpin).Halt() }()
	for {
		func() {
			s.GPIO.windsensor.windLock.Lock()
			defer s.GPIO.windsensor.windLock.Unlock()
			(*s.GPIO.windsensor.windpin).WaitForEdge(-1)
			s.GPIO.windsensor.pulseCount += 1
		}()
	}
}

func (s *Sensors) GetWindCount() int {
	s.GPIO.windsensor.windLock.Lock()
	defer s.GPIO.windsensor.windLock.Unlock()
	s.GPIO.windsensor.lastRead = time.Now().UnixMilli()
	count := s.GPIO.windsensor.pulseCount
	s.GPIO.windsensor.pulseCount = 0
	return count
}

// Return the number of tip events since last read
func (s *Sensors) GetRainCount() int {
	s.GPIO.rainsensor.rainLock.Lock()
	defer s.GPIO.rainsensor.rainLock.Unlock()
	s.GPIO.rainsensor.lastRead = time.Now().UnixMilli()
	count := s.GPIO.rainsensor.rainTip
	s.GPIO.rainsensor.rainTip = 0
	return count
}

func (s *Sensors) GetWindDirection() float64 {
	sample, err := (*s.IIC.WindDir).Read()
	if err != nil {
		logger.Errorf("Error reading wind direction value [%v]", err)
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
