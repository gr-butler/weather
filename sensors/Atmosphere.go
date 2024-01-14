package sensors

import (
	"flag"
	"math"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/devices/bmxx80"
	"periph.io/x/periph/experimental/devices/mcp9808"
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
	test bool
}

func NewAtmosphere(bus *i2c.Bus, testmode bool) *atmosphere {
	a := &atmosphere{}
	a.test = testmode

	temperatureAddr := flag.Int("address", 0x18, "I²C address")
	logger.Info("Starting MCP9808 Temperature Sensor")
	// Create a new temperature sensor with hig res
	tempSensor, err := mcp9808.New(*bus, &mcp9808.Opts{Addr: *temperatureAddr, Res: mcp9808.High})
	if err != nil {
		logger.Errorf("Failed to open MCP9808 sensor: %v", err)
		return nil
	}
	a.Temp = tempSensor

	logger.Info("Starting BMP280 reader...")
	bme, err := bmxx80.NewI2C(*bus, 0x76, &bmxx80.DefaultOpts)
	if err != nil {
		logger.Errorf("failed to initialize bme280: %v", err)
		return nil
	}
	a.PH = bme

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
		humidity := RelHumidity(math.Round(float64(em.Humidity) / float64(physic.PercentRH)))
		pressure := PressurehPa(math.Round((float64(em.Pressure)/float64(100*physic.Pascal))*100) / 100)
		if a.test {
			logger.Infof("Pressure [%2f], Humidity [%2f]", pressure.Float64(), humidity.Float64())
		}
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
		if a.test {
			logger.Infof("Temperature [%2f]", temp)
		}
		return temp
	}
	return 0
}
