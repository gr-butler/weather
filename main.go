package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/devices/bmxx80"
	"periph.io/x/periph/experimental/devices/mcp9808"
	"periph.io/x/periph/host"

	logger "github.com/sirupsen/logrus"
)

const (
	mmPerBucket float64 = 0.3
	hgToPa float64 = 133.322387415
)

type sensors struct {
	mcp         *mcp9808.Dev
	bme         *bmxx80.Dev
	btips      []int
	count        int				// GPIO bucket tip counter
	lastTip      time.Time			// Last bucket tip
	windticks    int				// GPIO wind speed tick counter
	windTicks250 int				// number of ticks per 250ms
	bus          *i2c.BusCloser
	rainpin      *gpio.PinIO
	windpin      *gpio.PinIO
	rainHr       float64
	pressure     float64
	pressureHg   float64
	humidity     float64
	temp         float64
	rain24     []float64
	pressure24 []float64
	humidity24 []float64
	temp24     []float64
}

type webdata struct {
	TimeNow    string    `json:"time"`
	Temp       float64   `json:"temp_C"`
	Humidity   float64   `json:"humidity_RH"`
	Pressure   float64   `json:"pressure_hPa"`
	PressureHg float64   `json:"pressure_mmHg"`
	RainHr     float64   `json:"rain_mm_hr"`
	LastTip    string    `json:"last_tip"`
	Wind       int       `json:"wind"`
}

type webHistory struct {
	Temp24H     []float64 `json:"temp_24hr"`
	Rain24H     []float64 `json:"rain_mm_24hr"`
	Pressure24H []float64 `json:"pressure_24hr"`
	Humidity24H []float64 `json:"humidity_24hr"`
	TipHistory  []int     `json:"tip_last_hour"`
}

var atmPresure = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "atmospheric_pressure",
        Help: "Atmospheric pressure hPa",
    },
)

var mmRainPerHour = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "mm_rain_last_hour",
        Help: "mm of Rain in the last hour",
    },
)

var mmRainPerMin = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "mm_rain_last_min",
        Help: "mm of Rain in the last min",
    },
)

var rh = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "relative_humidity",
        Help: "Relative Humidity",
    },
)

var temperature = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "temperature",
        Help: "Temperature C",
    },
)

var windspeed = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "windspeed",
        Help: "Wind Speed m/s",
    },
)

// called by prometheus
func init() {
	logger.Infof("%v: Initialize prometheus...", time.Now().Format(time.RFC822))
	prometheus.MustRegister(atmPresure)
	prometheus.MustRegister(mmRainPerHour)
	prometheus.MustRegister(rh)
	prometheus.MustRegister(temperature)
	prometheus.MustRegister(mmRainPerMin)
	prometheus.MustRegister(windspeed)
}

func main() {
	logger.Infof("%v: Initialize sensors...", time.Now().Format(time.RFC822))
	s := sensors{}
	s.initSensors()
	defer (*s.bus).Close()
	
	// get initial values
	s.measureSensors()

	// start go routines
	go s.recordHistory()
	go s.monitorRainGPIO()
	go s.monitorWindGPIO()
	go s.readWindsSpeedHF()

	// start web service
	http.HandleFunc("/", s.handler)
	http.HandleFunc("/hist", s.history)
	http.Handle("/metrics", promhttp.Handler())
	logger.Info("Starting webservice...")
	defer logger.Info("Exiting...")
	logger.Fatal(http.ListenAndServe(":80", nil))
}

