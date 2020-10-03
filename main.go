package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"net/http"
	"time"

	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/devices/bmxx80"
	"periph.io/x/periph/experimental/devices/mcp9808"
	"periph.io/x/periph/host"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
)

const mmPerBucket float64 = 0.3

type sensors struct {
	mcp   *mcp9808.Dev
	bme   *bmxx80.Dev
	btips []int
	count int
	lastTip time.Time
}

type webdata struct {
	Temp       float64 `json:"temp_C"`
	Temp1      float64 `json:"temp1_C"`
	Humidity   float64 `json:"humidity_RH"`
	Pressure   float64 `json:"pressure_hPa"`
	PressureHg float64 `json:"pressure_mmHg"`
	RainHr     float64 `json:"rain_mm_hr"`
	LastTip    string  `json:"last_tip"`
}

func main() {

	logger.Infof("%v: Initialize sensors...", time.Now().Format(time.RFC822))
	m, d, tipbucket, bus := initMCP9808()
	defer (*bus).Close()

	tips := make([]int, 60)
	s := sensors{bme: d, mcp: m, btips: tips, count: 0}

	go s.countBucketTips()
	go s.monitorGPIO(tipbucket)

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

	wd := webdata{
		Temp:       em.Temperature.Celsius(),
		Temp1:      e.Temperature.Celsius(),
		Humidity:   math.Round(float64(em.Humidity) / float64(physic.PercentRH)),
		Pressure:   math.Round(float64(em.Pressure) / float64(100 * physic.Pascal)),
		PressureHg: math.Round(float64(em.Pressure) / float64(physic.Pascal * 133)),
		RainHr:     s.getMMLastHour(),
		LastTip:    s.lastTip.Format(time.RFC822),
	}

	js, err := json.Marshal(wd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

func initMCP9808() (*mcp9808.Dev, *bmxx80.Dev, *gpio.PinIO, *i2c.BusCloser) {
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
	bme, err := bmxx80.NewI2C(bus, 0x76, &bmxx80.DefaultOpts)
	if err != nil {
		logger.Fatalf("failed to initialize bme280: %v", err)
	}

	logger.Info("Starting MCP9808 Temperature Sensor")

	// Create a new temperature sensor a sense with default options.
	sensor, err := mcp9808.New(bus, &mcp9808.Opts{Addr: *address})
	if err != nil {
		logger.Fatalf("failed to open new sensor: %v", err)
	}

	// Lookup a pin by its number:
	pin := gpioreg.ByName("GPIO17")
	if pin == nil {
		log.Fatal("Failed to find GPIO17")
	}

	logger.Infof("%s: %s", pin, pin.Function())

	if err = pin.In(gpio.PullUp, gpio.BothEdges); err != nil {
		log.Fatal(err)
	}

	return sensor, bme, &pin, &bus
}

func (s *sensors) monitorGPIO(p *gpio.PinIO) {
	logger.Info("Starting tip bucket")
	for {
		(*p).WaitForEdge(-1)
		if (*p).Read() == gpio.Low {
			logger.Info("Bucket tip")
			s.count++
			s.lastTip = time.Now()
		}
	}
}

func (s *sensors) countBucketTips() {
	for x := range time.Tick(time.Minute) {
		logger.Debugf("Tick at %v", x)
		min := x.Minute()
		logger.Debugf("Tick at min [%v] with [%v] tips", min, s.count)
		// store the count for the last minute
		s.btips[min] = s.count
		// clear the next one
		if (min == 59){
			min = 0
		}
		s.btips[min + 1] = -1 // use -1 so I know this is the head of the buffer
		// reset the counter
		s.count = 0 
	}
}

func (s *sensors) getMMLastHour() float64{
	total := s.count
	for _, x := range s.btips {
		if x > 0 {
			total += x
		}
	}
	return math.Round(float64(total) * mmPerBucket * 100) / 100
}
