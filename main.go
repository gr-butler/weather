package main

import (
	"encoding/json"
	"flag"
	"net/http"

	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/devices/bmxx80"
	"periph.io/x/periph/experimental/devices/mcp9808"
	"periph.io/x/periph/host"

	logger "github.com/sirupsen/logrus"
)


type sensors struct {
	mcp *mcp9808.Dev
	bme *bmxx80.Dev
}

type webdata struct {
	Temp float64 `json:"temp_C"`
	Temp1 float64 `json:"temp1_C"`
	Humidity float64 `json:"humidity_RH"`
	Pressure float64 `json:"pressure_hPa"`
}



func main() {

	logger.Info("Initialize sensors...")
	m, d, bus := initMCP9808()
	defer bus.Close()
	s := sensors{bme: d, mcp: m}
	http.HandleFunc("/", s.handler)
	logger.Info("Starting webservice...")
	logger.Fatal(http.ListenAndServe(":80", nil))
	
	logger.Info("Exiting...")
}

func (s *sensors) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	e := physic.Env{}
	s.mcp.Sense(&e)
	logger.Debugf("MCP: %8s %10s %9s\n", e.Temperature, e.Pressure, e.Humidity)

	em := physic.Env{}
	s.bme.Sense(&em)
	logger.Debugf("BME: %8s %10s %9s\n", em.Temperature, em.Pressure, em.Humidity)
	
	wd := webdata {
		Temp: em.Temperature.Celsius(),
		Temp1: e.Temperature.Celsius(),
		Humidity: float64(em.Humidity) / float64(physic.PercentRH),
		Pressure: float64(em.Pressure) / (10 * float64(physic.Pascal)),
	}

	js, err := json.Marshal(wd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

func initMCP9808() (*mcp9808.Dev, *bmxx80.Dev, i2c.BusCloser) {
	if _, err := host.Init(); err != nil {
		logger.Fatal(err)
	}
	address := flag.Int("address", 0x18, "I²C address")
	i2cbus := flag.String("bus", "", "I²C bus (/dev/i2c-1)")

	flag.Parse()

	// Open default I²C bus.
	bus, err := i2creg.Open(*i2cbus)
	if err != nil {
		logger.Fatalf("failed to open I²C: %v", err)
	}
	
	logger.Info("Starting BMP280 reader...")
	d, err := bmxx80.NewI2C(bus, 0x76, &bmxx80.DefaultOpts)
	if err != nil {
		logger.Fatalf("failed to initialize bme280: %v", err)
	}


	logger.Info("Starting MCP9808 Temperature Sensor")
	
	// Create a new temperature sensor a sense with default options.
	sensor, err := mcp9808.New(bus, &mcp9808.Opts{Addr: *address})
	if err != nil {
		logger.Fatalf("failed to open new sensor: %v", err)
	}
	return sensor, d, bus
}

