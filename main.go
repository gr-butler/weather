package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/d2r2/go-bsbmp"
	"github.com/d2r2/go-i2c"
	logger "github.com/d2r2/go-logger"
)

var lg = logger.NewPackageLogger("main",
	logger.DebugLevel,
	// logger.InfoLevel,
)

type bus struct {
	i2cbus *i2c.I2C
}

func main() {
	defer logger.FinalizeLogger()
	// Create new connection to i2c-bus on 1 line with address 0x76.
	// Use i2cdetect utility to find device address over the i2c-bus
	myi2c, err := i2c.NewI2C(0x76, 1)
	if err != nil {
		lg.Fatal(err)
	}
	defer myi2c.Close()

	b := bus{i2cbus: myi2c}

	// Uncomment/comment next lines to suppress/increase verbosity of output
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("bsbmp", logger.InfoLevel)

	http.HandleFunc("/", b.handler)
	log.Fatal(http.ListenAndServe(":80", nil))
}

func (b *bus) handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, read(b.i2cbus))
}

func read(i2c *i2c.I2C) string {

	// sensor, err := bsbmp.NewBMP(bsbmp.BMP180, i2c) // signature=0x55
	// sensor, err := bsbmp.NewBMP(bsbmp.BMP280, i2c) // signature=0x58
	sensor, err := bsbmp.NewBMP(bsbmp.BME280, i2c) // signature=0x60
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
