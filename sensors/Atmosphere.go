package sensors

import (
	"flag"
	"math"

	"github.com/gr-butler/devices/htu21d"
	"github.com/gr-butler/weather/env"
	logger "github.com/sirupsen/logrus"

	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/devices/v3/bmxx80"
	"periph.io/x/devices/v3/mcp9808"
)

const (
	MCP9808_I2C = 0x18
	BMP280_I2C  = 0x76
)

type PressurehPa float64
type RelHumidity float64
type TemperatureC float64

func (p PressurehPa) Float64() float64 {
	return float64(p)
}

func (r RelHumidity) Float64() float64 {
	return float64(r)
}

func (t TemperatureC) Float64() float64 {
	return float64(t)
}

type atmosphere struct {
	PH   *bmxx80.Dev  // BME280 Pressure & humidity
	Temp *mcp9808.Dev // MCP9808 temperature sensor
	args env.Args
}

func NewAtmosphere(bus *i2c.Bus, args env.Args) *atmosphere {
	a := &atmosphere{}
	a.args = args

	temperatureAddr := flag.Int("address", MCP9808_I2C, "I²C address")
	logger.Infof("Starting MCP9808 Temperature Sensor [%x]", MCP9808_I2C)
	// Create a new temperature sensor with hig res
	tempSensor, err := mcp9808.New(*bus, &mcp9808.Opts{Addr: *temperatureAddr, Res: mcp9808.High})
	if err != nil {
		logger.Errorf("Failed to open MCP9808 sensor: %v", err)
		return nil
	}
	a.Temp = tempSensor

	logger.Infof("Starting BMP280 reader [%x]", BMP280_I2C)
	bme, err := bmxx80.NewI2C(*bus, BMP280_I2C, &bmxx80.DefaultOpts)
	if err != nil {
		logger.Errorf("failed to initialize bme280: %v", err)
		return nil
	}
	a.PH = bme

	htu21d.NewI2C(*bus, 0x40, &htu21d.Opts{})

	return a
}

func (a *atmosphere) GetHumidityAndPressure() (PressurehPa, RelHumidity) {
	em := physic.Env{}
	if a.PH != nil {
		if err := a.PH.Sense(&em); err != nil {
			logger.Errorf("BME280 read failed [%v]", err)
			return 0, 0
		}
		// convert raw sensor output
		if *a.args.Humidity {
			logger.Infof("Hum raw [%v]", em.Humidity)
		}
		humidity := RelHumidity(math.Round(float64(em.Humidity) / float64(physic.PercentRH)))
		pressure := PressurehPa(math.Round((float64(em.Pressure)/float64(100*physic.Pascal))*100) / 100)

		return pressure, humidity
	}
	return 0, 0
}

func (a *atmosphere) GetTemperature() TemperatureC {
	hiT := physic.Env{}
	if a.Temp != nil {
		if err := a.Temp.Sense(&hiT); err != nil {
			logger.Errorf("MCP9808 read failed [%v]", err)
			return 0
		}
		temp := TemperatureC(hiT.Temperature.Celsius())

		return temp
	}
	return 0
}
