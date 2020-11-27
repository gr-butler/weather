package main

import (
	"encoding/json"
	"flag"
	"log"
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
	"periph.io/x/periph/experimental/devices/ads1x15"
	"periph.io/x/periph/experimental/devices/mcp9808"
	"periph.io/x/periph/host"

	logger "github.com/sirupsen/logrus"
)

type sensors struct {
	
}
type weatherstation struct {
	bme              *bmxx80.Dev
	hiResT           *mcp9808.Dev
	bus              *i2c.BusCloser
	rainpin          *gpio.PinIO
	windpin          *gpio.PinIO
	windDir          *ads1x15.PinADC

	btips            []int
	count            int       // GPIO bucket tip counter
	lastTip          time.Time // Last bucket tip
	rainHr           float64
	pressure         float64
	pressureHg       float64
	humidity         float64
	temp             float64
	hiResTemp        float64
	instantWindSpeed float64
	windSpeedAvg     float64
	windDirection    float64
	windVolts        float64
	windhist         []time.Time
	pHist            int
}

type webdata struct {
	TimeNow      string  `json:"time"`
	Temp         float64 `json:"temp_C"`
	TempHiRes    float64 `json:"temp2_C"`
	Humidity     float64 `json:"humidity_RH"`
	Pressure     float64 `json:"pressure_hPa"`
	PressureHg   float64 `json:"pressure_mmHg"`
	RainHr       float64 `json:"rain_mm_hr"`
	LastTip      string  `json:"last_tip"`
	WindDir      float64 `json:"wind_dir"`
	WindVolts    float64 `json:"wind_volt"`
	WindSpeed    float64 `json:"wind_speed"`
	WindSpeedAvg float64 `json:"wind_speed_avg"`
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

var highResTemp = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "hiResTemperature",
		Help: "Temperature C",
	},
)

var windspeed = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "windspeed",
		Help: "Average Wind Speed mph",
	},
)

var windgust = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "windgust",
		Help: "Instant wind speed mph",
	},
)

var windDirection = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "winddirection",
		Help: "Wind Direction Deg",
	},
)

// called by prometheus
func init() {
	logger.Infof("%v: Initialize prometheus...", time.Now().Format(time.RFC822))
	prometheus.MustRegister(atmPresure)
	prometheus.MustRegister(mmRainPerHour)
	prometheus.MustRegister(rh)
	prometheus.MustRegister(temperature, highResTemp)
	prometheus.MustRegister(mmRainPerMin)
	prometheus.MustRegister(windspeed)
	prometheus.MustRegister(windgust)
	prometheus.MustRegister(windDirection)
}

func main() {
	logger.Infof("%v: Initialize sensors...", time.Now().Format(time.RFC822))
	w := weatherstation{}
	w.initSensors()
	defer (*w.bus).Close()

	// start go routines
	go w.readAtmosphericSensors()
	go w.readRainData()
	go w.readWindData()

	// start web service
	http.HandleFunc("/", w.handler)
	http.Handle("/metrics", promhttp.Handler())
	logger.Info("Starting webservice...")
	defer logger.Info("Exiting...")
	logger.Fatal(http.ListenAndServe(":80", nil))
}


func (s *weatherstation) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	wd := webdata{
		Temp:         s.temp,
		TempHiRes:    s.hiResTemp,
		Humidity:     s.humidity,
		Pressure:     s.pressure,
		PressureHg:   s.pressureHg,
		RainHr:       s.getMMLastHour(),
		LastTip:      s.lastTip.Format(time.RFC822),
		TimeNow:      time.Now().Format(time.RFC822),
		WindDir:      s.windDirection,
		WindVolts:    s.windVolts,
		WindSpeed:    s.instantWindSpeed,
		WindSpeedAvg: s.windSpeedAvg,
	}

	js, err := json.Marshal(wd)
	if err != nil {
		logger.Errorf("JSON error [%v]" , err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

func (s *weatherstation) initSensors() {
	if _, err := host.Init(); err != nil {
		logger.Error("Failed to init i2c bus")
		logger.Fatal(err)
	}
	i2cbus := flag.String("bus", "", "I²C bus (/dev/i2c-1)")
	temperatureAddr := flag.Int("address", 0x18, "I²C address")

	

	// Open default I²C bus.
	bus, err := i2creg.Open(*i2cbus)
	if err != nil {
		logger.Fatalf("failed to open I²C: %v", err)
	}

	// Create a new temperature sensor a sense with default options.
	tempSensor, err := mcp9808.New(bus, &mcp9808.Opts{Addr: *temperatureAddr})
	if err != nil {
		logger.Errorf("failed to open MCP9808 sensor: %v", err)
	}


	logger.Info("Starting BMP280 reader...")
	bme, err := bmxx80.NewI2C(bus, 0x76, &bmxx80.DefaultOpts)
	if err != nil {
		logger.Errorf("failed to initialize bme280: %v", err)
	}

	logger.Info("Starting MCP9808 Temperature Sensor")

	// Lookup a rainpin by its number:
	rainpin := gpioreg.ByName("GPIO17")
	if rainpin == nil {
		logger.Error("Failed to find GPIO17")
	}

	logger.Infof("%s: %s", rainpin, rainpin.Function())

	if err = rainpin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		log.Fatal(err)
	}
	// Lookup a rainpin by its number:
	windpin := gpioreg.ByName("GPIO27")
	if windpin == nil {
		logger.Error("Failed to find GPIO27")
	}

	logger.Infof("%s: %s", windpin, windpin.Function())

	if err = windpin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		logger.Error(err)
	}

	logger.Info("Starting Wind direction ADC")
	// Create a new ADS1115 ADC.
	adc, err := ads1x15.NewADS1115(bus, &ads1x15.DefaultOpts)
	if err != nil {
		logger.Error(err)
	}

	// Obtain an analog pin from the ADC.
	dirPin, err := adc.PinForChannel(ads1x15.Channel0, 5*physic.Volt, 1*physic.Hertz, ads1x15.SaveEnergy)
	if err != nil {
		logger.Error(err)
	}
	defer dirPin.Halt()

	s.bme = bme
	s.hiResT = tempSensor
	s.btips = make([]int, 60)
	s.count = 0
	s.bus = &bus
	s.rainpin = &rainpin
	s.windpin = &windpin
	s.windDir = &dirPin
	s.windhist = make([]time.Time, 300)
	s.pHist = 0
}