func (s *sensors) history (w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	h := webHistory {
		Temp24H: s.temp24,
		Rain24H: s.rain24,
		TipHistory: s.btips,
		Pressure24H: s.pressure24,
		Humidity24H: s.humidity24,
	}

	js, err := json.Marshal(h)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

func (s *sensors) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.measureSensors()
	wd := webdata{
		Temp:       s.temp,
		Humidity:   s.humidity,
		Pressure:   s.pressure,
		PressureHg: s.pressureHg,
		RainHr:     s.getMMLastHour(),
		LastTip:    s.lastTip.Format(time.RFC822),
		TimeNow:    time.Now().Format(time.RFC822),
		Wind:		s.windTicks250,
	}

	js, err := json.Marshal(wd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

func (s *sensors) initSensors() () {
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

	// Lookup a rainpin by its number:
	rainpin := gpioreg.ByName("GPIO17")
	if rainpin == nil {
		log.Fatal("Failed to find GPIO17")
	}

	logger.Infof("%s: %s", rainpin, rainpin.Function())

	if err = rainpin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		log.Fatal(err)
	}
	// Lookup a rainpin by its number:
	windpin := gpioreg.ByName("GPIO27")
	if windpin == nil {
		log.Fatal("Failed to find GPIO27")
	}

	logger.Infof("%s: %s", windpin, windpin.Function())

	if err = windpin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		log.Fatal(err)
	}

	s.bme = bme 
	s.mcp = sensor 
	s.btips = make([]int, 60)
	s.count = 0 
	s.rain24 = make([]float64, 24) 
	s.pressure24 = make([]float64, 24)
	s.humidity24 = make([]float64, 24)
	s.temp24 = make([]float64, 24)
	s.bus = &bus
	s.rainpin = &rainpin
	s.windpin = &windpin
}

func (s *sensors) monitorRainGPIO() {
	logger.Info("Starting tip bucket")
	for {
		(*s.rainpin).WaitForEdge(-1)
		logger.Info("Bucket tip")
		s.count++
		s.lastTip = time.Now()
	}
}

func (s *sensors) monitorWindGPIO() {
	logger.Info("Starting tip bucket")
	for {
		(*s.windpin).WaitForEdge(-1)
		s.windticks++
	}
}

/*
Measuring gusts and wind intensity

Because wind is an element that varies rapidly over very short periods of 
time it is sampled at high frequency (every 0.25 sec) to capture the intensity 
of gusts, or short-lived peaks in speed, which inflict greatest damage in 
storms. The gust speed and direction are defined by the maximum three second 
average wind speed occurring in any period.

A better measure of the overall wind intensity is defined by the average speed 
and direction over the ten minute period leading up to the reporting time. 
Mean wind over other averaging periods may also be calculated. A gale is 
defined as a surface wind of mean speed of 34-40 knots, averaged over a period 
of ten minutes. Terms such as 'severe gale', 'storm', etc are also used to 
describe winds of 41 knots or greater.
*/

func (s *sensors) readWindsSpeedHF(){
	for range time.Tick(250 * time.Millisecond) {
		s.windTicks250 = s.windticks // windticks250 is so we can check the output directly - will remove all these local web values at some point I think
		s.windticks = 0
		windspeed.Set(float64(s.windTicks250))
	}
}

func (s *sensors) measureSensors() {
	e := physic.Env{}
	s.mcp.Sense(&e)
	logger.Debugf("MCP: %8s %10s %9s\n", e.Temperature, e.Pressure, e.Humidity)

	em := physic.Env{}
	s.bme.Sense(&em)
	logger.Debugf("BME: %8s %10s %9s\n", em.Temperature, em.Pressure, em.Humidity)
	s.humidity = math.Round(float64(em.Humidity) / float64(physic.PercentRH))
	s.pressure = math.Round(float64(em.Pressure) / float64(100 * physic.Pascal))
	s.pressureHg = math.Round(float64(em.Pressure) / (float64(physic.Pascal) * hgToPa))
	s.temp = e.Temperature.Celsius()
    // prometheus data
	mmRainPerHour.Set(s.getMMLastHour())
	atmPresure.Set(s.pressure)
	rh.Set(s.humidity)
	temperature.Set(s.temp)
	mmRainPerMin.Set(s.getMMLastMin())
}


func (s *sensors) recordHistory() {
	for x := range time.Tick(time.Minute) {
		s.measureSensors()
		min := x.Minute()
		// store the bucket tip count for the last minute
		s.btips[min] = s.count
		// reset the bucket tip counter
		s.count = 0 
				
		// local history - this will ultimately dissappear when prometheus and grafana are fully working
		if min == 0 {
			h := x.Hour()
			s.rain24[h] = s.getMMLastHour()
			s.humidity24[h] = s.humidity
			s.pressure24[h] = s.pressure
			s.temp24[h] = s.temp
		}
	}
}

func (s *sensors) getMMLastHour() float64 {
	total := s.count
	for _, x := range s.btips {
		total += x
	}
	return math.Round(float64(total) * mmPerBucket * 100) / 100
}

func (s *sensors) getMMLastMin() float64 {
	return (float64(s.count) * mmPerBucket)
}
