package bme280

import (
	"github.com/d2r2/go-bsbmp"
	"github.com/d2r2/go-i2c"
	logger "github.com/sirupsen/logrus"
)

//Bme280 the sensor
type Bme280 struct {
	I2cbus *i2c.I2C
}

type sensordata struct {
	Temp float64 
	Humidity float64 
	Pressure float64 
	PressureHg float64
}

func (b *Bme280) Read() *sensordata {

	sensor, err := bsbmp.NewBMP(bsbmp.BME280, b.I2cbus) 
	if err != nil {
		logger.Fatal(err)
	}

	_, err = sensor.ReadSensorID()
	if err != nil {
		logger.Fatal(err)
	}

	err = sensor.IsValidCoefficients()
	if err != nil {
		logger.Fatal(err)
	}

	// Read temperature in celsius degree
	t, err := sensor.ReadTemperatureC(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		logger.Fatal(err)
	}

	// Read atmospheric pressure in pascal
	p, err := sensor.ReadPressurePa(bsbmp.ACCURACY_LOW)
	if err != nil {
		logger.Fatal(err)
	}

	// Read atmospheric pressure in mmHg
	pm, err := sensor.ReadPressureMmHg(bsbmp.ACCURACY_LOW)
	if err != nil {
		logger.Fatal(err)
	}

	// Read atmospheric pressure in mmHg
	supported, h1, err := sensor.ReadHumidityRH(bsbmp.ACCURACY_LOW)
	if supported {
		if err != nil {
			logger.Fatal(err)
		}
	}

	logger.Infof("Temp [%v], Hum [%v], Pressure [%v]Pa ([%v] mmHg)", t, h1, p, pm) // add timestamp?

	return &sensordata{
		Temp: float64(t),
		Humidity: float64(h1),
		Pressure: float64(p),
		PressureHg: float64(pm),
	}
}
