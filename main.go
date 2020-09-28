package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/experimental/devices/mcp9808"
	"periph.io/x/periph/host"

	"github.com/d2r2/go-i2c"
	"github.com/pointer2null/weather/bme280"

	d2logger "github.com/d2r2/go-logger"
	logger "github.com/sirupsen/logrus"
)


type sensors struct {
	myBme *bme280.Bme280
	mcp *mcp9808.Dev
}

type webdata struct {
	Temp float64 `json:"temp"`
	Temp1 float64 `json:"temp1"`
	Humidity float64 `json:"humidity"`
	Pressure float64 `json:"pressure"`
}

func main() {
	// temp junk to stop log spam by d2r2
	defer d2logger.FinalizeLogger()
	d2logger.ChangePackageLogLevel("i2c", d2logger.InfoLevel)
	d2logger.ChangePackageLogLevel("bsbmp", d2logger.InfoLevel)


	// Create new connection to i2c-bus on 1 line with address 0x76.
	// Use i2cdetect utility to find device address over the i2c-bus
	myi2c, err := i2c.NewI2C(0x76, 1)
	if err != nil {
		logger.Fatal(err)
	}
	defer myi2c.Close()

	b := bme280.Bme280{I2cbus: myi2c}
	m := initMCP9808()
	s := sensors{myBme: &b, mcp: m}

	http.HandleFunc("/", s.handler)
	log.Fatal(http.ListenAndServe(":80", nil))
	logger.Info("Starting mcp9808 reader...")
	
}

func (s *sensors) handler(w http.ResponseWriter, r *http.Request) {
	sd := s.myBme.Read()

	t1 := getTemp(s.mcp)

	wd := webdata {
		Temp: sd.Temp,
		Temp1: t1,
		Humidity: sd.Humidity,
		Pressure: sd.Pressure,
	}
	json.NewEncoder(w).Encode(wd)
}

func initMCP9808() *mcp9808.Dev {
	if _, err := host.Init(); err != nil {
		logger.Fatal(err)
	}
	address := flag.Int("address", 0x18, "I²C address")
	i2cbus := flag.String("bus", "", "I²C bus (/dev/i2c-1)")

	flag.Parse()

	logger.Info("Starting MCP9808 Temperature Sensor")
	if _, err := host.Init(); err != nil {
		logger.Fatal(err)
	}

	// Open default I²C bus.
	bus, err := i2creg.Open(*i2cbus)
	if err != nil {
		logger.Fatalf("failed to open I²C: %v", err)
	}
	defer bus.Close()

	// Create a new temperature sensor a sense with default options.
	sensor, err := mcp9808.New(bus, &mcp9808.Opts{Addr: *address})
	if err != nil {
		logger.Fatalf("failed to open new sensor: %v", err)
	}
	return sensor
}

func getTemp(sensor *mcp9808.Dev) float64 {
	
	t, err := sensor.SenseTemp()
	if err != nil {
		logger.Errorf("sensor reading error: %v", err)
		return -1000.0
	}

	// Read values from sensor every second.
	// everySecond := time.Tick(time.Second)
	// var halt = make(chan os.Signal, 1)
	// signal.Notify(halt, syscall.SIGTERM)
	// signal.Notify(halt, syscall.SIGINT)

	// logger.Info("ctrl+c to exit")
	// for {
	// 	select {
	// 	case <-everySecond:
	// 		t, err := sensor.SenseTemp()
	// 		if err != nil {
	// 			return logger.Fatalf("sensor reading error: %v", err)
	// 		}
	// 		logger.Info(t)

	// 	case <-halt:
	// 		return nil
	// 	}
	// }
	return t.Celsius()
}
