package bme280

import (
	"fmt"

	"github.com/d2r2/go-bsbmp"
	"github.com/d2r2/go-i2c"
	logger "github.com/d2r2/go-logger"
)

var lg = logger.NewPackageLogger("main",
	logger.DebugLevel,
	// logger.InfoLevel,
)

type Bme280 struct {
	I2cbus *i2c.I2C
}

func (b *Bme280) Read() string {

	// sensor, err := bsbmp.NewBMP(bsbmp.BMP180, i2c) // signature=0x55
	// sensor, err := bsbmp.NewBMP(bsbmp.BMP280, i2c) // signature=0x58
	sensor, err := bsbmp.NewBMP(bsbmp.BME280, b.I2cbus) // signature=0x60
	// sensor, err := bsbmp.NewBMP(bsbmp.BMP388, i2c) // signature=0x50
	if err != nil {
		lg.Fatal(err)
	}

	id, err := sensor.ReadSensorID()
	if err != nil {
		lg.Fatal(err)
	}
	lg.Infof("This Bosch Sensortec sensor has signature: 0x%x", id)

	err = sensor.IsValidCoefficients()
	if err != nil {
		lg.Fatal(err)
	}

	// Read temperature in celsius degree
	t, err := sensor.ReadTemperatureC(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		lg.Fatal(err)
	}
	lg.Infof("Temprature = %v*C", t)

	// Read atmospheric pressure in pascal
	p, err := sensor.ReadPressurePa(bsbmp.ACCURACY_LOW)
	if err != nil {
		lg.Fatal(err)
	}
	lg.Infof("Pressure = %v Pa", p)

	// Read atmospheric pressure in mmHg
	pm, err := sensor.ReadPressureMmHg(bsbmp.ACCURACY_LOW)
	if err != nil {
		lg.Fatal(err)
	}
	lg.Infof("Pressure = %v mmHg", pm)

	// Read atmospheric pressure in mmHg
	supported, h1, err := sensor.ReadHumidityRH(bsbmp.ACCURACY_LOW)
	if supported {
		if err != nil {
			lg.Fatal(err)
		}
		lg.Infof("Humidity = %v %%", h1)
	}

	// Read atmospheric altitude in meters above sea level, if we assume
	// that pressure at see level is equal to 101325 Pa.
	a, err := sensor.ReadAltitude(bsbmp.ACCURACY_LOW)
	if err != nil {
		lg.Fatal(err)
	}
	lg.Infof("Altitude = %v m", a)

	return fmt.Sprintf("Temp [%v], Hum [%v], Pressure [%v]Pa ([%v] mmHg)", t, h1, p, pm)
}
